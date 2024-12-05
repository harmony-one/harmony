package consensus

import (
	"sync"

	types "github.com/harmony-one/harmony/common/types"
)

type ProposalCreationStatus int

const (
	// Ready indicates the consensus is prepared to create a new proposal.
	// No ongoing proposal or dependencies are blocking the proposal process.
	Ready ProposalCreationStatus = iota

	// WaitingForCommitSigs signifies the consensus is currently waiting for commit signatures
	// from the previous block or process. This state can persist for an extended duration,
	// typically up to 8 seconds, depending on proposal type and processing time.
	WaitingForCommitSigs

	// CreatingNewProposal indicates the consensus is already engaged in creating a new proposal.
	// During this state, no additional proposals can be initiated until the current one completes.
	CreatingNewProposal
)

func (pt ProposalCreationStatus) String() string {
	switch pt {
	case Ready:
		return "Ready"
	case WaitingForCommitSigs:
		return "WaitingForCommitSigs"
	case CreatingNewProposal:
		return "CreatingNewProposal"
	default:
		return "Unknown"
	}
}

type ProposalManager struct {
	history     *types.SafeMap[ProposalType, *Proposal]
	lasProposal *Proposal
	status      ProposalCreationStatus
	lock        *sync.RWMutex
}

// NewProposalManager initializes a new ProposalManager.
func NewProposalManager() *ProposalManager {
	return &ProposalManager{
		history:     types.NewSafeMap[ProposalType, *Proposal](),
		lasProposal: nil,
		status:      Ready,
		lock:        &sync.RWMutex{},
	}
}

// SetlasProposal updates the last processed proposal height.
func (pm *ProposalManager) SetlasProposal(p *Proposal) {
	if p == nil {
		return
	}
	pm.lock.Lock()
	defer pm.lock.Unlock()
	if pm.lasProposal == nil || p.Height > pm.lasProposal.Height {
		pm.lasProposal = p
	}
}

// GetlasProposal retrieves the last processed proposal height.
func (pm *ProposalManager) GetlasProposal() *Proposal {
	pm.lock.RLock()
	defer pm.lock.RUnlock()
	return pm.lasProposal
}

// SetStatus sets new proposal creation status.
func (pm *ProposalManager) SetStatus(newStatus ProposalCreationStatus) {
	pm.lock.Lock()
	defer pm.lock.Unlock()
	pm.status = newStatus
}

// StartWaitingForCommitSigs sets isWaitingForCommitSigs.
func (pm *ProposalManager) SetToWaitingForCommitSigsMode() {
	pm.lock.Lock()
	defer pm.lock.Unlock()
	pm.status = WaitingForCommitSigs
}

// IsWaitingForCommitSigs returns true if current proposal is waiting for commit sigs.
func (pm *ProposalManager) IsWaitingForCommitSigs() bool {
	pm.lock.RLock()
	defer pm.lock.RUnlock()
	return pm.status == WaitingForCommitSigs
}

// StartWaitingForCommitSigs sets isWaitingForCommitSigs.
func (pm *ProposalManager) SetToCreatingNewProposalMode() {
	pm.lock.Lock()
	defer pm.lock.Unlock()
	pm.status = CreatingNewProposal
}

// IsCreatingNewProposal returns true if consensus is busy with proposal creation.
func (pm *ProposalManager) IsCreatingNewProposal() bool {
	pm.lock.RLock()
	defer pm.lock.RUnlock()
	return pm.status == CreatingNewProposal
}

// Done sets status to ready.
func (pm *ProposalManager) Done() {
	pm.lock.Lock()
	defer pm.lock.Unlock()
	pm.status = Ready
}

// IsWaitingForCommitSigs returns true if current proposal is waiting for commit sigs.
func (pm *ProposalManager) IsReady() bool {
	pm.lock.RLock()
	defer pm.lock.RUnlock()
	return pm.status == Ready
}

// AddProposal adds a new proposal if valid or updates an existing one if better.
func (pm *ProposalManager) AddProposal(p *Proposal) bool {
	pm.lock.Lock()
	defer pm.lock.Unlock()
	existingProposal, exists := pm.history.Get(p.Type)
	if exists {
		if p.Height > existingProposal.Height || (p.Height == existingProposal.Height && p.ViewID > existingProposal.ViewID) {
			pm.history.Set(p.Type, p)
			return true
		}
		return false
	}
	pm.history.Set(p.Type, p)
	return true
}

// GetNextProposal retrieves and removes the next proposal based on priority.
func (pm *ProposalManager) GetNextProposal() (*Proposal, error) {
	pm.lock.Lock()
	defer pm.lock.Unlock()

	syncProposal, syncExist := pm.history.Get(SyncProposal)
	asyncProposal, asyncExist := pm.history.Get(AsyncProposal)

	var nextProposal *Proposal
	if syncExist {
		nextProposal = syncProposal.Clone()
		pm.history.Delete(SyncProposal)
	} else if asyncExist {
		nextProposal = asyncProposal.Clone()
		pm.history.Delete(AsyncProposal)
	}

	if nextProposal == nil {
		// no proposals available
		return nil, nil
	}

	pm.lasProposal = nextProposal
	return nextProposal, nil
}

// ClearHistory clears all proposals from the history.
func (pm *ProposalManager) ClearHistory() {
	pm.lock.Lock()
	defer pm.lock.Unlock()
	pm.history.Clear()
}

// Length returns the number of proposals in the history.
func (pm *ProposalManager) Length() int {
	pm.lock.RLock()
	defer pm.lock.RUnlock()
	return pm.history.Length()
}
