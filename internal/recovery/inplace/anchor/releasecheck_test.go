//go:build releasecheck

package anchor

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
)

// TestReleaseCrossCheckTargetHash is the pre-release cross-check build step:
// it fetches the target block from the public explorer/API endpoints and
// asserts the returned hash equals the compiled constant.
//
// It is guarded by the releasecheck build tag because it needs network
// access; the releaser runs it manually:
//
//	go test -tags releasecheck -run TestReleaseCrossCheck ./internal/recovery/inplace/anchor/...
//
// Validators never run it; CI never needs network.
func TestReleaseCrossCheckTargetHash(t *testing.T) {
	endpoints := []string{
		"https://api.harmony.one",
		"https://api.s0.t.hmny.io",
	}
	reqBody := fmt.Sprintf(
		`{"jsonrpc":"2.0","id":1,"method":"hmyv2_getBlockByNumber","params":[%d,{}]}`,
		MainnetTargetHeight,
	)
	client := &http.Client{Timeout: 30 * time.Second}
	var lastErr error
	for _, url := range endpoints {
		resp, err := client.Post(url, "application/json", bytes.NewBufferString(reqBody))
		if err != nil {
			lastErr = err
			continue
		}
		var parsed struct {
			Result struct {
				Hash   string `json:"hash"`
				Number uint64 `json:"number"`
			} `json:"result"`
			Error *struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		err = json.NewDecoder(resp.Body).Decode(&parsed)
		resp.Body.Close()
		if err != nil {
			lastErr = err
			continue
		}
		if parsed.Error != nil {
			lastErr = fmt.Errorf("%s: rpc error: %s", url, parsed.Error.Message)
			continue
		}
		if parsed.Result.Number != MainnetTargetHeight {
			t.Fatalf("%s returned block %d, requested %d", url, parsed.Result.Number, MainnetTargetHeight)
		}
		if !strings.EqualFold(parsed.Result.Hash, MainnetTargetHashHex) {
			t.Fatalf("compiled target hash %s does NOT match %s block %d hash %s",
				MainnetTargetHashHex, url, MainnetTargetHeight, parsed.Result.Hash)
		}
		t.Logf("compiled target hash confirmed by %s at height %d", url, MainnetTargetHeight)
		return
	}
	t.Fatalf("no endpoint reachable: %v", lastErr)
}
