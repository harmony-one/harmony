package main

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/internal/recoverydb/anchor"
	"github.com/harmony-one/harmony/internal/recoverydb/dbopen"
	"github.com/harmony-one/harmony/internal/recoverydb/harness"
	"github.com/harmony-one/harmony/internal/recoverydb/integrity"
	"github.com/harmony-one/harmony/internal/recoverydb/report"
	"github.com/harmony-one/harmony/internal/recoverydb/strictdb"
	"github.com/harmony-one/harmony/internal/recoverydb/verify"
)

func verifyCmd() *cobra.Command {
	var (
		dbPath          string
		readOnly        bool
		anchorPath      string
		metaRefManifest string
		fullState       bool
		fullOffchain    bool
		sourceReference string
		tempDir         string
		output          string
	)
	cmd := &cobra.Command{
		Use:   "verify-db",
		Short: "Read-only deep verifier of the compact validator artifact (plan WS6)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireGlobals(cmd); err != nil {
				return err
			}
			if !readOnly {
				return usageErr("--read-only is mandatory for verify-db")
			}
			if !fullState || !fullOffchain {
				return usageErr("--full-state-check and --full-offchain-check are mandatory for the compact artifact")
			}
			if dbPath == "" || anchorPath == "" || sourceReference == "" || output == "" {
				return usageErr("--db, --anchor-manifest, --source-reference and --output are mandatory")
			}
			if err := requireAbsPaths("db", dbPath, "anchor-manifest", anchorPath,
				"metadata-reference-manifest", metaRefManifest,
				"source-reference", sourceReference, "temp-dir", tempDir,
				"output", output); err != nil {
				return err
			}
			return runVerify(dbPath, anchorPath, metaRefManifest, sourceReference, tempDir, output)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "absolute path to the compact artifact (opened strictly read-only)")
	cmd.Flags().BoolVar(&readOnly, "read-only", false, "acknowledge read-only open (mandatory)")
	cmd.Flags().StringVar(&anchorPath, "anchor-manifest", "", "anchor manifest")
	cmd.Flags().StringVar(&metaRefManifest, "metadata-reference-manifest", "", "optional in-place reference manifest; must match compact.json's recorded mode")
	cmd.Flags().BoolVar(&fullState, "full-state-check", false, "full state traversal (mandatory)")
	cmd.Flags().BoolVar(&fullOffchain, "full-offchain-check", false, "full off-chain set checks (mandatory)")
	cmd.Flags().StringVar(&sourceReference, "source-reference", "", "compact.json of the build under verification")
	cmd.Flags().StringVar(&tempDir, "temp-dir", "", "scratch space for the reachable-set database (default OS temp)")
	cmd.Flags().StringVar(&output, "output", "", "verification.json output path")
	return cmd
}

