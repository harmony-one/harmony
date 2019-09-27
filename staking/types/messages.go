package types

import (
	"math/big"

	"github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/internal/common"
)

// StakingMessage must fulfill these interfaces
type StakingMessage interface {
	// Type returns a human-readable string for the type of the staking message
	Type() string

	// Signer returns the ECDSA address who must sign the outer transaction
	Signer() common.Address
}

// MsgCreateValidator - struct for creating a new validator
type MsgCreateValidator struct {
	Description       Description     `json:"description" yaml:"description"`
	Commission        CommissionRates `json:"commission" yaml:"commission"`
	MinSelfDelegation big.Int         `json:"min_self_delegation" yaml:"min_self_delegation"`
	Address           common.Address  `json:"validator_address" yaml:"validator_address"`
	ValidatingPubKey  bls.PublicKey   `json:"validating_pub_key" yaml:"validating_pub_key"`
	Amount            big.Int         `json:"amount" yaml:"amount"`
}

// Type ...
func (msg MsgCreateValidator) Type() string { return "create_validator" }

// Signer ...
func (msg MsgCreateValidator) Signer() common.Address { return msg.Address }
