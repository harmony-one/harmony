package hmyclient

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/core/types"
	types2 "github.com/harmony-one/harmony/staking/types"
)

type rpcBlock struct {
	Hash         common.Hash      `json:"hash"`
	Transactions []rpcTransaction `json:"transactions"`
	StakingTransactions []rpcStakingTransaction `json:"staking_transactions"`
	UncleHashes  []common.Hash    `json:"uncles"`
}

type rpcTransaction struct {
	tx *types.Transaction
	txExtraInfo
}

type txExtraInfo struct {
	BlockNumber *string         `json:"blockNumber,omitempty"`
	BlockHash   *common.Hash    `json:"blockHash,omitempty"`
	From        *common.Address `json:"from,omitempty"`
}

type rpcStakingTransaction struct {
	tx *types2.StakingTransaction
	txExtraInfo
}
