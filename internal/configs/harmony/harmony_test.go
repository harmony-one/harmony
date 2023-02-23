package harmony

import (
	"fmt"
	"testing"
	"time"

	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/stretchr/testify/assert"
)

func TestToRPCServerConfig(t *testing.T) {
	tests := []struct {
		input  HarmonyConfig
		output nodeconfig.RPCServerConfig
	}{
		{
			input: HarmonyConfig{
				HTTP: HttpConfig{
					Enabled:        true,
					RosettaEnabled: false,
					IP:             "127.0.0.1",
					Port:           nodeconfig.DefaultRPCPort,
					AuthPort:       nodeconfig.DefaultAuthRPCPort,
					RosettaPort:    nodeconfig.DefaultRosettaPort,
					ReadTimeout:    "-1",
					WriteTimeout:   "-2",
					IdleTimeout:    "-3",
				},
				WS: WsConfig{
					Enabled:  true,
					IP:       "127.0.0.1",
					Port:     nodeconfig.DefaultWSPort,
					AuthPort: nodeconfig.DefaultAuthWSPort,
				},
				RPCOpt: RpcOptConfig{
					DebugEnabled:       false,
					EthRPCsEnabled:     true,
					StakingRPCsEnabled: true,
					LegacyRPCsEnabled:  true,
					RpcFilterFile:      "./.hmy/rpc_filter.txt",
					RateLimterEnabled:  true,
					RequestsPerSecond:  nodeconfig.DefaultRPCRateLimit,
					EvmCallTimeout:     "-4",
				},
			},
			output: nodeconfig.RPCServerConfig{
				HTTPEnabled:        true,
				HTTPIp:             "127.0.0.1",
				HTTPPort:           nodeconfig.DefaultRPCPort,
				HTTPAuthPort:       nodeconfig.DefaultAuthRPCPort,
				HTTPTimeoutRead:    30 * time.Second,
				HTTPTimeoutWrite:   30 * time.Second,
				HTTPTimeoutIdle:    120 * time.Second,
				WSEnabled:          true,
				WSIp:               "127.0.0.1",
				WSPort:             nodeconfig.DefaultWSPort,
				WSAuthPort:         nodeconfig.DefaultAuthWSPort,
				DebugEnabled:       false,
				EthRPCsEnabled:     true,
				StakingRPCsEnabled: true,
				LegacyRPCsEnabled:  true,
				RpcFilterFile:      "./.hmy/rpc_filter.txt",
				RateLimiterEnabled: true,
				RequestsPerSecond:  nodeconfig.DefaultRPCRateLimit,
				EvmCallTimeout:     5 * time.Second,
			},
		},
	}
	for i, tt := range tests {
		assertObject := assert.New(t)
		name := fmt.Sprintf("TestToRPCServerConfig: #%d", i)
		t.Run(name, func(t *testing.T) {
			assertObject.Equal(
				tt.input.ToRPCServerConfig(),
				tt.output,
				name,
			)
		})
	}
}
