package apiv2

import (
	"encoding/hex"
	"math/big"
	"strings"
	"time"

	types2 "github.com/harmony-one/harmony/staking/types"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/harmony-one/harmony/block"
	"github.com/harmony-one/harmony/core/types"
	internal_common "github.com/harmony-one/harmony/internal/common"
)

// RPCTransaction represents a transaction that will serialize to the RPC representation of a transaction
type RPCTransaction struct {
	BlockHash        common.Hash   `json:"blockHash"`
	BlockNumber      *big.Int      `json:"blockNumber"`
	From             string        `json:"from"`
	Timestamp        uint64        `json:"timestamp"`
	Gas              uint64        `json:"gas"`
	GasPrice         *big.Int      `json:"gasPrice"`
	Hash             common.Hash   `json:"hash"`
	Input            hexutil.Bytes `json:"input"`
	Nonce            uint64        `json:"nonce"`
	To               string        `json:"to"`
	TransactionIndex uint64        `json:"transactionIndex"`
	Value            *big.Int      `json:"value"`
	ShardID          uint32        `json:"shardID"`
	ToShardID        uint32        `json:"toShardID"`
	V                *hexutil.Big  `json:"v"`
	R                *hexutil.Big  `json:"r"`
	S                *hexutil.Big  `json:"s"`
}

// RPCStakingTransaction represents a transaction that will serialize to the RPC representation of a staking transaction
type RPCStakingTransaction struct {
	BlockHash        common.Hash            `json:"blockHash"`
	BlockNumber      *big.Int               `json:"blockNumber"`
	From             string                 `json:"from"`
	Timestamp        uint64                 `json:"timestamp"`
	Gas              uint64                 `json:"gas"`
	GasPrice         *big.Int               `json:"gasPrice"`
	Hash             common.Hash            `json:"hash"`
	Nonce            uint64                 `json:"nonce"`
	TransactionIndex uint64                 `json:"transactionIndex"`
	V                *hexutil.Big           `json:"v"`
	R                *hexutil.Big           `json:"r"`
	S                *hexutil.Big           `json:"s"`
	Type             string                 `json:"type"`
	Msg              map[string]interface{} `json:"msg"`
}

// RPCCXReceipt represents a CXReceipt that will serialize to the RPC representation of a CXReceipt
type RPCCXReceipt struct {
	BlockHash   common.Hash `json:"blockHash"`
	BlockNumber *big.Int    `json:"blockNumber"`
	TxHash      common.Hash `json:"hash"`
	From        string      `json:"from"`
	To          string      `json:"to"`
	ShardID     uint32      `json:"shardID"`
	ToShardID   uint32      `json:"toShardID"`
	Amount      *big.Int    `json:"value"`
}

// HeaderInformation represents the latest consensus information
type HeaderInformation struct {
	BlockHash        common.Hash `json:"blockHash"`
	BlockNumber      uint64      `json:"blockNumber"`
	ShardID          uint32      `json:"shardID"`
	Leader           string      `json:"leader"`
	ViewID           uint64      `json:"viewID"`
	Epoch            uint64      `json:"epoch"`
	Timestamp        string      `json:"timestamp"`
	UnixTime         uint64      `json:"unixtime"`
	LastCommitSig    string      `json:"lastCommitSig"`
	LastCommitBitmap string      `json:"lastCommitBitmap"`
}

// RPCDelegation represents a particular delegation to a validator
type RPCDelegation struct {
	ValidatorAddress string            `json:"validator_address"`
	DelegatorAddress string            `json:"delegator_address"`
	Amount           *big.Int          `json:"amount"`
	Reward           *big.Int          `json:"reward"`
	Undelegations    []RPCUndelegation `json:"Undelegations"`
}

// RPCUndelegation represents one undelegation entry
type RPCUndelegation struct {
	Amount *big.Int
	Epoch  *big.Int
}

func newHeaderInformation(header *block.Header) *HeaderInformation {
	if header == nil {
		return nil
	}

	result := &HeaderInformation{
		BlockHash:        header.Hash(),
		BlockNumber:      header.Number().Uint64(),
		ShardID:          header.ShardID(),
		ViewID:           header.ViewID().Uint64(),
		Epoch:            header.Epoch().Uint64(),
		UnixTime:         header.Time().Uint64(),
		Timestamp:        time.Unix(header.Time().Int64(), 0).UTC().String(),
		LastCommitBitmap: hex.EncodeToString(header.LastCommitBitmap()),
	}

	sig := header.LastCommitSignature()
	result.LastCommitSig = hex.EncodeToString(sig[:])

	bechAddr, err := internal_common.AddressToBech32(header.Coinbase())
	if err != nil {
		bechAddr = header.Coinbase().Hex()
	}

	result.Leader = bechAddr
	return result
}

