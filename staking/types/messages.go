package types

import (
	"fmt"
	"math/big"

	"github.com/harmony-one/harmony/shard"

	"github.com/ethereum/go-ethereum/common"

	"github.com/harmony-one/harmony/numeric"
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
	// DirectiveRedelegate ...
	DirectiveRedelegate
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
		DirectiveRedelegate:      "Redelegate",
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
	Description        `json:"description" yaml:"description"`
	CommissionRates    `json:"commission" yaml:"commission"`
	MinSelfDelegation  *big.Int             `json:"min_self_delegation" yaml:"min_self_delegation"`
	MaxTotalDelegation *big.Int             `json:"max_total_delegation" yaml:"max_total_delegation"`
	ValidatorAddress   common.Address       `json:"validator_address" yaml:"validator_address"`
	SlotPubKeys        []shard.BlsPublicKey `json:"slot_pub_keys" yaml:"slot_pub_keys"`
	Amount             *big.Int             `json:"amount" yaml:"amount"`
}

// EditValidator - type for edit existing validator
type EditValidator struct {
	ValidatorAddress   common.Address      `json:"validator_address" yaml:"validator_address"`
	Description        *Description        `json:"description" yaml:"description"`
	CommissionRate     *numeric.Dec        `json:"commission_rate" yaml:"commission_rate"`
	MinSelfDelegation  *big.Int            `json:"min_self_delegation" yaml:"min_self_delegation"`
	MaxTotalDelegation *big.Int            `json:"max_total_delegation" yaml:"max_total_delegation"`
	SlotKeyToRemove    *shard.BlsPublicKey `json:"slot_key_to_remove" yaml:"slot_key_to_remove"`
	SlotKeyToAdd       *shard.BlsPublicKey `json:"slot_key_to_add" yaml:"slot_key_to_add"`
}

// Delegate - type for delegating to a validator
type Delegate struct {
	DelegatorAddress common.Address `json:"delegator_address" yaml:"delegator_address"`
	ValidatorAddress common.Address `json:"validator_address" yaml:"validator_address"`
	Amount           *big.Int       `json:"amount" yaml:"amount"`
}

// Undelegate - type for removing delegation responsibility
type Undelegate struct {
	DelegatorAddress common.Address `json:"delegator_address" yaml:"delegator_address"`
	ValidatorAddress common.Address `json:"validator_address" yaml:"validator_address"`
	Amount           *big.Int       `json:"amount" yaml:"amount"`
}

// CollectRewards - type for collecting token rewards
type CollectRewards struct {
	DelegatorAddress common.Address `json:"delegator_address" yaml:"delegator_address"`
}
