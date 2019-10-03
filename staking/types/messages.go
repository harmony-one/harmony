package types

import (
	"math/big"

	"github.com/harmony-one/harmony/internal/common"
	"github.com/harmony-one/harmony/shard"
	"github.com/pkg/errors"
)

// Directive says what kind of payload follows
type Directive byte

const (
	// DirectiveNewValidator ...
	DirectiveNewValidator Directive = iota
	// DirectiveEditValidator ...
	DirectiveEditValidator
	// DirectiveDelegate ...
	DirectiveDelegate
	// DirectiveRedelegate ...
	DirectiveRedelegate
	// DirectiveUndelegate ...
	DirectiveUndelegate
)

var (
	directiveKind = [...]string{
		"NewValidator", "EditValidator", "Delegate", "Redelegate", "Undelegate",
	}
	// ErrInvalidStakingKind given when caller gives bad staking message kind
	ErrInvalidStakingKind = errors.New("bad staking kind")
)

func (d Directive) String() string {
	return directiveKind[d]
}

// NewValidator - type for creating a new validator
type NewValidator struct {
	Description       `json:"ties" yaml:"ties"`
	CommissionRates   `json:"commission" yaml:"commission"`
	MinSelfDelegation *big.Int           `json:"min_self_delegation" yaml:"min_self_delegation"`
	StakingAddress    common.Address     `json:"staking_address" yaml:"staking_address"`
	PubKey            shard.BlsPublicKey `json:"validating_pub_key" yaml:"validating_pub_key"`
	Amount            *big.Int           `json:"amount" yaml:"amount"`
}

// EditValidator - type for edit existing validator
type EditValidator struct {
	Description
	StakingAddress    common.Address `json:"staking_address" yaml:"staking_address"`
	CommissionRate    Dec            `json:"commission_rate" yaml:"commission_rate"`
	MinSelfDelegation *big.Int       `json:"min_self_delegation" yaml:"min_self_delegation"`
}

// Delegate - type for delegating to a validator
type Delegate struct {
	DelegatorAddress common.Address `json:"delegator_address" yaml:"delegator_address"`
	ValidatorAddress common.Address `json:"validator_address" yaml:"validator_address"`
	Amount           *big.Int       `json:"amount" yaml:"amount"`
}

// Redelegate - type for reassigning delegation
type Redelegate struct {
	DelegatorAddress    common.Address `json:"delegator_address" yaml:"delegator_address"`
	ValidatorSrcAddress common.Address `json:"validator_src_address" yaml:"validator_src_address"`
	ValidatorDstAddress common.Address `json:"validator_dst_address" yaml:"validator_dst_address"`
	Amount              *big.Int       `json:"amount" yaml:"amount"`
}

// Undelegate - type for removing delegation responsibility
type Undelegate struct {
	DelegatorAddress common.Address `json:"delegator_address" yaml:"delegator_address"`
	ValidatorAddress common.Address `json:"validator_address" yaml:"validator_address"`
	Amount           *big.Int       `json:"amount" yaml:"amount"`
}
