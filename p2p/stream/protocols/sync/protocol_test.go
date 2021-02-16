package sync

import (
	"context"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p-core/discovery"
	libp2p_peer "github.com/libp2p/go-libp2p-core/peer"
)

func TestProtocol_Match(t *testing.T) {
	tests := []struct {
		targetID string
		exp      bool
	}{
		{"harmony/sync/unitest/0/1.0.1", true},
		{"h123456", false},
		{"harmony/sync/unitest/0/0.9.9", false},
		{"harmony/epoch/unitest/0/1.0.1", false},
		{"harmony/sync/mainnet/0/1.0.1", false},
		{"harmony/sync/unitest/1/1.0.1", false},
	}

	for i, test := range tests {
		p := &Protocol{
			config: Config{
				Network: "unitest",
				ShardID: 0,
			},
		}

		res := p.Match(test.targetID)

		if res != test.exp {
			t.Errorf("Test %v: unexpected result %v / %v", i, res, test.exp)
		}
	}
}

func TestProtocol_advertiseLoop(t *testing.T) {
	disc := newTestDiscovery(100 * time.Millisecond)
	p := &Protocol{
		disc:   disc,
		closeC: make(chan struct{}),
	}

	go p.advertiseLoop()

	time.Sleep(150 * time.Millisecond)
	close(p.closeC)

	if len(disc.advCnt) != len(p.supportedVersions()) {
		t.Errorf("unexpected advertise topic count: %v / %v", len(disc.advCnt),
			len(p.supportedVersions()))
	}
	for _, cnt := range disc.advCnt {
		if cnt < 1 {
			t.Errorf("unexpected discovery count: %v", cnt)
		}
	}
}

type testDiscovery struct {
	advCnt map[string]int
	sleep  time.Duration
}

func newTestDiscovery(discInterval time.Duration) *testDiscovery {
	return &testDiscovery{
		advCnt: make(map[string]int),
		sleep:  discInterval,
	}
}

func (disc *testDiscovery) Start() error {
	return nil
}

func (disc *testDiscovery) Close() error {
	return nil
}

func (disc *testDiscovery) Advertise(ctx context.Context, ns string) (time.Duration, error) {
	disc.advCnt[ns]++
	return disc.sleep, nil
}

func (disc *testDiscovery) FindPeers(ctx context.Context, ns string, peerLimit int) (<-chan libp2p_peer.AddrInfo, error) {
	return nil, nil
}

func (disc *testDiscovery) GetRawDiscovery() discovery.Discovery {
	return nil
}