// newRPCCXReceipt returns a CXReceipt that will serialize to the RPC representation
func newRPCCXReceipt(cx *types.CXReceipt, blockHash common.Hash, blockNumber uint64) *RPCCXReceipt {
	result := &RPCCXReceipt{
		BlockHash: blockHash,
		TxHash:    cx.TxHash,
		Amount:    cx.Amount,
		ShardID:   cx.ShardID,
		ToShardID: cx.ToShardID,
	}
	if blockHash != (common.Hash{}) {
		result.BlockHash = blockHash
		result.BlockNumber = (*big.Int)(new(big.Int).SetUint64(blockNumber))
	}

	fromAddr, err := internal_common.AddressToBech32(cx.From)
	if err != nil {
		return nil
	}
	toAddr := ""
	if cx.To != nil {
		if toAddr, err = internal_common.AddressToBech32(*cx.To); err != nil {
			return nil
		}
	}
	result.From = fromAddr
	result.To = toAddr

	return result
}

// newRPCTransaction returns a transaction that will serialize to the RPC
// representation, with the given location metadata set (if available).
func newRPCTransaction(tx *types.Transaction, blockHash common.Hash, blockNumber uint64, timestamp uint64, index uint64) *RPCTransaction {
	var signer types.Signer = types.FrontierSigner{}
	if tx.Protected() {
		signer = types.NewEIP155Signer(tx.ChainID())
	}
	from, _ := types.Sender(signer, tx)
	v, r, s := tx.RawSignatureValues()

	result := &RPCTransaction{
		Gas:       tx.Gas(),
		GasPrice:  tx.GasPrice(),
		Hash:      tx.Hash(),
		Input:     hexutil.Bytes(tx.Data()),
		Nonce:     tx.Nonce(),
		Value:     tx.Value(),
		ShardID:   tx.ShardID(),
		ToShardID: tx.ToShardID(),
		Timestamp: timestamp,
		V:         (*hexutil.Big)(v),
		R:         (*hexutil.Big)(r),
		S:         (*hexutil.Big)(s),
	}
	if blockHash != (common.Hash{}) {
		result.BlockHash = blockHash
		result.BlockNumber = (*big.Int)(new(big.Int).SetUint64(blockNumber))
		result.TransactionIndex = index
	}

	fromAddr, err := internal_common.AddressToBech32(from)
	if err != nil {
		return nil
	}
	toAddr := ""

	if tx.To() != nil {
		if toAddr, err = internal_common.AddressToBech32(*tx.To()); err != nil {
			return nil
		}
		result.From = fromAddr
	} else {
		result.From = strings.ToLower(from.Hex())
	}
	result.To = toAddr

	return result
}

