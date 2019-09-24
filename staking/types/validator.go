package types

import (
	"github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/internal/common"
	"math/big"
)

// Validator - data fields for a validator
type Validator struct {
	StakingAddress    common.Address `json:"staking_address" yaml:"staking_address"`         // ECDSA address of the validator
	ValidatingPubKey  bls.PublicKey  `json:"validating_pub_key" yaml:"validating_pub_key"`   // The BLS public key of the validator for consensus
	Description       Description    `json:"description" yaml:"description"`                 // description for the validator
	Active            bool           `json:"active" yaml:"active"`                           // Is the validator active in the validating process or not
	Stake             big.Int        `json:"stake" yaml:"stake"`                             // The stake put by the validator itself
	UnbondingHeight   big.Int        `json:"unbonding_height" yaml:"unbonding_height"`       // if unbonding, height at which this validator has begun unbonding
	Commission        Commission     `json:"commission" yaml:"commission"`                   // commission parameters
	MinSelfDelegation big.Int        `json:"min_self_delegation" yaml:"min_self_delegation"` // validator's self declared minimum self delegation
}

// Description - description fields for a validator
type Description struct {
	Name            string `json:"name" yaml:"name"`                         // name
	Identity        string `json:"identity" yaml:"identity"`                 // optional identity signature (ex. UPort or Keybase)
	Website         string `json:"website" yaml:"website"`                   // optional website link
	SecurityContact string `json:"security_contact" yaml:"security_contact"` // optional security contact info
	Details         string `json:"details" yaml:"details"`                   // optional details
}
