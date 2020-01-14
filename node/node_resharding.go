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

/*
func (node *Node) transitionIntoNextEpoch(shardState types.State) {
	logger = logger.New(
		"blsPubKey", hex.EncodeToString(node.Consensus.PubKey.Serialize()),
		"curShard", node.Blockchain().ShardID(),
		"curLeader", node.Consensus.IsLeader())
	for _, c := range shardState {
		utils.Logger().Debug().
			Uint32("shardID", c.ShardID).
			Str("nodeList", c.Slots).
         Msg("new shard information")
	}
	myShardID, isNextLeader := findRoleInShardState(
		node.Consensus.PubKey, shardState)
	logger = logger.New(
		"nextShard", myShardID,
		"nextLeader", isNextLeader)

	if myShardID == math.MaxUint32 {
		getLogger().Info("Somehow I got kicked out. Exiting")
		os.Exit(8) // 8 represents it's a loop and the program restart itself
	}

	myShardState := shardState[myShardID]

	// Update public keys
	var publicKeys []*bls.PublicKey
	for idx, nodeID := range myShardState.Slots {
		key := &bls.PublicKey{}
		err := key.Deserialize(nodeID.BlsPublicKey[:])
		if err != nil {
			getLogger().Error("Failed to deserialize BLS public key in shard state",
				"idx", idx,
				"error", err)
		}
		publicKeys = append(publicKeys, key)
	}
	node.Consensus.UpdatePublicKeys(publicKeys)
	//	node.DRand.UpdatePublicKeys(publicKeys)

	if node.Blockchain().ShardID() == myShardID {
		getLogger().Info("staying in the same shard")
	} else {
		getLogger().Info("moving to another shard")
		if err := node.shardChains.Close(); err != nil {
			getLogger().Error("cannot close shard chains", "error", err)
		}
		restartProcess(getRestartArguments(myShardID))
	}
}
*/

func findRoleInShardState(
	key *bls.PublicKey, state shard.State,
) (shardID uint32, isLeader bool) {
	keyBytes := key.Serialize()
	for idx, shard := range state.Shards {
		for nodeIdx, nodeID := range shard.Slots {
			if bytes.Compare(nodeID.BlsPublicKey[:], keyBytes) == 0 {
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