// newRPCStakingTransaction returns a transaction that will serialize to the RPC
// representation, with the given location metadata set (if available).
func newRPCStakingTransaction(tx *types2.StakingTransaction, blockHash common.Hash, blockNumber uint64, timestamp uint64, index uint64) *RPCStakingTransaction {
	from, _ := tx.SenderAddress()
	v, r, s := tx.RawSignatureValues()

	stakingTxType := tx.StakingType().String()
	message := tx.StakingMessage()
	fields := make(map[string]interface{}, 0)

	switch stakingTxType {
	case "CreateValidator":
		msg := message.(types2.CreateValidator)
		validatorAddress, err := internal_common.AddressToBech32(msg.ValidatorAddress)
		if err != nil {
			return nil
		}
		fields = map[string]interface{}{
			"validatorAddress":   validatorAddress,
			"name":               msg.Description.Name,
			"commissionRate":     (*big.Int)(msg.CommissionRates.Rate.Int),
			"maxCommissionRate":  (*big.Int)(msg.CommissionRates.MaxRate.Int),
			"maxChangeRate":      (*big.Int)(msg.CommissionRates.MaxChangeRate.Int),
			"minSelfDelegation":  (*big.Int)(msg.MinSelfDelegation),
			"maxTotalDelegation": (*big.Int)(msg.MaxTotalDelegation),
			"amount":             (*big.Int)(msg.Amount),
			"website":            msg.Description.Website,
			"identity":           msg.Description.Identity,
			"securityContact":    msg.Description.SecurityContact,
			"details":            msg.Description.Details,
			"slotPubKeys":        msg.SlotPubKeys,
		}
	case "EditValidator":
		msg := message.(types2.EditValidator)
		validatorAddress, err := internal_common.AddressToBech32(msg.ValidatorAddress)
		if err != nil {
			return nil
		}
		fields = map[string]interface{}{
			"validatorAddress":   validatorAddress,
			"commisionRate":      (*big.Int)(msg.CommissionRate.Int),
			"name":               msg.Description.Name,
			"minSelfDelegation":  (*big.Int)(msg.MinSelfDelegation),
			"maxTotalDelegation": (*big.Int)(msg.MaxTotalDelegation),
			"website":            msg.Description.Website,
			"identity":           msg.Description.Identity,
			"securityContact":    msg.Description.SecurityContact,
			"details":            msg.Description.Details,
			"slotPubKeyToAdd":    msg.SlotKeyToAdd,
			"slotPubKeyToRemove": msg.SlotKeyToRemove,
		}
	case "CollectRewards":
		msg := message.(types2.CollectRewards)
		delegatorAddress, err := internal_common.AddressToBech32(msg.DelegatorAddress)
		if err != nil {
			return nil
		}
		fields = map[string]interface{}{
			"delegatorAddress": delegatorAddress,
		}
	case "Delegate":
		msg := message.(types2.Delegate)
		delegatorAddress, err := internal_common.AddressToBech32(msg.DelegatorAddress)
		if err != nil {
			return nil
		}
		validatorAddress, err := internal_common.AddressToBech32(msg.ValidatorAddress)
		if err != nil {
			return nil
		}
		fields = map[string]interface{}{
			"delegatorAddress": delegatorAddress,
			"validatorAddress": validatorAddress,
			"amount":           (*big.Int)(msg.Amount),
		}
	case "Undelegate":
		msg := message.(types2.Undelegate)
		delegatorAddress, err := internal_common.AddressToBech32(msg.DelegatorAddress)
		if err != nil {
			return nil
		}
		validatorAddress, err := internal_common.AddressToBech32(msg.ValidatorAddress)
		if err != nil {
			return nil
		}
		fields = map[string]interface{}{
			"delegatorAddress": delegatorAddress,
			"validatorAddress": validatorAddress,
			"amount":           (*big.Int)(msg.Amount),
		}
	}

	result := &RPCStakingTransaction{
		Gas:       tx.Gas(),
		GasPrice:  tx.Price(),
		Hash:      tx.Hash(),
		Nonce:     tx.Nonce(),
		Timestamp: timestamp,
		V:         (*hexutil.Big)(v),
		R:         (*hexutil.Big)(r),
		S:         (*hexutil.Big)(s),
		Type:      stakingTxType,
		Msg:       fields,
	}
	if blockHash != (common.Hash{}) {
		result.BlockHash = blockHash
		result.BlockNumber = (*big.Int)(new(big.Int).SetUint64(blockNumber))
		result.TransactionIndex = index
	}

	fromAddr, err := internal_common.AddressToBech32(from)
	if err != nil {
		return nil
	}
	result.From = fromAddr

	return result
}

// newRPCPendingTransaction returns a pending transaction that will serialize to the RPC representation
func newRPCPendingTransaction(tx *types.Transaction) *RPCTransaction {
	return newRPCTransaction(tx, common.Hash{}, 0, 0, 0)
}

// RPCBlock represents a block that will serialize to the RPC representation of a block
type RPCBlock struct {
	Number           *big.Int         `json:"number"`
	Hash             common.Hash      `json:"hash"`
	ParentHash       common.Hash      `json:"parentHash"`
	Nonce            types.BlockNonce `json:"nonce"`
	MixHash          common.Hash      `json:"mixHash"`
	LogsBloom        ethtypes.Bloom   `json:"logsBloom"`
	StateRoot        common.Hash      `json:"stateRoot"`
	Miner            common.Address   `json:"miner"`
	Difficulty       *big.Int         `json:"difficulty"`
	ExtraData        []byte           `json:"extraData"`
	Size             uint64           `json:"size"`
	GasLimit         uint64           `json:"gasLimit"`
	GasUsed          uint64           `json:"gasUsed"`
	Timestamp        *big.Int         `json:"timestamp"`
	TransactionsRoot common.Hash      `json:"transactionsRoot"`
	ReceiptsRoot     common.Hash      `json:"receiptsRoot"`
	Transactions     []interface{}    `json:"transactions"`
	StakingTxs       []interface{}    `json:"stakingTxs`
	Uncles           []common.Hash    `json:"uncles"`
	TotalDifficulty  *big.Int         `json:"totalDifficulty"`
	Signers          []string         `json:"signers"`
}

