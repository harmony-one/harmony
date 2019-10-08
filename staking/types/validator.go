package types

import (
	"math/big"

	"github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/internal/common"
)

// Define validator staking related const
const (
	MaxNameLength            = 70
	MaxIdentityLength        = 3000
	MaxWebsiteLength         = 140
	MaxSecurityContactLength = 140
	MaxDetailsLength         = 280
)

// Validator - data fields for a validator
type Validator struct {
	// ECDSA address of the validator
	Address common.Address `json:"address" yaml:"address"`
	// The BLS public key of the validator for consensus
	ValidatingPubKey bls.PublicKey `json:"validating_pub_key" yaml:"validating_pub_key"`
	// The stake put by the validator itself
	Stake *big.Int `json:"stake" yaml:"stake"`
	// if unbonding, height at which this validator has begun unbonding
	UnbondingHeight *big.Int `json:"unbonding_height" yaml:"unbonding_height"`
	// validator's self declared minimum self delegation
	MinSelfDelegation *big.Int `json:"min_self_delegation" yaml:"min_self_delegation"`
	// commission parameters
	Commission `json:"commission" yaml:"commission"`
	// description for the validator
	Description `json:"description" yaml:"description"`
	// Is the validator active in the validating process or not
	IsCurrentlyActive bool `json:"active" yaml:"active"`
}

// Description - some possible IRL connections
type Description struct {
	Name            string `json:"name" yaml:"name"`                         // name
	Identity        string `json:"identity" yaml:"identity"`                 // optional identity signature (ex. UPort or Keybase)
	Website         string `json:"website" yaml:"website"`                   // optional website link
	SecurityContact string `json:"security_contact" yaml:"security_contact"` // optional security contact info
	Details         string `json:"details" yaml:"details"`                   // optional details
}
