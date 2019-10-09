package values

// QuorumPhase is a phase that needs quorum to proceed
type QuorumPhase byte

const (
	// QuorumPrepare ..
	QuorumPrepare QuorumPhase = iota
	// QuorumCommit ..
	QuorumCommit
)

// ConsensusMechanism is the way we decide consensus
type ConsensusMechanism byte

const (
	// SuperMajorityVote is a 2/3s voting mechanism, pre-PoS
	SuperMajorityVote ConsensusMechanism = iota
	// SuperMajorityStake is 2/3s of total staked amount for epoch
	SuperMajorityStake
)

// PBFTPhase : different phases of consensus
type PBFTPhase byte

// Enum for PbftPhase
const (
	PBFTAnnounce PBFTPhase = iota
	PBFTPrepare
	PBFTCommit
)

// PBFTState determines whether a node is in normal or viewchanging mode
type PBFTState byte

// Enum for node Mode
const (
	PBFTNormal PBFTState = iota
	PBFTViewChanging
	PBFTSyncing
	PBFTListening
)

var (
	modes  = [...]string{"Normal", "ViewChanging", "Syncing", "Listening"}
	phases = [...]string{"Announce", "Prepare", "Commit"}
)

func (s PBFTState) String() string {
	return modes[s]
}

// String print phase string
func (p PBFTPhase) String() string {
	return phases[p]
}
