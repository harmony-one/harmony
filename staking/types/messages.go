package types

import (
	"bytes"
	"fmt"
	"math/big"

	"github.com/harmony-one/harmony/crypto/bls"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/numeric"
	"github.com/harmony-one/harmony/staking/effective"
	"github.com/pkg/errors"
)

// Directive says what kind of payload follows
type Directive byte

const (
	// DirectiveCreateValidator ...
	DirectiveCreateValidator Directive = iota
	// DirectiveEditValidator ...
	DirectiveEditValidator
	// DirectiveDelegate ...
	DirectiveDelegate
	// DirectiveUndelegate ...
	DirectiveUndelegate
	// DirectiveCollectRewards ...
	DirectiveCollectRewards
	// DirectiveBatchDelegate ...
	DirectiveBatchDelegate
	// DirectiveBatchUndelegate ...
	DirectiveBatchUndelegate
	// DirectiveUndelegateAll ...
	DirectiveUndelegateAll
)

var (
	directiveNames = map[Directive]string{
		DirectiveCreateValidator: "CreateValidator",
		DirectiveEditValidator:   "EditValidator",
		DirectiveDelegate:        "Delegate",
		DirectiveUndelegate:      "Undelegate",
		DirectiveCollectRewards:  "CollectRewards",
		DirectiveBatchDelegate:   "BatchDelegate",
		DirectiveBatchUndelegate: "BatchUndelegate",
		DirectiveUndelegateAll:   "UndelegateAll",
	}
	// ErrInvalidStakingKind given when caller gives bad staking message kind
	ErrInvalidStakingKind = errors.New("bad staking kind")
)

func (d Directive) String() string {
	if name, ok := directiveNames[d]; ok {
		return name
	}
	return fmt.Sprintf("Directive %+v", byte(d))
}

// StakeMsg defines the interface of Stake Message
type StakeMsg interface {
	Type() Directive
	Copy() StakeMsg
}

// CreateValidator - type for creating a new validator
type CreateValidator struct {
	ValidatorAddress   common.Address `json:"validator-address"`
	Description        `json:"description"`
	CommissionRates    `json:"commission"`
	MinSelfDelegation  *big.Int                  `json:"min-self-delegation"`
	MaxTotalDelegation *big.Int                  `json:"max-total-delegation"`
	SlotPubKeys        []bls.SerializedPublicKey `json:"slot-pub-keys"`
	SlotKeySigs        []bls.SerializedSignature `json:"slot-key-sigs"`
	Amount             *big.Int                  `json:"amount"`
}

// Type of CreateValidator
func (v CreateValidator) Type() Directive {
	return DirectiveCreateValidator
}

// Copy returns a deep copy of the CreateValidator as a StakeMsg interface
func (v CreateValidator) Copy() StakeMsg {
	cp := CreateValidator{
		ValidatorAddress: v.ValidatorAddress,
		Description:      v.Description,
		CommissionRates:  v.CommissionRates.Copy(),
	}

	if v.SlotPubKeys != nil {
		cp.SlotPubKeys = make([]bls.SerializedPublicKey, len(v.SlotPubKeys))
		copy(cp.SlotPubKeys, v.SlotPubKeys)
	}
	if v.SlotKeySigs != nil {
		cp.SlotKeySigs = make([]bls.SerializedSignature, len(v.SlotKeySigs))
		copy(cp.SlotKeySigs, v.SlotKeySigs)
	}
	if v.MinSelfDelegation != nil {
		cp.MinSelfDelegation = new(big.Int).Set(v.MinSelfDelegation)
	}
	if v.MaxTotalDelegation != nil {
		cp.MaxTotalDelegation = new(big.Int).Set(v.MaxTotalDelegation)
	}
	if v.Amount != nil {
		cp.Amount = new(big.Int).Set(v.Amount)
	}
	return cp
}

// EditValidator - type for edit existing validator
type EditValidator struct {
	ValidatorAddress   common.Address `json:"validator-address"`
	Description        `json:"description"`
	CommissionRate     *numeric.Dec             `json:"commission-rate" rlp:"nil"`
	MinSelfDelegation  *big.Int                 `json:"min-self-delegation" rlp:"nil"`
	MaxTotalDelegation *big.Int                 `json:"max-total-delegation" rlp:"nil"`
	SlotKeyToRemove    *bls.SerializedPublicKey `json:"slot-key-to_remove" rlp:"nil"`
	SlotKeyToAdd       *bls.SerializedPublicKey `json:"slot-key-to_add" rlp:"nil"`
	SlotKeyToAddSig    *bls.SerializedSignature `json:"slot-key-to-add-sig" rlp:"nil"`
	EPOSStatus         effective.Eligibility    `json:"epos-eligibility-status"`
}

// Type of EditValidator
func (v EditValidator) Type() Directive {
	return DirectiveEditValidator
}

