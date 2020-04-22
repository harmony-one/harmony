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

type TxMessage interface {
	To() *common.Address
	ShardId() uint32
	ToShardId() uint32
	Value() *big.Int
	Data() []byte
	Copy() TxMessage
	Cost() *big.Int
}

func (v CreateValidator) To() *common.Address {
	return nil
}

func (v EditValidator) To() *common.Address {
	return nil
}

func (v Delegate) To() *common.Address {
	return &v.ValidatorAddress
}

func (v Undelegate) To() *common.Address {
	return &v.ValidatorAddress
}

func (v CollectRewards) To() *common.Address {
	return nil
}

func (v CreateValidator) ShardId() uint32 {
	return shard.BeaconChainShardID
}

func (v EditValidator) ShardId() uint32 {
	return shard.BeaconChainShardID
}

func (v Delegate) ShardId() uint32 {
	return shard.BeaconChainShardID
}

func (v Undelegate) ShardId() uint32 {
	return shard.BeaconChainShardID
}

func (v CollectRewards) ShardId() uint32 {
	return shard.BeaconChainShardID
}

func (v CreateValidator) ToShardId() uint32 {
	return shard.BeaconChainShardID
}

func (v EditValidator) ToShardId() uint32 {
	return shard.BeaconChainShardID
}

func (v Delegate) ToShardId() uint32 {
	return shard.BeaconChainShardID
}

func (v Undelegate) ToShardId() uint32 {
	return shard.BeaconChainShardID
}

func (v CollectRewards) ToShardId() uint32 {
	return shard.BeaconChainShardID
}

func (v CreateValidator) Value() *big.Int {
	return new(big.Int).SetInt64(0)
}

func (v EditValidator) Value() *big.Int {
	return new(big.Int).SetInt64(0)
}

func (v Delegate) Value() *big.Int {
	return new(big.Int).SetInt64(0)
}

func (v Undelegate) Value() *big.Int {
	return new(big.Int).SetInt64(0)
}

func (v CollectRewards) Value() *big.Int {
	return new(big.Int).SetInt64(0)
}

func (v CreateValidator) Data() []byte {
	return []byte{}
}

func (v EditValidator) Data() []byte {
	return []byte{}
}

func (v Delegate) Data() []byte {
	return []byte{}
}

func (v Undelegate) Data() []byte {
	return []byte{}
}

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

// Cost ..
func (tx *CreateValidator) Cost() *big.Int {
	return tx.Amount
}

// Cost ..
func (tx *EditValidator) Cost() *big.Int {
	return common.Big0
}

// Cost ..
func (tx *Delegate) Cost() *big.Int {
	return tx.Amount
}

// Cost ..
func (tx *Undelegate) Cost() *big.Int {
	return common.Big0
}

// Cost ..
func (tx *CollectRewards) Cost() *big.Int {
	return common.Big0
}
