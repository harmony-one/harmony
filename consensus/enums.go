package consensus

import "fmt"

// Mode is the current
type Mode byte

const (
	// Normal ..
	Normal Mode = iota
	// ViewChanging ..
	ViewChanging
	// Syncing ..
	Syncing
	// Listening ..
	Listening
	// NormalBackup Backup Node ..
	NormalBackup
)

// FBFTPhase : different phases of consensus
type FBFTPhase byte

// Enum for FBFTPhase
const (
	FBFTAnnounce FBFTPhase = iota
	FBFTPrepare
	FBFTCommit
)

var (
	modeNames = map[Mode]string{
		Normal:       "Normal",
		ViewChanging: "ViewChanging",
		Syncing:      "Syncing",
		Listening:    "Listening",
		NormalBackup: "NormalBackup",
	}
	phaseNames = map[FBFTPhase]string{
		FBFTAnnounce: "Announce",
		FBFTPrepare:  "Prepare",
		FBFTCommit:   "Commit",
	}
)

func (m Mode) String() string {
	if name, ok := modeNames[m]; ok {
		return name
	}
	return fmt.Sprintf("Mode %+v", byte(m))
}

func (p FBFTPhase) String() string {
	if name, ok := phaseNames[p]; ok {
		return name
	}
	return fmt.Sprintf("FBFTPhase %+v", byte(p))
}
