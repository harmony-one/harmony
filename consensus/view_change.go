package consensus

import (
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/bls/ffi/go/bls"
	bls_cosi "github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/internal/utils"
)

// PbftPhase  PBFT phases: pre-prepare, prepare and commit
type PbftPhase int

// Enum for PbftPhase
const (
	Announce PbftPhase = iota
	Prepare
	Commit
)

// Mode determines whether a node is in normal or viewchanging mode
type Mode int

// Enum for node Mode
const (
	Normal Mode = iota
	ViewChanging
)

// PbftMode contains mode and consensusID of viewchanging
type PbftMode struct {
	mode        Mode
	consensusID uint32
	mux         sync.Mutex
}

// Mode return the current node mode
func (pm *PbftMode) Mode() Mode {
	return pm.mode
}

// SetMode set the node mode as required
func (pm *PbftMode) SetMode(m Mode) {
	pm.mux.Lock()
	defer pm.mux.Unlock()
	pm.mode = m
}

// ConsensusID return the current viewchanging id
func (pm *PbftMode) ConsensusID() uint32 {
	return pm.consensusID
}

// SetConsensusID sets the viewchanging id accordingly
func (pm *PbftMode) SetConsensusID(consensusID uint32) {
	pm.mux.Lock()
	defer pm.mux.Unlock()
	pm.consensusID = consensusID
}

// startViewChange start a new view change
func (consensus *Consensus) startViewChange(consensusID uint32) {
	consensus.mode.SetMode(ViewChanging)
	consensus.mode.SetConsensusID(consensusID)
}

// switchPhase will switch PbftPhase to nextPhase if the desirePhase equals the nextPhase
func (consensus *Consensus) switchPhase(desirePhase PbftPhase) {
	utils.GetLogInstance().Debug("switchPhase: ", "desirePhase", desirePhase, "myPhase", consensus.phase)

	var nextPhase PbftPhase
	switch consensus.phase {
	case Announce:
		nextPhase = Prepare
	case Prepare:
		nextPhase = Commit
	case Commit:
		nextPhase = Announce
	}
	if nextPhase == desirePhase {
		consensus.phase = nextPhase
	}
}

// GetNextLeaderKey uniquely determine who is the leader for given consensusID
func (consensus *Consensus) GetNextLeaderKey() *bls.PublicKey {
	idx := consensus.getIndexOfPubKey(consensus.LeaderPubKey)
	if idx == -1 {
		utils.GetLogInstance().Warn("GetNextLeaderKey: currentLeaderKey not found", "key", consensus.LeaderPubKey.GetHexString())
	}
	idx = (idx + 1) % len(consensus.PublicKeys)
	return consensus.PublicKeys[idx]
}

func (consensus *Consensus) getIndexOfPubKey(pubKey *bls.PublicKey) int {
	for i := 0; i < len(consensus.PublicKeys); i++ {
		if consensus.PublicKeys[i].IsEqual(pubKey) {
			return i
		}
	}
	return -1 // not found
}

func (consensus *Consensus) ResetViewChangeState() {
	bhpBitmap, _ := bls_cosi.NewMask(consensus.PublicKeys, consensus.leader.ConsensusPubKey)
	nilBitmap, _ := bls_cosi.NewMask(consensus.PublicKeys, consensus.leader.ConsensusPubKey)
	consensus.bhpBitmap = bhpBitmap
	consensus.nilBitmap = nilBitmap

	consensus.bhpSigs = map[common.Address]*bls.Sign{}
	consensus.nilSigs = map[common.Address]*bls.Sign{}
	consensus.aggregatedBHPSig = nil
	consensus.aggregatedNILSig = nil
	consensus.vcBlockHash = [32]byte{}
}
