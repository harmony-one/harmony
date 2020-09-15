package hmy

import (
	//   "errors"
	"os"
	"time"

	//	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	//	commonRPC "github.com/harmony-one/harmony/rpc/common"
	"github.com/harmony-one/harmony/internal/utils"
)

const (
	safeExit    = 10000
	abortExit   = 10001
	killTimeout = 30 * time.Second
)

// KillNode ...
func (hmy *Harmony) KillNode(safe bool) string {
	utils.Logger().Info().Bool("safeKill", safe).Msg("KillNode")
	if safe {
		hmy.NodeAPI.ShutDown()
		defer os.Exit(safeExit)
	} else {
		defer os.Exit(abortExit)
	}
	return "Killed"
}

// KillLeader ...
func (hmy *Harmony) KillLeader(safe bool) string {
	utils.Logger().Info().Bool("safeKill", safe).Msg("KillLeader")
	if !hmy.NodeAPI.IsCurrentlyLeader() {
		return "Not Leader"
	}
	return hmy.KillNode(safe)
}

// KillNodeCond kills the node in different condition
// kill the node in specified phase, mode, blocknumber, and within timeout seconds
// string * matches anything
// block 0 matches any block
func (hmy *Harmony) KillNodeCond(safe bool, phase string, mode string, block uint64, timeout uint) string {
	finish := time.Now().Add(time.Duration(timeout) * time.Second)
	utils.Logger().Info().Bool("safeKill", safe).
		Str("phase", phase).
		Str("mode", mode).
		Uint64("block", block).
		Uint("timeout", timeout).
		Msg("KillNodeCond")

	for time.Now().Before(finish) {
		if (mode == "*" || hmy.NodeAPI.GetConsensusMode() == mode) &&
			(phase == "*" || hmy.NodeAPI.GetConsensusPhase() == phase) &&
			(block == 0 || hmy.CurrentBlock().NumberU64() == block) {
			return hmy.KillNode(safe)
		}
		time.Sleep(10 * time.Millisecond)
	}
	return "Doesn't match. Not killed within timeout."
}
