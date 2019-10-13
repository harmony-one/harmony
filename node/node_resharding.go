package node

import (
	"bytes"
	"errors"
	"math"
	"math/big"
	"os"
	"os/exec"
	"strconv"
	"syscall"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/harmony-one/bls/ffi/go/bls"
	proto_node "github.com/harmony-one/harmony/api/proto/node"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/types"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/harmony-one/harmony/internal/ctxerror"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p/host"
	"github.com/harmony-one/harmony/shard"
)

// validateNewShardState validate whether the new shard state root matches
func (node *Node) validateNewShardState(block *types.Block) error {
	// Common case first – blocks without resharding proposal
	header := block.Header()
	if header.ShardStateHash() == (common.Hash{}) {
		// No new shard state was proposed
		if block.ShardID() == 0 {
			if core.IsEpochLastBlock(block) {
				// TODO ek - invoke view change
				return errors.New("beacon leader did not propose resharding")
			}
		} else {
			if node.nextShardState.master != nil &&
				!time.Now().Before(node.nextShardState.proposeTime) {
				// TODO ek – invoke view change
				return errors.New("regular leader did not propose resharding")
			}
		}
		// We aren't expecting to reshard, so proceed to sign
		return nil
	}
	shardState := &shard.State{}
	err := rlp.DecodeBytes(header.ShardState(), shardState)
	if err != nil {
		return err
	}
	proposed := *shardState
	if block.ShardID() == 0 {
		// Beacon validators independently recalculate the master state and
		// compare it against the proposed copy.
		nextEpoch := new(big.Int).Add(block.Header().Epoch(), common.Big1)
		// TODO ek – this may be called from regular shards,
		//  for vetting beacon chain blocks received during block syncing.
		//  DRand may or or may not get in the way.  Test this out.
		expected, err := core.CalculateNewShardState(node.Blockchain(), nextEpoch)
		if err != nil {
			return ctxerror.New("cannot calculate expected shard state").
				WithCause(err)
		}
		if shard.CompareShardState(expected, proposed) != 0 {
			// TODO ek – log state proposal differences
			// TODO ek – this error should trigger view change
			err := errors.New("shard state proposal is different from expected")
			// TODO ek/chao – calculated shard state is different even with the
			//  same input, i.e. it is nondeterministic.
			//  Don't treat this as a blocker until we fix the nondeterminism.
			//return err
			ctxerror.Log15(utils.GetLogger().Warn, err)
		}
	} else {
		// Regular validators fetch the local-shard copy on the beacon chain
		// and compare it against the proposed copy.
		//
		// We trust the master proposal in our copy of beacon chain.
		// The sanity check for the master proposal is done earlier,
		// when the beacon block containing the master proposal is received
		// and before it is admitted into the local beacon chain.
		//
		// TODO ek – fetch masterProposal from beaconchain instead
		masterProposal := node.nextShardState.master.ShardState
		expected := masterProposal.FindCommitteeByID(block.ShardID())
		switch len(proposed) {
		case 0:
			// Proposal to discontinue shard
			if expected != nil {
				// TODO ek – invoke view change
				return errors.New(
					"leader proposed to disband against beacon decision")
			}
		case 1:
			// Proposal to continue shard
			proposed := proposed[0]
			// Sanity check: Shard ID should match
			if proposed.ShardID != block.ShardID() {
				// TODO ek – invoke view change
				return ctxerror.New("proposal has incorrect shard ID",
					"proposedShard", proposed.ShardID,
					"blockShard", block.ShardID())
			}
			// Did beaconchain say we are no more?
			if expected == nil {
				// TODO ek – invoke view change
				return errors.New(
					"leader proposed to continue against beacon decision")
			}
			// Did beaconchain say the same proposal?
			if shard.CompareCommittee(expected, &proposed) != 0 {
				// TODO ek – log differences
				// TODO ek – invoke view change
				return errors.New("proposal differs from one in beacon chain")
			}
		default:
			// TODO ek – invoke view change
			return ctxerror.New(
				"regular resharding proposal has incorrect number of shards",
				"numShards", len(proposed))
		}
	}
	return nil
}

func (node *Node) broadcastEpochShardState(newBlock *types.Block) error {
	shardState, err := newBlock.Header().GetShardState()
	if err != nil {
		return err
	}
	epochShardStateMessage := proto_node.ConstructEpochShardStateMessage(
		shard.EpochShardState{
			Epoch:      newBlock.Header().Epoch().Uint64() + 1,
			ShardState: shardState,
		},
	)
	return node.host.SendMessageToGroups(
		[]nodeconfig.GroupID{node.NodeConfig.GetClientGroupID()},
		host.ConstructP2pMessage(byte(0), epochShardStateMessage))
}

func (node *Node) epochShardStateMessageHandler(msgPayload []byte) error {
	epochShardState, err := proto_node.DeserializeEpochShardStateFromMessage(msgPayload)
	if err != nil {
		return ctxerror.New("Can't get shard state message").WithCause(err)
	}
	if node.Consensus == nil {
		return nil
	}
	receivedEpoch := big.NewInt(int64(epochShardState.Epoch))
	utils.Logger().Info().
		Int64("epoch", receivedEpoch.Int64()).
		Msg("received new shard state")

	node.nextShardState.master = epochShardState
	if node.Consensus.IsLeader() {
		// Wait a bit to allow the master table to reach other validators.
		node.nextShardState.proposeTime = time.Now().Add(5 * time.Second)
	} else {
		// Wait a bit to allow the master table to reach the leader,
		// and to allow the leader to propose next shard state based upon it.
		node.nextShardState.proposeTime = time.Now().Add(15 * time.Second)
	}
	// TODO ek – this should be done from replaying beaconchain once
	//  beaconchain sync is fixed
	err = node.Beaconchain().WriteShardState(
		receivedEpoch, epochShardState.ShardState)
	if err != nil {
		return ctxerror.New("cannot store shard state", "epoch", receivedEpoch).
			WithCause(err)
	}
	return nil
}

/*
func (node *Node) transitionIntoNextEpoch(shardState types.State) {
	logger = logger.New(
		"blsPubKey", hex.EncodeToString(node.Consensus.PubKey.Serialize()),
		"curShard", node.Blockchain().ShardID(),
		"curLeader", node.Consensus.IsLeader())
	for _, c := range shardState {
		utils.Logger().Debug().
			Uint32("shardID", c.ShardID).
			Str("nodeList", c.NodeList).
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
	for idx, nodeID := range myShardState.NodeList {
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
	for idx, shard := range state {
		for nodeIdx, nodeID := range shard.NodeList {
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
