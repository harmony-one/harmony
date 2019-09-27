package types

import (
	"bytes"
	"encoding/json"
	"math/big"

	"github.com/harmony-one/harmony/shard"

	"github.com/ethereum/go-ethereum/common"
	"github.com/pkg/errors"
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
	Description       Description        `json:"description" yaml:"description"`
	Commission        CommissionRates    `json:"commission" yaml:"commission"`
	MinSelfDelegation *big.Int           `json:"min_self_delegation" yaml:"min_self_delegation"`
	StakingAddress    common.Address     `json:"staking_address" yaml:"staking_address"`
	ValidatingPubKey  shard.BlsPublicKey `json:"validating_pub_key" yaml:"validating_pub_key"`
	Amount            *big.Int           `json:"amount" yaml:"amount"`
}

// msgCreateValidatorJSON - struct for creating a new validator for JSON
type msgCreateValidatorJSON struct {
	Description       Description        `json:"description" yaml:"description"`
	Commission        CommissionRates    `json:"commission" yaml:"commission"`
	MinSelfDelegation *big.Int           `json:"min_self_delegation" yaml:"min_self_delegation"`
	StakingAddress    common.Address     `json:"staking_address" yaml:"staking_address"`
	ValidatingPubKey  shard.BlsPublicKey `json:"validating_pub_key" yaml:"validating_pub_key"`
	Amount            *big.Int           `json:"amount" yaml:"amount"`
}

// NewMsgCreateValidator creates a new validator
func NewMsgCreateValidator(
	description Description, commission CommissionRates, minSelfDelegation *big.Int, stakingAddress common.Address, validatingPubKey shard.BlsPublicKey, amount *big.Int) MsgCreateValidator {

	return MsgCreateValidator{
		Description:       description,
		Commission:        commission,
		MinSelfDelegation: minSelfDelegation,
		StakingAddress:    stakingAddress,
		ValidatingPubKey:  validatingPubKey,
		Amount:            amount,
	}
}

// Type ...
func (msg MsgCreateValidator) Type() string { return "create_validator" }

// Signer ...
func (msg MsgCreateValidator) Signer() common.Address { return msg.StakingAddress }

// MarshalJSON implements the json.Marshaler interface to provide custom JSON
// serialization of the MsgCreateValidator type.
func (msg MsgCreateValidator) MarshalJSON() ([]byte, error) {
	return json.Marshal(msgCreateValidatorJSON{
		Description:       msg.Description,
		Commission:        msg.Commission,
		MinSelfDelegation: msg.MinSelfDelegation,
		StakingAddress:    msg.StakingAddress,
		ValidatingPubKey:  msg.ValidatingPubKey,
		Amount:            msg.Amount,
	})
}

// UnmarshalJSON implements the json.Unmarshaler interface to provide custom
// JSON deserialization of the MsgCreateValidator type.
func (msg *MsgCreateValidator) UnmarshalJSON(bz []byte) error {
	var msgCreateValJSON msgCreateValidatorJSON
	if err := json.Unmarshal(bz, &msgCreateValJSON); err != nil {
		return err
	}

	msg.Description = msgCreateValJSON.Description
	msg.Commission = msgCreateValJSON.Commission
	msg.MinSelfDelegation = msgCreateValJSON.MinSelfDelegation
	msg.StakingAddress = msgCreateValJSON.StakingAddress
	msg.ValidatingPubKey = msgCreateValJSON.ValidatingPubKey
	msg.Amount = msgCreateValJSON.Amount

	return nil
}

// ValidateBasic quick validity check
func (msg MsgCreateValidator) ValidateBasic() error {
	// note that unmarshaling from bech32 ensures either empty or valid
	if msg.StakingAddress.Big().Uint64() == 0 {
		return errors.New("[CreateValidator] address is empty")
	}
	emptyKey := shard.BlsPublicKey{}
	if bytes.Compare(msg.ValidatingPubKey[:], emptyKey[:]) == 0 {
		return errors.New("[CreateValidator] invalid BLS public key")
	}
	if msg.Description == (Description{}) {
		return errors.New("[CreateValidator] description must be included")
	}
	if msg.Commission == (CommissionRates{}) {
		return errors.New("[CreateValidator] commission must be included")
	}
	if msg.Amount.Cmp(msg.MinSelfDelegation) > 0 {
		return errors.New("[CreateValidator] stake amount must be >= MinSelfDelegation")
	}

	return nil
}

