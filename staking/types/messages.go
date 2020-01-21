package types

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"

	"github.com/harmony-one/harmony/numeric"
	"github.com/harmony-one/harmony/shard"
	"github.com/pkg/errors"
)

// Directive says what kind of payload follows
type Directive byte

// StakeMsg defines the interface of Stake Message
type StakeMsg interface {
	Type() Directive
	Copy() StakeMsg
}

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

// CreateValidator - type for creating a new validator
type CreateValidator struct {
	ValidatorAddress   common.Address `json:"validator_address" yaml:"validator_address"`
	Description        *Description   `json:"description" yaml:"description"`
	CommissionRates    `json:"commission" yaml:"commission"`
	MinSelfDelegation  *big.Int             `json:"min_self_delegation" yaml:"min_self_delegation"`
	MaxTotalDelegation *big.Int             `json:"max_total_delegation" yaml:"max_total_delegation"`
	SlotPubKeys        []shard.BlsPublicKey `json:"slot_pub_keys" yaml:"slot_pub_keys"`
	SlotKeySigs        []shard.BlsSignature `json:"slot_key_sigs" yaml:"slot_key_sigs"`
	Amount             *big.Int             `json:"amount" yaml:"amount"`
}

// EditValidator - type for edit existing validator
type EditValidator struct {
	ValidatorAddress   common.Address      `json:"validator_address" yaml:"validator_address" rlp:"nil"`
	Description        *Description        `json:"description" yaml:"description" rlp:"nil"`
	CommissionRate     *numeric.Dec        `json:"commission_rate" yaml:"commission_rate" rlp:"nil"`
	MinSelfDelegation  *big.Int            `json:"min_self_delegation" yaml:"min_self_delegation" rlp:"nil"`
	MaxTotalDelegation *big.Int            `json:"max_total_delegation" yaml:"max_total_delegation" rlp:"nil"`
	SlotKeyToRemove    *shard.BlsPublicKey `json:"slot_key_to_remove" yaml:"slot_key_to_remove" rlp:"nil"`
	SlotKeyToAdd       *shard.BlsPublicKey `json:"slot_key_to_add" yaml:"slot_key_to_add" rlp:"nil"`
	SlotKeyToAddSig    shard.BlsSignature  `json:"slot_key_to_add_sig" yaml:"slot_key_to_add_sig" rlp:"nil"`
	Active             *bool               `json:"active" rlp:"nil"`
}

// Delegate - type for delegating to a validator
type Delegate struct {
	DelegatorAddress common.Address `json:"delegator_address" yaml:"delegator_address"`
	ValidatorAddress common.Address `json:"validator_address" yaml:"validator_address"`
	Amount           *big.Int       `json:"amount" yaml:"amount" rlp:"nil"`
}

// Undelegate - type for removing delegation responsibility
type Undelegate struct {
	DelegatorAddress common.Address `json:"delegator_address" yaml:"delegator_address" rlp:"nil"`
	ValidatorAddress common.Address `json:"validator_address" yaml:"validator_address" rlp:"nil"`
	Amount           *big.Int       `json:"amount" yaml:"amount" rlp:"nil"`
}

// CollectRewards - type for collecting token rewards
type CollectRewards struct {
	DelegatorAddress common.Address `json:"delegator_address" yaml:"delegator_address"`
}

// Type of CreateValidator
func (v CreateValidator) Type() Directive {
	return DirectiveCreateValidator
}

// Type of EditValidator
func (v EditValidator) Type() Directive {
	return DirectiveEditValidator
}

// Type of Delegate
func (v Delegate) Type() Directive {
	return DirectiveDelegate
}

// Type of Undelegate
func (v Undelegate) Type() Directive {
	return DirectiveUndelegate
}

// Type of CollectRewards
func (v CollectRewards) Type() Directive {
	return DirectiveCollectRewards
}

// Copy deep copy of the interface
func (v CreateValidator) Copy() StakeMsg {
	v1 := v
	desc := *v.Description
	v1.Description = &desc
	return v1
}

// Copy deep copy of the interface
func (v EditValidator) Copy() StakeMsg {
	v1 := v
	desc := *v.Description
	v1.Description = &desc
	return v1
}

// Copy deep copy of the interface
func (v Delegate) Copy() StakeMsg {
	v1 := v
	return v1
}

// Copy deep copy of the interface
func (v Undelegate) Copy() StakeMsg {
	v1 := v
	return v1
}

// Copy deep copy of the interface
func (v CollectRewards) Copy() StakeMsg {
	v1 := v
	return v1
}
