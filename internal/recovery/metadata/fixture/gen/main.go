// Command gen regenerates the committed metadata acceptance fixture kit
// (default testdata/recovery/metadata/kit). The kit is byte-reproducible
// (deterministic block times, fixture-only BLS secrets, and canonicalized
// LevelDB directories — sorted-order reinsertion makes the physical bytes a
// pure function of the logical content), so a fresh run reproduces the
// committed tree exactly. Layout (plan WS7):
//
//	kit/harmony_db_0/         — the dirty chain (head 48, target 30, the
//	                            abandoned branch with post-target staking
//	                            and the 0xfc precompile delegation matrix)
//	kit/clean/harmony_db_0/   — the clean twin (the same deterministic
//	                            chain ended at the target, no branch)
//	kit/fixture-keys/         — fixture-only BLS secrets (public test keys)
//	kit/recovery-anchor.localnet.json — the anchor config for the kit
//	kit/reference/            — golden export artifacts over the dirty DB
//	                            (metadata-30.hmr, metadata-30.reference.json,
//	                            run-checksums.sha256)
//	kit/ground-truth.json     — target tuple + reference digest summary
//
// The acceptance suite also generates fixtures in-process; this kit is the
// committed golden the suite pins against (acceptance.TestCommittedKitGolden
// and TestKitRegeneratesByteIdentical), the devops-pilot input and the
// manual-inspection artifact (§5.7). The generation logic lives in
// metafixture.GenerateKit so there is exactly one code path.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	metafixture "github.com/harmony-one/harmony/internal/recovery/metadata/fixture"
)

func main() {
	outRoot := "testdata/recovery/metadata/kit"
	if len(os.Args) > 1 {
		outRoot = os.Args[1]
	}
	abs, err := filepath.Abs(outRoot)
	if err != nil {
		fmt.Fprintf(os.Stderr, "resolve out root: %v\n", err)
		os.Exit(1)
	}
	if err := metafixture.GenerateKit(abs, metafixture.RepoKeysDir(), os.Stderr); err != nil {
		fmt.Fprintf(os.Stderr, "gen: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("metadata fixture kit written to %s\n", outRoot)
}
