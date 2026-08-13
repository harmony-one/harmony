// Package release implements package-db: the minimal single-invocation
// sealer that stages the verified compact artifact, fully re-verifies it,
// atomically promotes it into the release layout, and seals it with a READY
// completeness sentinel — journaled so every crash window reconciles
// deterministically (plan WS7).
package release

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"
	"time"

	"github.com/harmony-one/harmony/internal/recoverydb/anchor"
	"github.com/harmony-one/harmony/internal/recoverydb/dbopen"
	"github.com/harmony-one/harmony/internal/recoverydb/integrity"
	"github.com/harmony-one/harmony/internal/recoverydb/report"
	"github.com/harmony-one/harmony/internal/recoverydb/verify"
)

//go:embed install_template.md
var installTemplate string

// FieldAbsent is the recorded value for optional in-place integration fields
// whose flags were omitted (plan WS7, revision 11: never blocking).
const FieldAbsent = "absent"

// Config parameterizes package-db.
type Config struct {
	Network string
	ShardID uint32

	DBPath                 string // the COMPLETE_VERIFIED compact destination
	AnchorPath             string
	TargetHeight           uint64
	VerificationReportPath string

	// Optional in-place integration inputs.
	RecoveryHarmonyBinarySHA256   string
	ProvisionalMinimumStartViewID string

	ReleaseRoot string
	ToolVersion string

	// OutputPath is package.json, written durably BEFORE the journal's
	// terminal record so a COMPLETE_VERIFIED journal always has its report
	// (round 13 finding 8).
	OutputPath string
}

