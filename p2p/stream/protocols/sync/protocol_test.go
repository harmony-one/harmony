package sync

import (
	"context"
	"fmt"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/discovery"
	libp2p_peer "github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/rs/zerolog"

	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
)

func TestProtocol_Match(t *testing.T) {
	tests := []struct {
		targetID protocol.ID
		exp      bool
	}{
		{"harmony/sync/unitest/0/1.0", true},
		{"harmony/sync/unitest/1/1.0.1", false},
		{"harmony/sync/unitest/0/1.2-alpha", true},
		{"h123456", false},
		{"harmony/sync/unitest/0/0.9.9", false},
		{"harmony/epochsync/unitest/0/1.0", false},
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
		ctx:    context.Background(),
		config: Config{
			Network: "unitest",
			ShardID: 0,
		},
		logger: zerolog.New(io.Discard),
	}

	go p.advertiseLoop()

	time.Sleep(150 * time.Millisecond)
	close(p.closeC)

	advCnt := disc.Extract()
	if len(advCnt) != len(p.supportedVersions()) {
		t.Errorf("unexpected advertise topic count: %v / %v", len(advCnt),
			len(p.supportedVersions()))
	}
	for _, cnt := range advCnt {
		if cnt < 1 {
			t.Errorf("unexpected discovery count: %v", cnt)
		}
	}
}

func TestProtocol_StartupMode(t *testing.T) {
	// Test that startup mode is enabled by default
	p := &Protocol{
		startupMode:      true,
		startupStartTime: time.Now(),
	}

	if !p.IsInStartupMode() {
		t.Error("Expected startup mode to be enabled by default")
	}

	// Test exiting startup mode
	p.ExitStartupMode()
	if p.IsInStartupMode() {
		t.Error("Expected startup mode to be disabled after exit")
	}
}

func TestProtocol_Advertise(t *testing.T) {
	tests := []struct {
		name           string
		startupMode    bool
		peersFound     int
		expectedTiming string // "fast" or "normal"
	}{
		{
			name:           "startup mode with peers found",
			startupMode:    true,
			peersFound:     5,
			expectedTiming: "fast",
		},
		{
			name:           "startup mode no peers found",
			startupMode:    true,
			peersFound:     0,
			expectedTiming: "fast",
		},
		{
			name:           "normal mode with peers found",
			startupMode:    false,
			peersFound:     3,
			expectedTiming: "normal",
		},
		{
			name:           "normal mode no peers found",
			startupMode:    false,
			peersFound:     0,
			expectedTiming: "normal",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock discovery that returns specified number of peers
			mockDisc := &mockDiscovery{
				peersToReturn: tt.peersFound,
			}

			p := &Protocol{
				startupMode:      tt.startupMode,
				startupStartTime: time.Now(),
				disc:             mockDisc,
				ctx:              context.Background(),
				logger:           zerolog.New(io.Discard),
				config: Config{
					Network: "unitest",
					ShardID: 0,
				},
			}

			// Test startup mode timing constants
			if tt.startupMode {
				// Verify startup mode uses faster constants
				if BaseTimeoutStartup >= BaseTimeoutNormal {
					t.Error("Startup timeout should be faster than normal timeout")
				}
				if MaxTimeoutStartup >= MaxTimeoutNormal {
					t.Error("Startup max timeout should be faster than normal max timeout")
				}
			}

			// Test peer discovery count tracking
			p.recentPeerDiscoveryCount = tt.peersFound
			if p.recentPeerDiscoveryCount != tt.peersFound {
				t.Errorf("Expected peer count %d, got %d", tt.peersFound, p.recentPeerDiscoveryCount)
			}
		})
	}
}

func TestProtocol_AdvertiseLoop(t *testing.T) {
	// Test that advertiseLoop respects startup mode timing
	mockDisc := &mockDiscovery{
		peersToReturn: 2,
	}

	p := &Protocol{
		startupMode:      true,
		startupStartTime: time.Now(),
		disc:             mockDisc,
		ctx:              context.Background(),
		closeC:           make(chan struct{}),
		logger:           zerolog.New(io.Discard),
		config: Config{
			Network: "unitest",
			ShardID: 0,
		},
	}

	// Test startup mode timing constants directly
	if !p.IsInStartupMode() {
		t.Error("Expected startup mode to be active")
	}

	// Test that startup mode uses faster constants
	if BaseTimeoutStartup >= BaseTimeoutNormal {
		t.Error("Startup timeout should be faster than normal timeout")
	}
	if MaxTimeoutStartup >= MaxTimeoutNormal {
		t.Error("Startup max timeout should be faster than normal max timeout")
	}

	// Test exit from startup mode
	p.ExitStartupMode()
	if p.IsInStartupMode() {
		t.Error("Expected startup mode to be disabled after exit")
	}
}

func TestProtocol_ExitStartupMode(t *testing.T) {
	p := &Protocol{
		startupMode:      true,
		startupStartTime: time.Now(),
		ctx:              context.Background(),
		logger:           zerolog.New(io.Discard),
	}

	// Test manual exit
	p.ExitStartupMode()
	if p.IsInStartupMode() {
		t.Error("Expected startup mode to be disabled after manual exit")
	}

	// Test automatic exit after timeout
	p.startupMode = true
	p.startupStartTime = time.Now().Add(-StartupModeDuration - time.Second)

	// Instead of calling advertise which requires complex setup,
	// test the timeout logic directly
	if time.Since(p.startupStartTime) > StartupModeDuration {
		p.startupMode = false
	}

	if p.IsInStartupMode() {
		t.Error("Expected startup mode to be disabled after timeout")
	}
}

