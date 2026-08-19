package main

import (
	"os/exec"
	"strings"
	"testing"
)

// forbiddenDeps is the static dependency guard: a compile-time code-hygiene
// boundary (distinct from any approved-build allowlist). The preflight
// binary must never link networking, RPC, consensus-service wiring or BLS
// keystore loading - it reads a database and verifies signatures, nothing
// else. Exact package or prefix matches against `go list -deps`.
var forbiddenDeps = []string{
	// harmony p2p service / host wiring (the libp2p transport stack hangs
	// off this package; note the type-only go-libp2p/core/{crypto,peer}
	// packages leak in via internal/utils logging helpers and open no
	// sockets - the boundary is harmony's own p2p package)
	"github.com/harmony-one/harmony/p2p",
	// RPC and API services (incl. sync services)
	"github.com/harmony-one/harmony/rpc",
	"github.com/harmony-one/harmony/api/service",
	// node service wiring
	"github.com/harmony-one/harmony/node",
	// consensus service (the engine's verification-only subpackages
	// consensus/engine, consensus/quorum, consensus/signature,
	// consensus/votepower are allowed; the service package itself is not)
	"github.com/harmony-one/harmony/consensus\x00exact",
	// BLS keystore loading (validators' signing keys must never be touched;
	// multibls is a pure key-slice type and is allowed)
	"github.com/harmony-one/harmony/internal/blsgen",
	// libp2p host construction (transport/muxer/swarm - the actual network
	// stack, as opposed to the type-only core packages)
	"github.com/libp2p/go-libp2p\x00exact",
	"github.com/libp2p/go-libp2p/p2p",
}

// exemptDeps are exact-match exemptions consulted before recording a
// violation. The metadata audit-branch engine links package core (the
// masked-overlay re-execution needs the production BlockChain), and core's
// blockchain_pruner_metric.go imports api/service/prometheus purely for
// metric REGISTRATION - nothing in this binary constructs or starts the
// prometheus service (no listener; the process-isolation test enforces
// that). The api/service prefix ban and every other rule stay intact.
var exemptDeps = map[string]bool{
	"github.com/harmony-one/harmony/api/service/prometheus": true,
}

// TestDependencyGuard runs `go list -deps ./cmd/harmony-recovery` and fails
// on forbidden imports.
func TestDependencyGuard(t *testing.T) {
	out, err := exec.Command("go", "list", "-deps", ".").CombinedOutput()
	if err != nil {
		t.Fatalf("go list -deps failed: %v\n%s", err, out)
	}
	deps := strings.Split(strings.TrimSpace(string(out)), "\n")
	depSet := make(map[string]bool, len(deps))
	for _, d := range deps {
		depSet[strings.TrimSpace(d)] = true
	}
	var violations []string
	for _, rule := range forbiddenDeps {
		if exact, ok := strings.CutSuffix(rule, "\x00exact"); ok {
			if depSet[exact] && !exemptDeps[exact] {
				violations = append(violations, exact)
			}
			continue
		}
		for dep := range depSet {
			if (dep == rule || strings.HasPrefix(dep, rule+"/")) && !exemptDeps[dep] {
				violations = append(violations, dep)
			}
		}
	}
	if len(violations) > 0 {
		t.Fatalf("forbidden dependencies linked into harmony-recovery:\n  %s",
			strings.Join(violations, "\n  "))
	}
	// Sanity: the audited verification packages ARE expected.
	for _, want := range []string{
		"github.com/harmony-one/harmony/internal/chain",
		"github.com/harmony-one/harmony/consensus/quorum",
	} {
		if !depSet[want] {
			t.Fatalf("expected dependency %s missing; the dependency guard may be checking the wrong package", want)
		}
	}
}