// Run stages, verifies, promotes and seals in one invocation, returning the
// package report. Reruns reconcile per the journal substate rules.
func Run(cfg Config) (*report.PackageReport, string, error) {
	// ---- Gates. ----
	if err := dbopen.RequireAbsolute(cfg.DBPath); err != nil {
		return nil, "", err
	}
	if err := dbopen.RequireAbsolute(cfg.ReleaseRoot); err != nil {
		return nil, "", err
	}
	if _, err := integrity.VerifyChecksumFile(cfg.AnchorPath); err != nil {
		return nil, "", fmt.Errorf("release: checksum gate: %w", err)
	}
	anchorRef, err := integrity.NewInputRef("anchor-manifest", cfg.AnchorPath)
	if err != nil {
		return nil, "", err
	}
	anc, err := anchor.Load(cfg.AnchorPath)
	if err != nil {
		return nil, "", err
	}
	if err := anc.RequireTargetHeight(cfg.TargetHeight); err != nil {
		return nil, "", err
	}
	if anc.Network != cfg.Network || anc.ShardID != cfg.ShardID {
		return nil, "", fmt.Errorf("release: anchor is for %s shard %d, run is %s shard %d (round 13 finding 3)",
			anc.Network, anc.ShardID, cfg.Network, cfg.ShardID)
	}
	if _, err := integrity.VerifyChecksumFile(cfg.VerificationReportPath); err != nil {
		return nil, "", fmt.Errorf("release: checksum gate: %w", err)
	}
	verifRef, err := integrity.NewInputRef("verification-report", cfg.VerificationReportPath)
	if err != nil {
		return nil, "", err
	}
	var verif report.VerificationReport
	if err := report.ReadJSONStrict(cfg.VerificationReportPath, &verif); err != nil {
		return nil, "", err
	}
	if !verif.Passed {
		return nil, "", fmt.Errorf("release: --verification-report did not pass; refusing to package")
	}
	chained := false
	for _, in := range verif.Inputs {
		if in.Name == "anchor-manifest" && in.SHA256 == anchorRef.SHA256 {
			chained = true
		}
	}
	if !chained {
		return nil, "", fmt.Errorf("release: --verification-report does not chain to the supplied anchor (broken hash chain)")
	}
	// Destination journal must be COMPLETE_VERIFIED; an oversized
	// COMPLETE_UNRELEASABLE build has its own error.
	jstate, _, err := report.JournalState(report.JournalPath(cfg.DBPath))
	if err != nil {
		return nil, "", fmt.Errorf("release: destination journal: %w", err)
	}
	switch jstate {
	case report.StateCompleteVerified:
	case report.StateCompleteUnreleasable:
		return nil, "", fmt.Errorf("release: destination is COMPLETE_UNRELEASABLE (e.g. failed the size gate); refusing to package")
	default:
		return nil, "", fmt.Errorf("release: destination journal state %s; only COMPLETE_VERIFIED is packageable", jstate)
	}
	if verif.JournalState != report.StateCompleteVerified {
		return nil, "", fmt.Errorf("release: --verification-report records journal state %q", verif.JournalState)
	}

	// ---- Hold the source storage lock for the whole stage-and-hash pass. ----
	srcHold, err := dbopen.OpenReadOnly(cfg.DBPath)
	if err != nil {
		return nil, "", fmt.Errorf("release: hold source: %w", err)
	}
	defer srcHold.Close()

	// ---- Bind the packaging run to the database the report actually
	// verified (round 13 finding 2): identity fields, the artifact's own
	// recovery marker, and a full recomputation of the marker-excluded
	// logical KV digest. A different or modified database cannot be sealed
	// under a stale passing report.
	if verif.DBPath != cfg.DBPath {
		return nil, "", fmt.Errorf("release: --verification-report verified %q, packaging %q; refusing", verif.DBPath, cfg.DBPath)
	}
	if verif.Network != cfg.Network || verif.ShardID != cfg.ShardID {
		return nil, "", fmt.Errorf("release: --verification-report is for %s shard %d, run is %s shard %d",
			verif.Network, verif.ShardID, cfg.Network, cfg.ShardID)
	}
	if verif.LogicalKVDigest == "" {
		return nil, "", fmt.Errorf("release: --verification-report carries no logical KV digest")
	}
	marker, err := verify.ReadMarker(srcHold)
	if err != nil {
		return nil, "", fmt.Errorf("release: held source: %w", err)
	}
	switch {
	case marker.AnchorManifestSHA256 != anchorRef.SHA256:
		return nil, "", fmt.Errorf("release: source marker chains to anchor %s, supplied anchor is %s", marker.AnchorManifestSHA256, anchorRef.SHA256)
	case marker.TargetHeight != anc.TargetHeight || marker.TargetHash != anc.TargetHash.Hex():
		return nil, "", fmt.Errorf("release: source marker target (%d,%s) != anchor (%d,%s)",
			marker.TargetHeight, marker.TargetHash, anc.TargetHeight, anc.TargetHash.Hex())
	case marker.Network != cfg.Network || marker.ShardID != cfg.ShardID:
		return nil, "", fmt.Errorf("release: source marker identity (%s, shard %d) != run (%s, shard %d)",
			marker.Network, marker.ShardID, cfg.Network, cfg.ShardID)
	case marker.LogicalKVDigest != verif.LogicalKVDigest:
		return nil, "", fmt.Errorf("release: source marker logical digest %s != verification report %s", marker.LogicalKVDigest, verif.LogicalKVDigest)
	}
	logical, err := verify.ComputeLogicalDigest(srcHold)
	if err != nil {
		return nil, "", fmt.Errorf("release: recompute source logical digest: %w", err)
	}
	if logical.Total.SHA256 != verif.LogicalKVDigest {
		return nil, "", fmt.Errorf("release: held source logical digest %s != verification report %s (database modified since verification)",
			logical.Total.SHA256, verif.LogicalKVDigest)
	}

	payloadBytes, payloadFiles, err := payloadSize(cfg.DBPath)
	if err != nil {
		return nil, "", err
	}

	// ---- release.json content (frozen first; ID derived from it). ----
	rj := &report.ReleaseJSON{
		SchemaVersion: report.ReleaseSchemaV1,
		ReleaseID:     "-",
		Network:       cfg.Network,
		ShardID:       cfg.ShardID,
		Profile:       "validator",

		TargetHeight:     anc.TargetHeight,
		TargetHash:       anc.TargetHash.Hex(),
		TargetParentHash: anc.TargetParentHash.Hex(),
		TargetEpoch:      anc.TargetEpoch,
		StateRoot:        verif.DigestSet.StateRoot,

		AbandonedChildHeight: anc.AbandonedChildHeight,
		AbandonedChildHash:   anc.AbandonedChildHash.Hex(),
		RejectedShard1Height: anc.RejectedShard1Height,
		RejectedShard1Hash:   anc.RejectedShard1Hash.Hex(),

		DatabaseFormat:  "leveldb/harmony_db_0",
		StateTrieScheme: "hashScheme",

		PayloadBytes: payloadBytes,
		PayloadFiles: payloadFiles,

		ProducerBinarySHA256:     verif.ToolBinary,
		ProducerToolVersion:      verif.ToolVersion,
		VerificationReportSHA256: verifRef.SHA256,

		Mode:                    verif.Mode,
		MetadataReferenceDigest: metadataDigestOf(&verif),
		NormalizedOutputDigest:  verif.NormalizedOutputDigest,
		LogicalKVDigest:         verif.LogicalKVDigest,

		RecoveryHarmonyBinarySHA256:   orAbsent(cfg.RecoveryHarmonyBinarySHA256),
		ProvisionalMinimumStartViewID: orAbsent(cfg.ProvisionalMinimumStartViewID),

		Inputs: []report.ChainLink{
			{Name: "anchor-manifest", SHA256: anchorRef.SHA256},
			{Name: "verification-report", SHA256: verifRef.SHA256},
		},
	}
	releaseID, err := DeriveReleaseID(rj)
	if err != nil {
		return nil, "", err
	}
	rj.ReleaseID = releaseID
	rj.CreatedAt = time.Now().UTC().Format(time.RFC3339)

	parent := filepath.Join(cfg.ReleaseRoot, "recovery", cfg.Network,
		fmt.Sprintf("shard-%d", cfg.ShardID), fmt.Sprintf("%d", anc.TargetHeight),
		strings.ToLower(anc.TargetHash.Hex()), "validator")
	finalDir := filepath.Join(parent, releaseID)
	journalPath := filepath.Join(parent, releaseID+".journal")

	if err := os.MkdirAll(parent, 0o755); err != nil {
		return nil, "", fmt.Errorf("release: create release parent: %w", err)
	}

	// ---- Rerun reconciliation (the single defined no-resume exception). ----
	if _, err := os.Stat(journalPath); err == nil {
		return reconcile(cfg, rj, parent, finalDir, journalPath, releaseID)
	}
	if _, err := os.Stat(finalDir); err == nil {
		// A final directory without any journal record is quarantined and
		// reported, never silently reused (plan WS7).
		qdir := finalDir + ".quarantined-" + fmt.Sprintf("%d", time.Now().Unix())
		if err := os.Rename(finalDir, qdir); err != nil {
			return nil, "", fmt.Errorf("release: quarantine unjournaled release dir: %w", err)
		}
		return nil, "", fmt.Errorf("release: found release directory with no journal; quarantined to %s — investigate before rerunning", qdir)
	}

	journal, err := report.CreateJournal(journalPath)
	if err != nil {
		return nil, "", err
	}
	defer journal.Close()

	// ---- Stage. ----
	staging, err := os.MkdirTemp(parent, ".staging-")
	if err != nil {
		return nil, "", fmt.Errorf("release: create staging: %w", err)
	}
	defer os.RemoveAll(staging) // no-op after successful promotion

	payloadDst := filepath.Join(staging, "payload", "harmony_db_0")
	if err := copyTree(cfg.DBPath, payloadDst); err != nil {
		return nil, "", err
	}
	// Build reports: the verification report and the anchor, with sidecars.
	reportsDir := filepath.Join(staging, "build-reports")
	if err := os.MkdirAll(reportsDir, 0o755); err != nil {
		return nil, "", err
	}
	for _, src := range []string{cfg.VerificationReportPath, integrity.ChecksumPath(cfg.VerificationReportPath), cfg.AnchorPath, integrity.ChecksumPath(cfg.AnchorPath)} {
		if err := copyFile(src, filepath.Join(reportsDir, filepath.Base(src))); err != nil {
			return nil, "", err
		}
	}

	// release.json frozen first (no digest of SHA256SUMS inside — round 12
	// finding 1), then INSTALL.md, then SHA256SUMS generated LAST.
	rjRaw, err := json.MarshalIndent(rj, "", "  ")
	if err != nil {
		return nil, "", fmt.Errorf("release: marshal release.json: %w", err)
	}
	rjRaw = append(rjRaw, '\n')
	if err := writeFileSync(filepath.Join(staging, "release.json"), rjRaw); err != nil {
		return nil, "", err
	}
	installRaw, err := renderInstall(rj)
	if err != nil {
		return nil, "", err
	}
	if err := writeFileSync(filepath.Join(staging, "INSTALL.md"), installRaw); err != nil {
		return nil, "", err
	}
	if err := writeSums(staging); err != nil {
		return nil, "", err
	}
	if err := report.FsyncWalk(report.OSFS, staging); err != nil {
		return nil, "", err
	}

	// ---- Full pre-promotion re-verification (every entry re-hashed; no
	// missing/extra files against the defined tree). ----
	sumsEntries, err := verifyTree(staging)
	if err != nil {
		return nil, "", fmt.Errorf("release: pre-promotion re-verification failed: %w", err)
	}

	// ---- Journaled promote and seal. ----
	if err := journal.Substate(report.SubstatePromoting, releaseID); err != nil {
		return nil, "", err
	}
	if err := os.Rename(staging, finalDir); err != nil {
		return nil, "", fmt.Errorf("release: promote (rename): %w", err)
	}
	if err := fsyncDir(parent); err != nil {
		return nil, "", err
	}
	if err := journal.Substate(report.SubstatePromoted, releaseID); err != nil {
		return nil, "", err
	}
	if err := writeFileSync(filepath.Join(finalDir, "READY"), []byte(releaseID+"\n")); err != nil {
		return nil, "", err
	}
	if err := fsyncDir(finalDir); err != nil {
		return nil, "", err
	}
	if err := journal.Substate(report.SubstateSealed, releaseID); err != nil {
		return nil, "", err
	}
	rep, err := emitPackageReport(cfg, rj, finalDir, sumsEntries)
	if err != nil {
		return nil, "", err
	}
	if err := journal.Complete(report.StateCompleteVerified, "sealed "+releaseID); err != nil {
		return nil, "", err
	}
	return rep, finalDir, nil
}