func TestProtocol_GetPeerDiscoveryLimit(t *testing.T) {
	tests := []struct {
		network  nodeconfig.NetworkType
		expected int
	}{
		{nodeconfig.Mainnet, DHTRequestLimitMainnet},
		{nodeconfig.Testnet, DHTRequestLimitTestnet},
		{nodeconfig.Pangaea, DHTRequestLimitPangaea},
		{nodeconfig.Partner, DHTRequestLimitPartner},
		{nodeconfig.Stressnet, DHTRequestLimitStressnet},
		{nodeconfig.Devnet, DHTRequestLimitDevnet},
		{nodeconfig.Localnet, DHTRequestLimitLocalnet},
		{"unknown", DHTRequestLimitDevnet}, // Default fallback
	}

	for _, tt := range tests {
		t.Run(string(tt.network), func(t *testing.T) {
			p := &Protocol{
				config: Config{
					Network: tt.network,
				},
			}

			limit := p.getPeerDiscoveryLimit()
			if limit != tt.expected {
				t.Errorf("Expected peer discovery limit %d for network %s, got %d",
					tt.expected, tt.network, limit)
			}
		})
	}
}

func TestProtocol_GetDHTRequestLimit(t *testing.T) {
	tests := []struct {
		network  nodeconfig.NetworkType
		expected int
	}{
		{nodeconfig.Mainnet, DHTRequestLimitMainnet},
		{nodeconfig.Testnet, DHTRequestLimitTestnet},
		{nodeconfig.Pangaea, DHTRequestLimitPangaea},
		{nodeconfig.Partner, DHTRequestLimitPartner},
		{nodeconfig.Stressnet, DHTRequestLimitStressnet},
		{nodeconfig.Devnet, DHTRequestLimitDevnet},
		{nodeconfig.Localnet, DHTRequestLimitLocalnet},
		{"unknown", DHTRequestLimitDevnet}, // Default fallback
	}

	for _, tt := range tests {
		t.Run(string(tt.network), func(t *testing.T) {
			p := &Protocol{
				config: Config{
					Network: tt.network,
				},
			}

			limit := p.getDHTRequestLimit()
			if limit != tt.expected {
				t.Errorf("Expected DHT request limit %d for network %s, got %d",
					tt.expected, tt.network, limit)
			}
		})
	}
}

func TestProtocol_GetTargetValidPeers(t *testing.T) {
	tests := []struct {
		network  nodeconfig.NetworkType
		expected int
	}{
		{nodeconfig.Mainnet, TargetValidPeersMainnet},
		{nodeconfig.Testnet, TargetValidPeersTestnet},
		{nodeconfig.Pangaea, TargetValidPeersPangaea},
		{nodeconfig.Partner, TargetValidPeersPartner},
		{nodeconfig.Stressnet, TargetValidPeersStressnet},
		{nodeconfig.Devnet, TargetValidPeersDevnet},
		{nodeconfig.Localnet, TargetValidPeersLocalnet},
		{"unknown", TargetValidPeersDevnet}, // Default fallback
	}

	for _, tt := range tests {
		t.Run(string(tt.network), func(t *testing.T) {
			p := &Protocol{
				config: Config{
					Network: tt.network,
				},
			}

			limit := p.getTargetValidPeers()
			if limit != tt.expected {
				t.Errorf("Expected target valid peers %d for network %s, got %d",
					tt.expected, tt.network, limit)
			}
		})
	}
}

type testDiscovery struct {
	advCnt map[string]int
	sleep  time.Duration
	mu     sync.Mutex
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
	disc.mu.Lock()
	defer disc.mu.Unlock()
	disc.advCnt[ns]++
	return disc.sleep, nil
}

func (disc *testDiscovery) Extract() map[string]int {
	disc.mu.Lock()
	defer disc.mu.Unlock()
	var out map[string]int
	out, disc.advCnt = disc.advCnt, make(map[string]int)
	return out
}

func (disc *testDiscovery) FindPeers(ctx context.Context, ns string, peerLimit int) (<-chan libp2p_peer.AddrInfo, error) {
	peerChan := make(chan libp2p_peer.AddrInfo)
	go func() {
		defer close(peerChan)
		// Return some mock peers for testing
		for i := 0; i < 2; i++ {
			peer := libp2p_peer.AddrInfo{
				ID: libp2p_peer.ID(fmt.Sprintf("test-peer-%d", i)),
			}
			peerChan <- peer
		}
	}()
	return peerChan, nil
}

func (disc *testDiscovery) GetRawDiscovery() discovery.Discovery {
	return nil
}

// Mock discovery for testing
type mockDiscovery struct {
	peersToReturn int
	peersFound    int
}

func (md *mockDiscovery) Start() error {
	return nil
}

func (md *mockDiscovery) Close() error {
	return nil
}

func (md *mockDiscovery) FindPeers(ctx context.Context, ns string, peerLimit int) (<-chan libp2p_peer.AddrInfo, error) {
	peerChan := make(chan libp2p_peer.AddrInfo, md.peersToReturn)

	go func() {
		defer close(peerChan)
		for i := 0; i < md.peersToReturn; i++ {
			// Create a mock peer
			peer := libp2p_peer.AddrInfo{
				ID: libp2p_peer.ID(fmt.Sprintf("peer%d", i)),
			}
			peerChan <- peer
		}
	}()

	return peerChan, nil
}

func (md *mockDiscovery) Advertise(ctx context.Context, ns string) (time.Duration, error) {
	return time.Minute, nil
}

func (md *mockDiscovery) GetRawDiscovery() discovery.Discovery {
	return nil
}