// Copy returns a deep copy of the EditValidator as a StakeMsg interface
func (v EditValidator) Copy() StakeMsg {
	cp := EditValidator{
		ValidatorAddress: v.ValidatorAddress,
		Description:      v.Description,
		EPOSStatus:       v.EPOSStatus,
	}
	if v.CommissionRate != nil {
		cr := v.CommissionRate.Copy()
		cp.CommissionRate = &cr
	}
	if v.MinSelfDelegation != nil {
		cp.MinSelfDelegation = new(big.Int).Set(v.MinSelfDelegation)
	}
	if v.MaxTotalDelegation != nil {
		cp.MaxTotalDelegation = new(big.Int).Set(v.MaxTotalDelegation)
	}
	if v.SlotKeyToRemove != nil {
		keyRem := *v.SlotKeyToRemove
		cp.SlotKeyToRemove = &keyRem
	}
	if v.SlotKeyToAdd != nil {
		keyAdd := *v.SlotKeyToAdd
		cp.SlotKeyToAdd = &keyAdd
	}
	if v.SlotKeyToAddSig != nil {
		sigAdd := *v.SlotKeyToAddSig
		cp.SlotKeyToAddSig = &sigAdd
	}
	return cp
}

// Delegate - type for delegating to a validator
type Delegate struct {
	DelegatorAddress common.Address `json:"delegator_address"`
	ValidatorAddress common.Address `json:"validator_address"`
	Amount           *big.Int       `json:"amount"`
}

// Type of Delegate
func (v Delegate) Type() Directive {
	return DirectiveDelegate
}

// Copy returns a deep copy of the Delegate as a StakeMsg interface
func (v Delegate) Copy() StakeMsg {
	cp := Delegate{
		DelegatorAddress: v.DelegatorAddress,
		ValidatorAddress: v.ValidatorAddress,
	}
	if v.Amount != nil {
		cp.Amount = new(big.Int).Set(v.Amount)
	}
	return cp
}

// Equals returns if v and s are equal
func (v Delegate) Equals(s Delegate) bool {
	if !bytes.Equal(v.DelegatorAddress.Bytes(), s.DelegatorAddress.Bytes()) {
		return false
	}
	if !bytes.Equal(v.ValidatorAddress.Bytes(), s.ValidatorAddress.Bytes()) {
		return false
	}
	if v.Amount == nil {
		return s.Amount == nil
	}
	return s.Amount != nil && v.Amount.Cmp(s.Amount) == 0 // pointer
}

// Undelegate - type for removing delegation responsibility
type Undelegate struct {
	DelegatorAddress common.Address `json:"delegator_address"`
	ValidatorAddress common.Address `json:"validator_address"`
	Amount           *big.Int       `json:"amount"`
}

// Type of Undelegate
func (v Undelegate) Type() Directive {
	return DirectiveUndelegate
}

// Copy returns a deep copy of the Undelegate as a StakeMsg interface
func (v Undelegate) Copy() StakeMsg {
	cp := Undelegate{
		DelegatorAddress: v.DelegatorAddress,
		ValidatorAddress: v.ValidatorAddress,
	}
	if v.Amount != nil {
		cp.Amount = new(big.Int).Set(v.Amount)
	}
	return cp
}

// Equals returns if v and s are equal
func (v Undelegate) Equals(s Undelegate) bool {
	if !bytes.Equal(v.DelegatorAddress.Bytes(), s.DelegatorAddress.Bytes()) {
		return false
	}
	if !bytes.Equal(v.ValidatorAddress.Bytes(), s.ValidatorAddress.Bytes()) {
		return false
	}
	if v.Amount == nil {
		return s.Amount == nil
	}
	return s.Amount != nil && v.Amount.Cmp(s.Amount) == 0 // pointer
}

// CollectRewards - type for collecting token rewards
type CollectRewards struct {
	DelegatorAddress common.Address `json:"delegator_address"`
}

// Type of CollectRewards
func (v CollectRewards) Type() Directive {
	return DirectiveCollectRewards
}

// Copy returns a deep copy of the CollectRewards as a StakeMsg interface
func (v CollectRewards) Copy() StakeMsg {
	return CollectRewards{
		DelegatorAddress: v.DelegatorAddress,
	}
}

// Equals returns if v and s are equal
func (v CollectRewards) Equals(s CollectRewards) bool {
	return bytes.Equal(v.DelegatorAddress.Bytes(), s.DelegatorAddress.Bytes())
}

// Migration Msg - type for switching delegation from one user to next
type MigrationMsg struct {
	From common.Address `json:"from" rlp:"nil"`
	To   common.Address `json:"to" rlp:"nil"`
}

func (v MigrationMsg) Copy() MigrationMsg {
	return MigrationMsg{
		From: v.From,
		To:   v.To,
	}
}

func (v MigrationMsg) Equals(s MigrationMsg) bool {
	return v.From == s.From && v.To == s.To
}