// reconcile handles reruns per the plan's deterministic windows.
func reconcile(cfg Config, rj *report.ReleaseJSON, parent, finalDir, journalPath, releaseID string) (*report.PackageReport, string, error) {
	journal, err := report.LoadJournal(journalPath)
	if err != nil {
		return nil, "", err
	}
	defer journal.Close()
	last := journal.Last()

	switch {
	case last.State == report.StateCompleteVerified:
		// Sealed and journal-completed: the already-exists refusal applies
		// (byte-identical rebuild maps to the same ID).
		return nil, "", fmt.Errorf("release: release %s already exists (sealed and journal-completed); corrections are new builds with new IDs", releaseID)

	case last.Substate == report.SubstatePromoted || last.Substate == report.SubstateSealed:
		if _, err := os.Stat(finalDir); err != nil {
			return nil, "", fmt.Errorf("release: journal records %s but the release directory is missing: %w", last.Substate, err)
		}
		readyPath := filepath.Join(finalDir, "READY")
		readyRaw, readyErr := os.ReadFile(readyPath)
		if readyErr == nil {
			// Crash after the seal but before the terminal record: fully
			// re-verify, validate READY content, regenerate package.json if
			// missing, complete the journal.
			if strings.TrimSpace(string(readyRaw)) != releaseID {
				return nil, "", fmt.Errorf("release: READY content %q != release ID %s", strings.TrimSpace(string(readyRaw)), releaseID)
			}
			sums, err := verifyTree(finalDir)
			if err != nil {
				return nil, "", fmt.Errorf("release: sealed-directory re-verification failed: %w", err)
			}
			rep, err := emitPackageReport(cfg, rj, finalDir, sums)
			if err != nil {
				return nil, "", err
			}
			if err := journal.Complete(report.StateCompleteVerified, "reconciled sealed "+releaseID); err != nil {
				return nil, "", err
			}
			return rep, finalDir, nil
		}
		// Crash between the atomic rename and the READY write: re-verify
		// the unsealed directory in full; if clean, seal and complete; a
		// failed re-verify quarantines.
		sums, err := verifyTree(finalDir)
		if err != nil {
			qdir := finalDir + ".quarantined-" + fmt.Sprintf("%d", time.Now().Unix())
			if qerr := os.Rename(finalDir, qdir); qerr != nil {
				return nil, "", fmt.Errorf("release: re-verify failed (%v) and quarantine failed: %w", err, qerr)
			}
			return nil, "", fmt.Errorf("release: promoted directory failed re-verification and was quarantined to %s: %w", qdir, err)
		}
		if err := writeFileSync(filepath.Join(finalDir, "READY"), []byte(releaseID+"\n")); err != nil {
			return nil, "", err
		}
		if err := fsyncDir(finalDir); err != nil {
			return nil, "", err
		}
		if err := journal.Substate(report.SubstateSealed, releaseID); err != nil {
			return nil, "", err
		}
		rep, err := emitPackageReport(cfg, rj, finalDir, sums)
		if err != nil {
			return nil, "", err
		}
		if err := journal.Complete(report.StateCompleteVerified, "reconciled and sealed "+releaseID); err != nil {
			return nil, "", err
		}
		return rep, finalDir, nil

	case last.Substate == report.SubstatePromoting:
		// Only the disposable staging remains: discard and rebuild.
		if _, err := os.Stat(finalDir); err == nil {
			// Rename happened but PROMOTED was not recorded; treat as the
			// promoted-unsealed window (the rename is the atomic point).
			sums, err := verifyTree(finalDir)
			if err != nil {
				qdir := finalDir + ".quarantined-" + fmt.Sprintf("%d", time.Now().Unix())
				if qerr := os.Rename(finalDir, qdir); qerr != nil {
					return nil, "", fmt.Errorf("release: re-verify failed (%v) and quarantine failed: %w", err, qerr)
				}
				return nil, "", fmt.Errorf("release: promoted directory failed re-verification and was quarantined to %s: %w", qdir, err)
			}
			if err := writeFileSync(filepath.Join(finalDir, "READY"), []byte(releaseID+"\n")); err != nil {
				return nil, "", err
			}
			if err := fsyncDir(finalDir); err != nil {
				return nil, "", err
			}
			if err := journal.Substate(report.SubstateSealed, releaseID); err != nil {
				return nil, "", err
			}
			rep, err := emitPackageReport(cfg, rj, finalDir, sums)
			if err != nil {
				return nil, "", err
			}
			if err := journal.Complete(report.StateCompleteVerified, "reconciled and sealed "+releaseID); err != nil {
				return nil, "", err
			}
			return rep, finalDir, nil
		}
		// Only the disposable staging remains: discard and rebuild (plan
		// WS7 — the payload was fully verified before PROMOTING was ever
		// recorded, so a fresh build is always safe).
		journal.Close()
		removeStaging(parent)
		if err := os.Remove(journalPath); err != nil {
			return nil, "", fmt.Errorf("release: discard stale journal: %w", err)
		}
		return Run(cfg)

	default:
		// IN_PROGRESS before promotion: staging is disposable; discard and
		// rebuild.
		journal.Close()
		removeStaging(parent)
		if err := os.Remove(journalPath); err != nil {
			return nil, "", fmt.Errorf("release: discard stale journal: %w", err)
		}
		return Run(cfg)
	}
}

