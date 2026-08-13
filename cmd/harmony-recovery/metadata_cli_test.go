package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/spf13/pflag"

	"github.com/harmony-one/harmony/internal/recovery/inplace/anchor"
)

// runRecovery drives the CLI through run() capturing stdout/stderr.
func runRecovery(args ...string) (int, string, string) {
	var stdout, stderr bytes.Buffer
	code := run(args, &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

// TestRootHelpListsBothFamilies pins that the root help lists preflight and
// the metadata group, and that the root Short/Long no longer read
// preflight-only.
func TestRootHelpListsBothFamilies(t *testing.T) {
	_, out, _ := runRecovery("--help")
	if !strings.Contains(out, "preflight") {
		t.Fatal("root help must still list preflight")
	}
	if !strings.Contains(out, "metadata") {
		t.Fatal("root help must list the metadata group")
	}
}

// TestMetadataHelpListsThreeSubcommands pins that `metadata --help` lists
// exactly scan, export-reference, audit-branch.
func TestMetadataHelpListsThreeSubcommands(t *testing.T) {
	_, out, _ := runRecovery("metadata", "--help")
	for _, sub := range []string{"scan", "export-reference", "audit-branch"} {
		if !strings.Contains(out, sub) {
			t.Fatalf("metadata help missing subcommand %q\n%s", sub, out)
		}
	}
}

// TestPreflightPreservationPin is the WS1 preflight-preservation pin: the
// preflight command's registered flag set and its exit-code contract are
// byte-identical after the metadata registration edit. No new root-global
// flags exist (metadata flags are all group-local).
func TestPreflightPreservationPin(t *testing.T) {
	// Preflight flag set (name + shorthand + default), sorted.
	var preflightFlags []string
	newPreflightCommand().Flags().VisitAll(func(f *pflag.Flag) {
		preflightFlags = append(preflightFlags, f.Name+"="+f.DefValue)
	})
	sort.Strings(preflightFlags)
	want := []string{
		"db-cache-mb=256", "db=", "handles=512", "name=",
		"network=mainnet", "report=preflight-result.json", "shard=0",
		"storage-workers=0", "target-hash=", "target-height=0", "trie-cache-mb=256",
	}
	if strings.Join(preflightFlags, ",") != strings.Join(want, ",") {
		t.Fatalf("preflight flag set drifted:\n got %v\nwant %v", preflightFlags, want)
	}

	// No root-global (persistent) flags were introduced.
	root := newRootCommand()
	var persistent []string
	root.PersistentFlags().VisitAll(func(f *pflag.Flag) { persistent = append(persistent, f.Name) })
	if len(persistent) != 0 {
		t.Fatalf("root must have no persistent flags, got %v", persistent)
	}
	// The root's local flags are only cobra's built-in help; no domain flags.
	root.LocalFlags().VisitAll(func(f *pflag.Flag) {
		if f.Name != "help" && f.Name != "version" {
			t.Fatalf("unexpected root-local flag %q", f.Name)
		}
	})
}

// TestPreflightExitContractPreserved re-checks the preflight 0/1/2/3 exit
// contract is untouched: an unusable invocation (missing --db) exits 2.
func TestPreflightExitContractPreserved(t *testing.T) {
	// Preflight's unusable exit is 2 (report.ExitUnusable in the
	// inplace/report package); a missing --db is a cobra required-flag
	// parse error, which the root maps to 2.
	code, _, _ := runRecovery("preflight")
	if code != 2 {
		t.Fatalf("preflight without --db exits %d, want 2 (unusable)", code)
	}
}

// TestMetadataExitDelivery checks a metadata invocation error maps to the
// metadata table (not preflight's 0/1/2/3): a missing --anchor is exit 2
// from cobra's required-flag parse (root contract), while a validation
// failure inside RunE is exit 15.
func TestMetadataFlagParseExit(t *testing.T) {
	// Missing required flags: cobra returns a plain error -> ExitUnusable
	// (2) at the landed root contract (§4.1).
	code, _, _ := runRecovery("metadata", "scan")
	if code != 2 {
		t.Fatalf("metadata scan without required flags exits %d, want 2 (cobra parse error)", code)
	}
}

// TestAnchorDriftPin pins the shipped mainnet recovery-anchor.json against
// the compiled importable symbols (§4.1 WS1 drift test).
func TestAnchorDriftPin(t *testing.T) {
	path := "../../docs/recovery/recovery-anchor.mainnet.json"
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("shipped mainnet anchor config missing at %s: %v", path, err)
	}
	var cfg struct {
		Network      string `json:"network"`
		TargetHeight uint64 `json:"target_height"`
		TargetHash   string `json:"target_hash"`
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.Network != "mainnet" {
		t.Fatalf("shipped config network %q, want mainnet", cfg.Network)
	}
	if cfg.TargetHeight != anchor.MainnetTargetHeight {
		t.Fatalf("shipped target_height %d != compiled %d", cfg.TargetHeight, anchor.MainnetTargetHeight)
	}
	if cfg.TargetHash != anchor.MainnetTargetHashHex {
		t.Fatalf("shipped target_hash %s != compiled %s", cfg.TargetHash, anchor.MainnetTargetHashHex)
	}
}

// TestGoldenHelpFiles pins root and metadata help output against committed
// goldens (byte-stable CLI contract; regenerate with UPDATE_GOLDEN=1).
func TestGoldenHelpFiles(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"root-help.txt", []string{"--help"}},
		{"metadata-help.txt", []string{"metadata", "--help"}},
		{"preflight-help.txt", []string{"preflight", "--help"}},
	}
	dir := "../../testdata/recovery/metadata/golden"
	for _, c := range cases {
		_, out, _ := runRecovery(c.args...)
		// Strip the version line (build-stamp dependent) from root help.
		out = stripVersionLine(out)
		path := filepath.Join(dir, c.name)
		if os.Getenv("UPDATE_GOLDEN") == "1" {
			_ = os.MkdirAll(dir, 0o755)
			if err := os.WriteFile(path, []byte(out), 0o644); err != nil {
				t.Fatal(err)
			}
			continue
		}
		want, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("golden %s missing (run UPDATE_GOLDEN=1): %v", c.name, err)
		}
		if out != string(want) {
			t.Fatalf("%s drifted from golden:\n--- got ---\n%s\n--- want ---\n%s", c.name, out, want)
		}
	}
}

// TestMetadataDocMatchesFlags asserts docs/recovery/metadata.md documents
// every visible flag of every metadata subcommand (WS8 automated check).
func TestMetadataDocMatchesFlags(t *testing.T) {
	raw, err := os.ReadFile("../../docs/recovery/metadata.md")
	if err != nil {
		t.Fatalf("metadata doc missing: %v", err)
	}
	doc := string(raw)
	root := newMetadataCommand()
	for _, sub := range root.Commands() {
		sub.Flags().VisitAll(func(f *pflag.Flag) {
			if f.Hidden {
				return
			}
			if !strings.Contains(doc, "--"+f.Name) {
				t.Errorf("metadata %s flag --%s is not documented in metadata.md", sub.Name(), f.Name)
			}
		})
	}
}

func stripVersionLine(s string) string {
	lines := strings.Split(s, "\n")
	var kept []string
	for _, l := range lines {
		if strings.HasPrefix(l, "harmony-recovery version ") {
			continue
		}
		kept = append(kept, l)
	}
	return strings.Join(kept, "\n")
}