// RPCMarshalBlock converts the given block to the RPC output which depends on fullTx. If inclTx is true transactions are
// returned. When fullTx is true the returned block contains full transaction details, otherwise it will only contain
// transaction hashes.
func RPCMarshalBlock(b *types.Block, blockArgs BlockArgs) (map[string]interface{}, error) {
	head := b.Header() // copies the header once
	fields := map[string]interface{}{
		"number":           (*big.Int)(head.Number()),
		"hash":             b.Hash(),
		"parentHash":       head.ParentHash(),
		"nonce":            0, // Remove this because we don't have it in our header
		"mixHash":          head.MixDigest(),
		"logsBloom":        head.Bloom(),
		"stateRoot":        head.Root(),
		"miner":            head.Coinbase(),
		"difficulty":       0, // Remove this because we don't have it in our header
		"extraData":        hexutil.Bytes(head.Extra()),
		"size":             uint64(b.Size()),
		"gasLimit":         head.GasLimit(),
		"gasUsed":          head.GasUsed(),
		"timestamp":        head.Time().Uint64(),
		"transactionsRoot": head.TxHash(),
		"receiptsRoot":     head.ReceiptHash(),
	}

	if blockArgs.InclTx {
		formatTx := func(tx *types.Transaction) (interface{}, error) {
			return tx.Hash(), nil
		}
		if blockArgs.FullTx {
			formatTx = func(tx *types.Transaction) (interface{}, error) {
				return newRPCTransactionFromBlockHash(b, tx.Hash()), nil
			}
		}
		txs := b.Transactions()
		transactions := make([]interface{}, len(txs))
		var err error
		for i, tx := range txs {
			if transactions[i], err = formatTx(tx); err != nil {
				return nil, err
			}
		}
		fields["transactions"] = transactions

		if blockArgs.InclStaking {
			formatStakingTx := func(tx *types2.StakingTransaction) (interface{}, error) {
				return tx.Hash(), nil
			}
			if blockArgs.FullTx {
				formatStakingTx = func(tx *types2.StakingTransaction) (interface{}, error) {
					return newRPCStakingTransactionFromBlockHash(b, tx.Hash()), nil
				}
			}
			stakingTxs := b.StakingTransactions()
			stakingTransactions := make([]interface{}, len(stakingTxs))
			for i, tx := range stakingTxs {
				if stakingTransactions[i], err = formatStakingTx(tx); err != nil {
					return nil, err
				}
			}
			fields["stakingTransactions"] = stakingTransactions
		}
	}

	uncles := b.Uncles()
	uncleHashes := make([]common.Hash, len(uncles))
	for i, uncle := range uncles {
		uncleHashes[i] = uncle.Hash()
	}
	fields["uncles"] = uncleHashes
	if blockArgs.WithSigners {
		fields["signers"] = blockArgs.Signers
	}
	return fields, nil
}

// newRPCTransactionFromBlockHash returns a transaction that will serialize to the RPC representation.
func newRPCTransactionFromBlockHash(b *types.Block, hash common.Hash) *RPCTransaction {
	for idx, tx := range b.Transactions() {
		if tx.Hash() == hash {
			return newRPCTransactionFromBlockIndex(b, uint64(idx))
		}
	}
	return nil
}

// newRPCTransactionFromBlockIndex returns a transaction that will serialize to the RPC representation.
func newRPCTransactionFromBlockIndex(b *types.Block, index uint64) *RPCTransaction {
	txs := b.Transactions()
	if index >= uint64(len(txs)) {
		return nil
	}
	return newRPCTransaction(txs[index], b.Hash(), b.NumberU64(), b.Time().Uint64(), index)
}

// newRPCStakingTransactionFromBlockHash returns a transaction that will serialize to the RPC representation.
func newRPCStakingTransactionFromBlockHash(b *types.Block, hash common.Hash) *RPCStakingTransaction {
	for idx, tx := range b.StakingTransactions() {
		if tx.Hash() == hash {
			return newRPCStakingTransactionFromBlockIndex(b, uint64(idx))
		}
	}
	return nil
}

// newRPCStakingTransactionFromBlockIndex returns a transaction that will serialize to the RPC representation.
func newRPCStakingTransactionFromBlockIndex(b *types.Block, index uint64) *RPCStakingTransaction {
	txs := b.StakingTransactions()
	if index >= uint64(len(txs)) {
		return nil
	}
	return newRPCStakingTransaction(txs[index], b.Hash(), b.NumberU64(), b.Time().Uint64(), index)
}

// CallArgs represents the arguments for a call.
type CallArgs struct {
	From     *common.Address `json:"from"`
	To       *common.Address `json:"to"`
	Gas      *hexutil.Uint64 `json:"gas"`
	GasPrice *hexutil.Big    `json:"gasPrice"`
	Value    *hexutil.Big    `json:"value"`
	Data     *hexutil.Bytes  `json:"data"`
}