func removeStaging(parent string) {
	entries, err := os.ReadDir(parent)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() && strings.HasPrefix(e.Name(), ".staging-") {
			os.RemoveAll(filepath.Join(parent, e.Name()))
		}
	}
}

func metadataDigestOf(v *report.VerificationReport) string {
	return v.MetadataReferenceDigest // "internal:none" sentinel in internal mode
}

func orAbsent(s string) string {
	if s == "" {
		return FieldAbsent
	}
	return s
}

// DeriveReleaseID computes the first 16 hex chars of SHA-256 over the
// canonical release.json with release_id fixed to "-" and created_at
// excluded (plan WS7).
func DeriveReleaseID(rj *report.ReleaseJSON) (string, error) {
	clone := *rj
	clone.ReleaseID = "-"
	clone.CreatedAt = ""
	raw, err := json.Marshal(&clone)
	if err != nil {
		return "", fmt.Errorf("release: derive ID: %w", err)
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])[:16], nil
}

func renderInstall(rj *report.ReleaseJSON) ([]byte, error) {
	tpl, err := template.New("install").Parse(installTemplate)
	if err != nil {
		return nil, fmt.Errorf("release: parse INSTALL template: %w", err)
	}
	var sb strings.Builder
	if err := tpl.Execute(&sb, rj); err != nil {
		return nil, fmt.Errorf("release: render INSTALL.md: %w", err)
	}
	return []byte(sb.String()), nil
}