// DelegationAction represents a single delegation action in a batch operation
type DelegationAction struct {
	ValidatorAddress common.Address `json:"validator_address"`
	Amount           *big.Int       `json:"amount"`
}

// BatchDelegate - type for delegating to multiple validators in one transaction
type BatchDelegate struct {
	DelegatorAddress common.Address     `json:"delegator_address"`
	Delegations      []DelegationAction `json:"delegations"`
}

// Type of BatchDelegate
func (v BatchDelegate) Type() Directive {
	return DirectiveBatchDelegate
}

// Copy returns a deep copy of the BatchDelegate as a StakeMsg interface
func (v BatchDelegate) Copy() StakeMsg {
	cp := BatchDelegate{
		DelegatorAddress: v.DelegatorAddress,
		Delegations:      make([]DelegationAction, len(v.Delegations)),
	}
	for i, d := range v.Delegations {
		cp.Delegations[i] = DelegationAction{
			ValidatorAddress: d.ValidatorAddress,
		}
		if d.Amount != nil {
			cp.Delegations[i].Amount = new(big.Int).Set(d.Amount)
		}
	}
	return cp
}

// Equals returns if v and s are equal
func (v BatchDelegate) Equals(s BatchDelegate) bool {
	if !bytes.Equal(v.DelegatorAddress.Bytes(), s.DelegatorAddress.Bytes()) {
		return false
	}
	if len(v.Delegations) != len(s.Delegations) {
		return false
	}
	for i := range v.Delegations {
		if !bytes.Equal(v.Delegations[i].ValidatorAddress.Bytes(), s.Delegations[i].ValidatorAddress.Bytes()) {
			return false
		}
		if v.Delegations[i].Amount == nil {
			if s.Delegations[i].Amount != nil {
				return false
			}
		} else if s.Delegations[i].Amount == nil {
			return false
		} else if v.Delegations[i].Amount.Cmp(s.Delegations[i].Amount) != 0 {
			return false
		}
	}
	return true
}

// MaxBatchStakingActions is the maximum number of actions allowed in a single
// BatchDelegate or BatchUndelegate transaction.
const MaxBatchStakingActions = 50

// UndelegationAction represents a single undelegation action in a batch operation
type UndelegationAction struct {
	ValidatorAddress common.Address `json:"validator_address"`
	Amount           *big.Int       `json:"amount"`
}

// BatchUndelegate - type for undelegating from multiple validators in one transaction
type BatchUndelegate struct {
	DelegatorAddress common.Address       `json:"delegator_address"`
	Undelegations    []UndelegationAction `json:"undelegations"`
}

// Type of BatchUndelegate
func (v BatchUndelegate) Type() Directive {
	return DirectiveBatchUndelegate
}

// Copy returns a deep copy of the BatchUndelegate as a StakeMsg interface
func (v BatchUndelegate) Copy() StakeMsg {
	cp := BatchUndelegate{
		DelegatorAddress: v.DelegatorAddress,
		Undelegations:    make([]UndelegationAction, len(v.Undelegations)),
	}
	for i, u := range v.Undelegations {
		cp.Undelegations[i] = UndelegationAction{
			ValidatorAddress: u.ValidatorAddress,
		}
		if u.Amount != nil {
			cp.Undelegations[i].Amount = new(big.Int).Set(u.Amount)
		}
	}
	return cp
}

// Equals returns if v and s are equal
func (v BatchUndelegate) Equals(s BatchUndelegate) bool {
	if !bytes.Equal(v.DelegatorAddress.Bytes(), s.DelegatorAddress.Bytes()) {
		return false
	}
	if len(v.Undelegations) != len(s.Undelegations) {
		return false
	}
	for i := range v.Undelegations {
		if !bytes.Equal(v.Undelegations[i].ValidatorAddress.Bytes(), s.Undelegations[i].ValidatorAddress.Bytes()) {
			return false
		}
		if v.Undelegations[i].Amount == nil {
			if s.Undelegations[i].Amount != nil {
				return false
			}
		} else if s.Undelegations[i].Amount == nil {
			return false
		} else if v.Undelegations[i].Amount.Cmp(s.Undelegations[i].Amount) != 0 {
			return false
		}
	}
	return true
}

// UndelegateAll - type for undelegating all from all validators
type UndelegateAll struct {
	DelegatorAddress common.Address `json:"delegator_address"`
}

// Type of UndelegateAll
func (v UndelegateAll) Type() Directive {
	return DirectiveUndelegateAll
}

// Copy returns a deep copy of the UndelegateAll as a StakeMsg interface
func (v UndelegateAll) Copy() StakeMsg {
	return UndelegateAll{
		DelegatorAddress: v.DelegatorAddress,
	}
}

// Equals returns if v and s are equal
func (v UndelegateAll) Equals(s UndelegateAll) bool {
	return bytes.Equal(v.DelegatorAddress.Bytes(), s.DelegatorAddress.Bytes())
}
