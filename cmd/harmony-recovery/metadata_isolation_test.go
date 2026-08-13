package main

import (
	"fmt"
	"net"
	"path/filepath"
	"testing"
	"time"

	metafixture "github.com/harmony-one/harmony/internal/recovery/metadata/fixture"
)

// harmonyServicePorts are the default harmony service ports: 9000 p2p,
// 9500 rpc, 9800 ws, 9900 prometheus.
var harmonyServicePorts = []int{9000, 9500, 9800, 9900}

// bindableServicePorts returns the harmony service ports that are free at
// baseline (ports occupied by unrelated processes are excluded from the
// isolation probes rather than failing the test).
func bindableServicePorts(t *testing.T) []int {
	t.Helper()
	var free []int
	for _, port := range harmonyServicePorts {
		ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
		if err != nil {
			t.Logf("port %d busy at baseline (external process?); excluded from isolation probes", port)
			continue
		}
		_ = ln.Close()
		free = append(free, port)
	}
	return free
}

// TestMetadataProcessIsolation asserts the metadata subcommands never
// initialize networking or a serve loop (in-place handoff §4 safety
// contract; complements the static dependency guard). Two layers:
//
//  1. Fast-error runs of every subcommand return promptly (no blocking
//     Serve loop and no listener held afterward).
//  2. A REAL, successful scan over a genuine fixture chain — the code path
//     that links core/chain/EVM — is sampled WHILE it runs: every free
//     harmony service port must remain bindable at every sample point
//     during the working run, not just before/after a fast failure.
func TestMetadataProcessIsolation(t *testing.T) {
	if testing.Short() {
		t.Skip("fixture generation is not short")
	}
	freePorts := bindableServicePorts(t)

	// Layer 1: fast-error prompt return.
	subs := [][]string{
		{"metadata", "scan", "--db", "/nonexistent/harmony_db_0", "--anchor", "/nonexistent/anchor.json", "--report", t.TempDir() + "/r.json"},
		{"metadata", "export-reference", "--db", "/nonexistent/harmony_db_0", "--anchor", "/nonexistent/anchor.json", "--out-dir", t.TempDir()},
		{"metadata", "audit-branch", "--db", "/nonexistent/harmony_db_0", "--anchor", "/nonexistent/anchor.json", "--out-dir", t.TempDir(), "--scratch", t.TempDir()},
	}
	for _, args := range subs {
		done := make(chan int, 1)
		go func(a []string) {
			code, _, _ := runRecovery(a...)
			done <- code
		}(args)
		select {
		case <-done:
			// Returned promptly: no serve loop, no blocking listener.
		case <-time.After(15 * time.Second):
			t.Fatalf("%v did not return promptly (a serve loop / listener would block)", args)
		}
	}

	// Layer 2: a real successful scan, sampled while it works.
	dir := filepath.Join(t.TempDir(), "harmony_db_0")
	c, err := metafixture.Open(dir, metafixture.RepoKeysDir())
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	if err := c.Generate(metafixture.Spec{
		Blocks: 44, CreateValidatorAt: 22, DelegateAt: 26,
		PostCreateValidatorAt: 40, PostDelegateAt: 42,
	}); err != nil {
		t.Fatalf("generate fixture: %v", err)
	}
	if err := c.Finalize(); err != nil {
		t.Fatalf("finalize fixture: %v", err)
	}
	anchorPath := filepath.Join(t.TempDir(), "recovery-anchor.json")
	if err := metafixture.WriteAnchorConfig(dir, 30, 44, nil, anchorPath); err != nil {
		t.Fatal(err)
	}

	// Tight bind/close sampler loop (no ticker: the localnet scan can
	// finish in tens of milliseconds and must still be sampled mid-run).
	stop := make(chan struct{})
	sampled := make(chan error, 1)
	go func() {
		var firstErr error
		samples := 0
		for {
			select {
			case <-stop:
				if firstErr == nil && samples == 0 && len(freePorts) > 0 {
					firstErr = fmt.Errorf("port sampler took no samples (scan too fast?)")
				}
				sampled <- firstErr
				return
			default:
				for _, port := range freePorts {
					ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
					if err != nil {
						if firstErr == nil {
							firstErr = fmt.Errorf("port %d became occupied during a working scan: %v", port, err)
						}
						continue
					}
					_ = ln.Close()
					samples++
				}
				time.Sleep(time.Millisecond)
			}
		}
	}()

	code, _, _ := runRecovery("metadata", "scan",
		"--db", dir, "--anchor", anchorPath,
		"--report", filepath.Join(t.TempDir(), "scan-report.json"))
	close(stop)
	if err := <-sampled; err != nil {
		t.Fatal(err)
	}
	if code != 0 {
		t.Fatalf("real scan over the fixture must succeed for the isolation probe, got exit %d", code)
	}

	// And still no listener afterward.
	for _, port := range freePorts {
		ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
		if err != nil {
			t.Fatalf("port %d is held after the runs: %v", port, err)
		}
		_ = ln.Close()
	}
}