// writeSums generates SHA256SUMS last, covering every file in the tree
// except exactly {SHA256SUMS, READY} (round 12 finding 1: no back-reference,
// release.json and INSTALL.md included).
func writeSums(root string) error {
	entries, err := treeEntries(root)
	if err != nil {
		return err
	}
	var sums []integrity.SumsEntry
	for _, rel := range entries {
		sum, err := integrity.FileSHA256(filepath.Join(root, rel))
		if err != nil {
			return err
		}
		sums = append(sums, integrity.SumsEntry{SHA256: sum, Name: rel})
	}
	return integrity.WriteSums(filepath.Join(root, "SHA256SUMS"), sums)
}

// treeEntries lists relative regular-file paths minus the two exclusions.
func treeEntries(root string) ([]string, error) {
	var out []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel == "SHA256SUMS" || rel == "READY" {
			return nil
		}
		out = append(out, rel)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("release: walk %s: %w", root, err)
	}
	sort.Strings(out)
	return out, nil
}

// verifyTree re-hashes every SHA256SUMS entry and asserts the entry set is
// exactly the tree minus {SHA256SUMS, READY}.
func verifyTree(root string) (uint64, error) {
	sums, err := integrity.ReadSums(filepath.Join(root, "SHA256SUMS"))
	if err != nil {
		return 0, err
	}
	listed := map[string]string{}
	for _, e := range sums {
		if _, dup := listed[e.Name]; dup {
			return 0, fmt.Errorf("duplicate SHA256SUMS entry %q", e.Name)
		}
		listed[e.Name] = e.SHA256
	}
	actual, err := treeEntries(root)
	if err != nil {
		return 0, err
	}
	if len(actual) != len(listed) {
		return 0, fmt.Errorf("SHA256SUMS lists %d files, tree has %d (missing or extra files)", len(listed), len(actual))
	}
	for _, rel := range actual {
		want, ok := listed[rel]
		if !ok {
			return 0, fmt.Errorf("file %q present in tree but not in SHA256SUMS", rel)
		}
		if err := integrity.VerifyRecorded(filepath.Join(root, rel), want); err != nil {
			return 0, err
		}
	}
	return uint64(len(actual)), nil
}