func runVerify(dbPath, anchorPath, metaRefManifest, sourceReference, tempDir, output string) error {
	start := time.Now()

	if _, err := integrity.VerifyChecksumFile(anchorPath); err != nil {
		return preconditionErr(err)
	}
	anchorRef, err := integrity.NewInputRef("anchor-manifest", anchorPath)
	if err != nil {
		return ioErr(err)
	}
	anc, err := anchor.Load(anchorPath)
	if err != nil {
		return preconditionErr(err)
	}
	if _, err := integrity.VerifyChecksumFile(sourceReference); err != nil {
		return preconditionErr(err)
	}
	compactRef, err := integrity.NewInputRef("source-reference", sourceReference)
	if err != nil {
		return ioErr(err)
	}
	var compactRep report.CompactReport
	if err := report.ReadJSONStrict(sourceReference, &compactRep); err != nil {
		return preconditionErr(err)
	}
	if compactRep.DigestSet == nil {
		return preconditionErr(fmt.Errorf("verify-db: compact.json carries no DigestSet"))
	}
	// Mode must match: supplied on one side and not the other is an error
	// (plan §4 CLI contract).
	if compactRep.Mode == report.ModeReference && metaRefManifest == "" {
		return preconditionErr(fmt.Errorf("verify-db: compact.json records reference mode; --metadata-reference-manifest is required"))
	}
	if compactRep.Mode == report.ModeInternal && metaRefManifest != "" {
		return preconditionErr(fmt.Errorf("verify-db: compact.json records internal mode; --metadata-reference-manifest must not be supplied"))
	}

	inputs := []integrity.InputRef{anchorRef, compactRef}
	if metaRefManifest != "" {
		ref, err := integrity.NewInputRef("metadata-reference-manifest", metaRefManifest)
		if err != nil {
			return ioErr(err)
		}
		inputs = append(inputs, ref)
	}

	sched, err := harness.Schedule(flagNetwork)
	if err != nil {
		return usageErr("%v", err)
	}
	if _, err := harness.InitSchedule(flagNetwork); err != nil {
		return usageErr("%v", err)
	}
	// The verification window is the one compact.json RECORDS (round 13
	// finding 5: --retain-from-height builds carry an extended window).
	// ComputeWindow validates that the recorded window is the schedule
	// default or an extension of it — never a shrink.
	window, err := anchor.ComputeWindow(sched, anc.TargetHeight, 0)
	if err != nil {
		return preconditionErr(err)
	}
	if compactRep.Window.Target != anc.TargetHeight {
		return preconditionErr(fmt.Errorf("verify-db: compact.json window target %d != anchor target %d",
			compactRep.Window.Target, anc.TargetHeight))
	}
	if compactRep.Window.RetainFrom != window.RetainFrom {
		window, err = anchor.ComputeWindow(sched, anc.TargetHeight, compactRep.Window.RetainFrom)
		if err != nil {
			return preconditionErr(fmt.Errorf("verify-db: compact.json records an invalid window: %w", err))
		}
	}
	chainConfig, err := harness.ChainConfig(flagNetwork, flagShard)
	if err != nil {
		return usageErr("%v", err)
	}

	// Destination journal state, recorded in the report (package-db gates
	// on it).
	jstate, _, jerr := report.JournalState(report.JournalPath(dbPath))
	if jerr != nil {
		jstate = "MISSING"
	}

	roDB, ro, err := dbopen.OpenSourceDatabase(dbPath)
	if err != nil {
		return err
	}
	roClosed := false
	defer func() {
		if !roClosed {
			ro.Close()
		}
	}()
	db := strictdb.NewWriteRefusing(roDB) // defense in depth on the verification handle

	if tempDir == "" {
		tempDir = os.TempDir()
	}
	result, err := verify.Run(db, verify.Params{
		Network: flagNetwork, ShardID: flagShard, ChainConfig: chainConfig,
		Anchor: anc, AnchorSHA256: anchorRef.SHA256,
		Compact:                       &compactRep,
		MetadataReferenceManifestPath: metaRefManifest,
		Window:                        window,
		TargetIsEpochLast:             sched.EpochLastBlock(window.Epoch) == window.Target,
		TempDir:                       tempDir,
	})
	if err != nil {
		return ioErr(err)
	}

	// §11.2 reopen requirement: clean close, reopen, re-traverse the state;
	// digests must be identical.
	checks := result.Checks
	if result.DigestSet != nil {
		if err := ro.Close(); err != nil {
			return ioErr(fmt.Errorf("verify-db: clean close failed: %w", err))
		}
		roClosed = true
		roDB2, ro2, err := dbopen.OpenSourceDatabase(dbPath)
		if err != nil {
			return err
		}
		defer ro2.Close()
		walk2, err := verify.WalkState(roDB2, common.HexToHash(result.DigestSet.StateRoot), verify.StateWalkOptions{})
		if err != nil {
			checks = append(checks, report.Check{ID: verify.CheckStateReopen, OK: false, Detail: err.Error()})
			result.Passed = false
		} else if walk2.Accounts != result.DigestSet.Accounts ||
			walk2.StorageSlots != result.DigestSet.StorageSlots ||
			walk2.Codes != result.DigestSet.Codes {
			checks = append(checks, report.Check{ID: verify.CheckStateReopen, OK: false, Detail: "state digests differ across reopen"})
			result.Passed = false
		} else {
			checks = append(checks, report.Check{ID: verify.CheckStateReopen, OK: true})
		}
	}

	meta, err := report.NewMeta(report.VerificationSchemaV1, "verify-db", flagNetwork, flagShard, toolVersion(), inputs)
	if err != nil {
		return ioErr(err)
	}
	rep := &report.VerificationReport{
		Meta:   meta,
		DBPath: dbPath,
		Mode:   compactRep.Mode,
		Checks: checks,
		Passed: result.Passed,

		DigestSet:               result.DigestSet,
		LogicalKVDigest:         logicalOf(result),
		NormalizedOutputDigest:  result.NormalizedOutput,
		MetadataReferenceDigest: compactRep.MetadataReferenceDigest,

		CertificatesVerified: result.CertificatesVerified,
		WallSeconds:          time.Since(start).Seconds(),
		JournalState:         jstate,
	}
	sum, err := report.WriteJSON(output, rep)
	if err != nil {
		return ioErr(err)
	}
	failedIDs := []string{}
	for _, c := range rep.Checks {
		if !c.OK {
			failedIDs = append(failedIDs, c.ID)
		}
	}
	fmt.Printf("verify-db: passed=%v (%d checks, %d certificates verified); verification.json %s (sha256 %s)\n",
		rep.Passed, len(rep.Checks), rep.CertificatesVerified, output, sum)
	if !rep.Passed {
		return verificationErr(fmt.Errorf("verify-db: checks failed: %v", failedIDs))
	}
	return nil
}

func logicalOf(res *verify.Result) string {
	if res.Logical == nil {
		return ""
	}
	return res.Logical.Total.SHA256
}
