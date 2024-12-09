package consensus

import (
	"sync"
	"time"

	bls_cosi "github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/internal/utils"
)

// ProposalType is to indicate the type of signal for new block proposal
type ProposalType byte

// Constant of the type of new block proposal
const (
	SyncProposal ProposalType = iota
	AsyncProposal
)

func (pt ProposalType) String() string {
	if pt == SyncProposal {
		return "SyncProposal"
	}
	return "AsyncProposal"
}

// Proposal represents a new block proposal with associated metadata
type Proposal struct {
	leaderPubKey *bls_cosi.PublicKeyWrapper
	Type         ProposalType
	Caller       string
	Height       uint64
	ViewID       uint64
	Source       string
	Reason       string
	CreatedAt    time.Time
	lock         *sync.RWMutex
}

// NewProposal creates a new proposal
func NewProposal(leaderPubKey *bls_cosi.PublicKeyWrapper, t ProposalType, viewID uint64, height uint64, source string, reason string) *Proposal {
	return &Proposal{
		leaderPubKey: leaderPubKey,
		Type:         t,
		Caller:       utils.GetCallStackInfo(2),
		ViewID:       0,
		Height:       0,
		Source:       source,
		Reason:       reason,
		CreatedAt:    time.Now(),
		lock:         &sync.RWMutex{},
	}
}

// Clone returns a copy of proposal
func (p *Proposal) Clone() *Proposal {
	p.lock.RLock()
	defer p.lock.RUnlock()
	return &Proposal{
		Type:      p.Type,
		Caller:    p.Caller,
		ViewID:    p.ViewID,
		Height:    p.Height,
		Source:    p.Source,
		Reason:    p.Reason,
		CreatedAt: p.CreatedAt,
		lock:      &sync.RWMutex{},
	}
}

// GetType retrieves the Proposal type
func (p *Proposal) GetType() ProposalType {
	p.lock.RLock()
	defer p.lock.RUnlock()
	return p.Type
}

// SetType updates the Proposal type
func (p *Proposal) SetType(t ProposalType) {
	p.lock.Lock()
	defer p.lock.Unlock()
	p.Type = t
}

// GetCaller retrieves the Proposal caller
func (p *Proposal) GetCaller() string {
	p.lock.RLock()
	defer p.lock.RUnlock()
	return p.Caller
}

// SetCaller updates the Proposal caller
func (p *Proposal) SetCaller(caller string) {
	p.lock.Lock()
	defer p.lock.Unlock()
	p.Caller = caller
}

// GetHeight retrieves the Proposal height
func (p *Proposal) GetHeight() uint64 {
	p.lock.RLock()
	defer p.lock.RUnlock()
	return p.Height
}

// SetHeight updates the Proposal height
func (p *Proposal) SetHeight(height uint64) {
	p.lock.Lock()
	defer p.lock.Unlock()
	p.Height = height
}

// GetViewID retrieves the Proposal view ID
func (p *Proposal) GetViewID() uint64 {
	p.lock.RLock()
	defer p.lock.RUnlock()
	return p.ViewID
}

// SetViewID updates the Proposal view ID
func (p *Proposal) SetViewID(viewID uint64) {
	p.lock.Lock()
	defer p.lock.Unlock()
	p.ViewID = viewID
}

// GetCreatedAt retrieves the Proposal creation time
func (p *Proposal) GetCreatedAt() time.Time {
	p.lock.RLock()
	defer p.lock.RUnlock()
	return p.CreatedAt
}

// SetCreatedAt updates the Proposal creation time
func (p *Proposal) SetCreatedAt(createdAt time.Time) {
	p.lock.Lock()
	defer p.lock.Unlock()
	p.CreatedAt = createdAt
}
