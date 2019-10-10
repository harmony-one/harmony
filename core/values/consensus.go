package values

// QuorumPhase is a phase that needs quorum to proceed
type QuorumPhase byte

const (
	// QuorumPrepare ..
	QuorumPrepare QuorumPhase = iota
	// QuorumCommit ..
	QuorumCommit
	// QuorumViewChange ..
	QuorumViewChange
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
	stateNames  = map[PBFTState]string{
		PBFTNormal:       "Normal",
		PBFTViewChanging: "ViewChanging",
		PBFTSyncing:      "Syncing",
		PBFTListening:    "Listening",
	}
	phases = [...]string{"Announce", "Prepare", "Commit"}
)

func (s PBFTState) String() string {
	if name, ok := stateNames[s]; ok {
		return name
	}
	return fmt.Sprintf("PBFTState %v", byte(s))
}

// String print phase string
func (p PBFTPhase) String() string {
	return phases[p]
}
