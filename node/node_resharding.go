package node

import (
	"bytes"
	"math"
	"os"
	"os/exec"
	"strconv"
	"syscall"

	"github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/shard"
)

func findRoleInShardState(
	key *bls.PublicKey, state shard.State,
) (shardID uint32, isLeader bool) {
	keyBytes := key.Serialize()
	for idx, shard := range state {
		for nodeIdx, nodeID := range shard.NodeList {
			if bytes.Compare(nodeID.BLSPublicKey[:], keyBytes) == 0 {
				return uint32(idx), nodeIdx == 0
			}
		}
	}
	return math.MaxUint32, false
}

func restartProcess(args []string) {
	execFile, err := getBinaryPath()
	if err != nil {
		utils.Logger().Error().
			Err(err).
			Str("file", execFile).
			Msg("Failed to get program path when restarting program")
	}
	utils.Logger().Info().
		Strs("args", args).
		Strs("env", os.Environ()).
		Msg("Restarting program")
	err = syscall.Exec(execFile, args, os.Environ())
	if err != nil {
		utils.Logger().Error().
			Err(err).
			Msg("Failed to restart program after resharding")
	}
	panic("syscall.Exec() is not supposed to return")
}

func getRestartArguments(myShardID uint32) []string {
	args := os.Args
	hasShardID := false
	shardIDFlag := "-shard_id"
	// newNodeFlag := "-is_newnode"
	for i, arg := range args {
		if arg == shardIDFlag {
			if i+1 < len(args) {
				args[i+1] = strconv.Itoa(int(myShardID))
			} else {
				args = append(args, strconv.Itoa(int(myShardID)))
			}
			hasShardID = true
		}
		// TODO: enable this
		//if arg == newNodeFlag {
		//	args[i] = ""  // remove new node flag
		//}
	}
	if !hasShardID {
		args = append(args, shardIDFlag)
		args = append(args, strconv.Itoa(int(myShardID)))
	}
	return args
}

// Gets the path of this currently running binary program.
func getBinaryPath() (argv0 string, err error) {
	argv0, err = exec.LookPath(os.Args[0])
	if nil != err {
		return
	}
	if _, err = os.Stat(argv0); nil != err {
		return
	}
	return
}
