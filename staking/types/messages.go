package types

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/numeric"
	"github.com/harmony-one/harmony/shard"
	"github.com/harmony-one/harmony/staking/effective"
)

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

// Delegate - type for delegating to a validator
type Delegate struct {
	DelegatorAddress common.Address `json:"delegator_address"`
	ValidatorAddress common.Address `json:"validator_address"`
	Amount           *big.Int       `json:"amount"`
}

// Undelegate - type for removing delegation responsibility
type Undelegate struct {
	DelegatorAddress common.Address `json:"delegator_address"`
	ValidatorAddress common.Address `json:"validator_address"`
	Amount           *big.Int       `json:"amount"`
}

// CollectRewards - type for collecting token rewards
type CollectRewards struct {
	DelegatorAddress common.Address `json:"delegator_address"`
}

// TxMessage defines contract for valid transaction messages
type TxMessage interface {
	To() *common.Address
	FromShard() uint32
	ToShard() uint32
	Value() *big.Int
	Data() []byte
	Copy() TxMessage
	Cost() *big.Int
}

// To returns the message to address
func (v CreateValidator) To() *common.Address {
	return nil
}

// To returns the message to address
func (v EditValidator) To() *common.Address {
	return nil
}

// To returns the message to address
func (v Delegate) To() *common.Address {
	return &v.ValidatorAddress
}

// To returns the message to address
func (v Undelegate) To() *common.Address {
	return &v.ValidatorAddress
}

// To returns the message to address
func (v CollectRewards) To() *common.Address {
	return nil
}

// FromShard returns the message from shard id
func (v CreateValidator) FromShard() uint32 {
	return shard.BeaconChainShardID
}

// FromShard returns the message from shard id
func (v EditValidator) FromShard() uint32 {
	return shard.BeaconChainShardID
}

// FromShard returns the message from shard id
func (v Delegate) FromShard() uint32 {
	return shard.BeaconChainShardID
}

// FromShard returns the message from shard id
func (v Undelegate) FromShard() uint32 {
	return shard.BeaconChainShardID
}

// FromShard returns the message from shard id
func (v CollectRewards) FromShard() uint32 {
	return shard.BeaconChainShardID
}

// ToShard returns the message to shard id
func (v CreateValidator) ToShard() uint32 {
	return shard.BeaconChainShardID
}

// ToShard returns the message to shard id
func (v EditValidator) ToShard() uint32 {
	return shard.BeaconChainShardID
}

// ToShard returns the message to shard id
func (v Delegate) ToShard() uint32 {
	return shard.BeaconChainShardID
}

// ToShard returns the message to shard id
func (v Undelegate) ToShard() uint32 {
	return shard.BeaconChainShardID
}

// ToShard returns the message to shard id
func (v CollectRewards) ToShard() uint32 {
	return shard.BeaconChainShardID
}

// Value returns the message amount
func (v CreateValidator) Value() *big.Int {
	return new(big.Int).SetInt64(0)
}

// Value returns the message amount
func (v EditValidator) Value() *big.Int {
	return new(big.Int).SetInt64(0)
}

// Value returns the message amount
func (v Delegate) Value() *big.Int {
	return new(big.Int).SetInt64(0)
}

// Value returns the message amount
func (v Undelegate) Value() *big.Int {
	return new(big.Int).SetInt64(0)
}

// Value returns the message amount
func (v CollectRewards) Value() *big.Int {
	return new(big.Int).SetInt64(0)
}

// Data returns the message payload
func (v CreateValidator) Data() []byte {
	return []byte{}
}

// Data returns the message payload
func (v EditValidator) Data() []byte {
	return []byte{}
}

// Data returns the message payload
func (v Delegate) Data() []byte {
	return []byte{}
}

// Data returns the message payload
func (v Undelegate) Data() []byte {
	return []byte{}
}

// Data returns the message payload
func (v CollectRewards) Data() []byte {
	return []byte{}
}

// Copy deep copy of the interface
func (v CreateValidator) Copy() TxMessage {
	v1 := v
	v1.Description = v.Description
	return &v1
}

// Copy deep copy of the interface
func (v EditValidator) Copy() TxMessage {
	v1 := v
	v1.Description = v.Description
	return &v1
}

// Copy deep copy of the interface
func (v Delegate) Copy() TxMessage {
	v1 := v
	return &v1
}

// Copy deep copy of the interface
func (v Undelegate) Copy() TxMessage {
	v1 := v
	return &v1
}

// Copy deep copy of the interface
func (v CollectRewards) Copy() TxMessage {
	v1 := v
	return &v1
}

// Cost returns the message cost
func (v *CreateValidator) Cost() *big.Int {
	return v.Amount
}

// Cost returns the message cost
func (v *EditValidator) Cost() *big.Int {
	return common.Big0
}

// Cost returns the message cost
func (v *Delegate) Cost() *big.Int {
	return v.Amount
}

// Cost returns the message cost
func (v *Undelegate) Cost() *big.Int {
	return common.Big0
}

// Cost returns the message cost
func (v *CollectRewards) Cost() *big.Int {
	return common.Big0
}
