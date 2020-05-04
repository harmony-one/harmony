package types

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/numeric"
	"github.com/harmony-one/harmony/shard"
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
)

var (
	directiveNames = map[Directive]string{
		DirectiveCreateValidator: "CreateValidator",
		DirectiveEditValidator:   "EditValidator",
		DirectiveDelegate:        "Delegate",
		DirectiveUndelegate:      "Undelegate",
		DirectiveCollectRewards:  "CollectRewards",
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
	MinSelfDelegation  *big.Int             `json:"min-self-delegation"`
	MaxTotalDelegation *big.Int             `json:"max-total-delegation"`
	SlotPubKeys        []shard.BLSPublicKey `json:"slot-pub-keys"`
	SlotKeySigs        []shard.BLSSignature `json:"slot-key-sigs"`
	Amount             *big.Int             `json:"amount"`
}

// Type of CreateValidator
func (v CreateValidator) Type() Directive {
	return DirectiveCreateValidator
}

// Copy deep copy of the interface
func (v CreateValidator) Copy() StakeMsg {
	cp := CreateValidator{
		ValidatorAddress: v.ValidatorAddress,
		Description:      v.Description,
		CommissionRates:  v.CommissionRates.Copy(),
		SlotPubKeys:      make([]shard.BLSPublicKey, len(v.SlotPubKeys)),
		SlotKeySigs:      make([]shard.BLSSignature, len(v.SlotKeySigs)),
	}
	copy(cp.SlotPubKeys, v.SlotPubKeys)
	copy(cp.SlotKeySigs, v.SlotKeySigs)

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
	CommissionRate     *numeric.Dec          `json:"commission-rate" rlp:"nil"`
	MinSelfDelegation  *big.Int              `json:"min-self-delegation" rlp:"nil"`
	MaxTotalDelegation *big.Int              `json:"max-total-delegation" rlp:"nil"`
	SlotKeyToRemove    *shard.BLSPublicKey   `json:"slot-key-to_remove" rlp:"nil"`
	SlotKeyToAdd       *shard.BLSPublicKey   `json:"slot-key-to_add" rlp:"nil"`
	SlotKeyToAddSig    *shard.BLSSignature   `json:"slot-key-to-add-sig" rlp:"nil"`
	EPOSStatus         effective.Eligibility `json:"epos-eligibility-status" rlp:"nil"`
}

// Type of EditValidator
func (v EditValidator) Type() Directive {
	return DirectiveEditValidator
}

// Copy deep copy of the interface
func (v EditValidator) Copy() StakeMsg {
	v1 := v
	v1.Description = v.Description
	return v1
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

// Copy deep copy of the interface
func (v Delegate) Copy() StakeMsg {
	v1 := v
	return v1
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

// Copy deep copy of the interface
func (v Undelegate) Copy() StakeMsg {
	v1 := v
	return v1
}

// CollectRewards - type for collecting token rewards
type CollectRewards struct {
	DelegatorAddress common.Address `json:"delegator_address"`
}

// Type of CollectRewards
func (v CollectRewards) Type() Directive {
	return DirectiveCollectRewards
}

// Copy deep copy of the interface
func (v CollectRewards) Copy() StakeMsg {
	v1 := v
	return v1
}