// MsgEditValidator - struct for editing a validator
type MsgEditValidator struct {
	Description
	StakingAddress common.Address `json:"staking_address" yaml:"staking_address"`

	CommissionRate    Dec      `json:"commission_rate" yaml:"commission_rate"`
	MinSelfDelegation *big.Int `json:"min_self_delegation" yaml:"min_self_delegation"`
}

// MsgEditValidatorJSON - struct for editing a validator for JSON
type MsgEditValidatorJSON struct {
	Description
	StakingAddress common.Address `json:"staking_address" yaml:"staking_address"`

	// TODO: allow update of bls public key
	CommissionRate    Dec      `json:"commission_rate" yaml:"commission_rate"`
	MinSelfDelegation *big.Int `json:"min_self_delegation" yaml:"min_self_delegation"`
}

// NewMsgEditValidator creates a new MsgEditValidator.
func NewMsgEditValidator(
	description Description, stakingAddress common.Address, commissionRate Dec, minSelfDelegation *big.Int) MsgEditValidator {

	return MsgEditValidator{
		Description:       description,
		StakingAddress:    stakingAddress,
		CommissionRate:    commissionRate,
		MinSelfDelegation: minSelfDelegation,
	}
}

// Type ...
func (msg MsgEditValidator) Type() string { return "edit_validator" }

// Signer ...
func (msg MsgEditValidator) Signer() common.Address { return msg.StakingAddress }

// MsgDelegate - struct for bonding transactions
type MsgDelegate struct {
	DelegatorAddress common.Address `json:"delegator_address" yaml:"delegator_address"`
	ValidatorAddress common.Address `json:"validator_address" yaml:"validator_address"`
	Amount           *big.Int       `json:"amount" yaml:"amount"`
}

// NewMsgDelegate creates a new MsgDelegate.
func NewMsgDelegate(
	validatorAddress common.Address, delegatorAddress common.Address, amount *big.Int) MsgDelegate {

	return MsgDelegate{
		DelegatorAddress: delegatorAddress,
		ValidatorAddress: validatorAddress,
		Amount:           amount,
	}
}

// Type ...
func (msg MsgDelegate) Type() string { return "delegate" }

// Signer ...
func (msg MsgDelegate) Signer() common.Address { return msg.DelegatorAddress }

// MsgRedelegate - struct for re-bonding transactions
type MsgRedelegate struct {
	DelegatorAddress    common.Address `json:"delegator_address" yaml:"delegator_address"`
	ValidatorSrcAddress common.Address `json:"validator_src_address" yaml:"validator_src_address"`
	ValidatorDstAddress common.Address `json:"validator_dst_address" yaml:"validator_dst_address"`
	Amount              *big.Int       `json:"amount" yaml:"amount"`
}

// NewMsgRedelegate creates a new MsgRedelegate.
func NewMsgRedelegate(delAddr, valSrcAddr, valDstAddr common.Address, amount *big.Int) MsgRedelegate {
	return MsgRedelegate{
		DelegatorAddress:    delAddr,
		ValidatorSrcAddress: valSrcAddr,
		ValidatorDstAddress: valDstAddr,
		Amount:              amount,
	}
}

// MsgUndelegate - struct for unbonding transactions
type MsgUndelegate struct {
	DelegatorAddress common.Address `json:"delegator_address" yaml:"delegator_address"`
	ValidatorAddress common.Address `json:"validator_address" yaml:"validator_address"`
	Amount           *big.Int       `json:"amount" yaml:"amount"`
}

// NewMsgUndelegate creates a new MsgUndelegate.
func NewMsgUndelegate(delAddr common.Address, valAddr common.Address, amount *big.Int) MsgUndelegate {
	return MsgUndelegate{
		DelegatorAddress: delAddr,
		ValidatorAddress: valAddr,
		Amount:           amount,
	}
}