// emitPackageReport builds package.json and writes it durably (when an
// output path is configured) BEFORE the caller records the journal's
// terminal state, so COMPLETE_VERIFIED always implies the report exists
// (round 13 finding 8).
func emitPackageReport(cfg Config, rj *report.ReleaseJSON, finalDir string, sumsEntries uint64) (*report.PackageReport, error) {
	rep, err := buildPackageReport(cfg, rj, finalDir, sumsEntries)
	if err != nil {
		return nil, err
	}
	if cfg.OutputPath != "" {
		if _, err := report.WriteJSON(cfg.OutputPath, rep); err != nil {
			return nil, err
		}
	}
	return rep, nil
}

func buildPackageReport(cfg Config, rj *report.ReleaseJSON, finalDir string, sumsEntries uint64) (*report.PackageReport, error) {
	meta, err := report.NewMeta(report.PackageSchemaV1, "package-db", cfg.Network, cfg.ShardID, cfg.ToolVersion, nil)
	if err != nil {
		return nil, err
	}
	rjSum, err := integrity.FileSHA256(filepath.Join(finalDir, "release.json"))
	if err != nil {
		return nil, err
	}
	return &report.PackageReport{
		Meta:              meta,
		ReleaseID:         rj.ReleaseID,
		ReleaseDir:        finalDir,
		TargetHeight:      rj.TargetHeight,
		TargetHash:        rj.TargetHash,
		PayloadBytes:      rj.PayloadBytes,
		PayloadFiles:      rj.PayloadFiles,
		SumsEntries:       sumsEntries,
		ReleaseJSONSHA256: rjSum,
		JournalState:      report.StateCompleteVerified,
	}, nil
}

// PublishNote is printed for the devops handoff (plan WS7).
func PublishNote(finalDir string) string {
	return fmt.Sprintf(`Devops publish handoff
======================
Sealed release directory: %s

How to publish (upload execution is devops-owned):
  - Transfer the WHOLE release directory with checksums intact:
      rclone copy --checksum %s <remote>/<same-path>
  - Preferably upload READY last so a partial upload is never mistaken for a
    complete one (e.g. exclude READY on the first pass, copy it after
    verification):
      rclone copy --checksum --exclude READY %s <remote>/<same-path>
      rclone check --checksum --exclude READY %s <remote>/<same-path>
      rclone copyto %s/READY <remote>/<same-path>/READY
  - Consumers detect partial/tampered trees via SHA256SUMS either way.
Sealed paths never mutate; corrections are new builds with new IDs.
`, finalDir, finalDir, finalDir, finalDir, finalDir)
}

// ---- small fs helpers ----

func copyTree(srcRoot, dstRoot string) error {
	return filepath.Walk(srcRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(srcRoot, path)
		if err != nil {
			return err
		}
		// The LOCK file is a runtime lock artifact, not database content;
		// stock leveldb recreates it at open. Excluding it keeps the
		// payload tree deterministic.
		if rel == "LOCK" {
			return nil
		}
		dst := filepath.Join(dstRoot, rel)
		if info.IsDir() {
			return os.MkdirAll(dst, 0o755)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("release: refusing non-regular file %s in payload", path)
		}
		return copyFile(path, dst)
	})
}

// copyFile streams with a fixed buffer and fsyncs the destination (plan WS7:
// streamed copy, each staged file fsynced; reflink is a linux-only
// optimization intentionally not used here — correctness first).
func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("release: open %s: %w", src, err)
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("release: create %s: %w", dst, err)
	}
	buf := make([]byte, 1<<20)
	if _, err := io.CopyBuffer(out, in, buf); err != nil {
		out.Close()
		return fmt.Errorf("release: copy %s: %w", src, err)
	}
	if err := out.Sync(); err != nil {
		out.Close()
		return fmt.Errorf("release: fsync %s: %w", dst, err)
	}
	return out.Close()
}

func writeFileSync(path string, data []byte) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("release: create %s: %w", path, err)
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		return fmt.Errorf("release: write %s: %w", path, err)
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return fmt.Errorf("release: fsync %s: %w", path, err)
	}
	return f.Close()
}

func fsyncDir(dir string) error {
	f, err := os.Open(dir)
	if err != nil {
		return fmt.Errorf("release: open dir %s: %w", dir, err)
	}
	defer f.Close()
	if err := f.Sync(); err != nil {
		return fmt.Errorf("release: fsync dir %s: %w", dir, err)
	}
	return nil
}

func payloadSize(root string) (uint64, uint64, error) {
	var b, n uint64
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		if rel == "LOCK" {
			return nil
		}
		if info.Mode().IsRegular() {
			b += uint64(info.Size())
			n++
		}
		return nil
	})
	if err != nil {
		return 0, 0, fmt.Errorf("release: payload size: %w", err)
	}
	return b, n, nil
}
