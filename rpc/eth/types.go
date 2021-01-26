package v1

import (
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/harmony-one/harmony/core/types"
	internal_common "github.com/harmony-one/harmony/internal/common"
	rpc_common "github.com/harmony-one/harmony/rpc/common"
	rpc_utils "github.com/harmony-one/harmony/rpc/utils"
)

// BlockWithTxHash represents a block that will serialize to the RPC representation of a block
// having ONLY transaction hashes in the Transaction fields.
type BlockWithTxHash struct {
	Number           *hexutil.Big   `json:"number"`
	ViewID           *hexutil.Big   `json:"viewID"`
	Epoch            *hexutil.Big   `json:"epoch"`
	Hash             common.Hash    `json:"hash"`
	ParentHash       common.Hash    `json:"parentHash"`
	Nonce            uint64         `json:"nonce"`
	MixHash          common.Hash    `json:"mixHash"`
	LogsBloom        ethtypes.Bloom `json:"logsBloom"`
	StateRoot        common.Hash    `json:"stateRoot"`
	Miner            string         `json:"miner"`
	Difficulty       uint64         `json:"difficulty"`
	ExtraData        hexutil.Bytes  `json:"extraData"`
	Size             hexutil.Uint64 `json:"size"`
	GasLimit         hexutil.Uint64 `json:"gasLimit"`
	GasUsed          hexutil.Uint64 `json:"gasUsed"`
	Timestamp        hexutil.Uint64 `json:"timestamp"`
	TransactionsRoot common.Hash    `json:"transactionsRoot"`
	ReceiptsRoot     common.Hash    `json:"receiptsRoot"`
	Uncles           []common.Hash  `json:"uncles"`
	Transactions     []common.Hash  `json:"transactions"`
	Signers          []string       `json:"signers,omitempty"`
}

// BlockWithFullTx represents a block that will serialize to the RPC representation of a block
// having FULL transactions in the Transaction fields.
type BlockWithFullTx struct {
	Number           *hexutil.Big   `json:"number"`
	ViewID           *hexutil.Big   `json:"viewID"`
	Epoch            *hexutil.Big   `json:"epoch"`
	Hash             common.Hash    `json:"hash"`
	ParentHash       common.Hash    `json:"parentHash"`
	Nonce            uint64         `json:"nonce"`
	MixHash          common.Hash    `json:"mixHash"`
	LogsBloom        ethtypes.Bloom `json:"logsBloom"`
	StateRoot        common.Hash    `json:"stateRoot"`
	Miner            string         `json:"miner"`
	Difficulty       uint64         `json:"difficulty"`
	ExtraData        hexutil.Bytes  `json:"extraData"`
	Size             hexutil.Uint64 `json:"size"`
	GasLimit         hexutil.Uint64 `json:"gasLimit"`
	GasUsed          hexutil.Uint64 `json:"gasUsed"`
	Timestamp        hexutil.Uint64 `json:"timestamp"`
	TransactionsRoot common.Hash    `json:"transactionsRoot"`
	ReceiptsRoot     common.Hash    `json:"receiptsRoot"`
	Uncles           []common.Hash  `json:"uncles"`
	Transactions     []*Transaction `json:"transactions"`
	Signers          []string       `json:"signers,omitempty"`
}

// Transaction represents a transaction that will serialize to the RPC representation of a transaction
type Transaction struct {
	BlockHash        common.Hash    `json:"blockHash"`
	BlockNumber      *hexutil.Big   `json:"blockNumber"`
	From             string         `json:"from"`
	Timestamp        hexutil.Uint64 `json:"timestamp"`
	Gas              hexutil.Uint64 `json:"gas"`
	GasPrice         *hexutil.Big   `json:"gasPrice"`
	Hash             common.Hash    `json:"hash"`
	Input            hexutil.Bytes  `json:"input"`
	Nonce            hexutil.Uint64 `json:"nonce"`
	To               string         `json:"to"`
	TransactionIndex hexutil.Uint   `json:"transactionIndex"`
	Value            *hexutil.Big   `json:"value"`
	ShardID          uint32         `json:"shardID"`
	ToShardID        uint32         `json:"toShardID"`
	V                *hexutil.Big   `json:"v"`
	R                *hexutil.Big   `json:"r"`
	S                *hexutil.Big   `json:"s"`
}

// TxReceipt represents a transaction receipt that will serialize to the RPC representation.
type TxReceipt struct {
	BlockHash         common.Hash    `json:"blockHash"`
	TransactionHash   common.Hash    `json:"transactionHash"`
	BlockNumber       hexutil.Uint64 `json:"blockNumber"`
	TransactionIndex  hexutil.Uint64 `json:"transactionIndex"`
	GasUsed           hexutil.Uint64 `json:"gasUsed"`
	CumulativeGasUsed hexutil.Uint64 `json:"cumulativeGasUsed"`
	ContractAddress   common.Address `json:"contractAddress"`
	Logs              []*types.Log   `json:"logs"`
	LogsBloom         ethtypes.Bloom `json:"logsBloom"`
	ShardID           uint32         `json:"shardID"`
	From              string         `json:"from"`
	To                string         `json:"to"`
	Root              hexutil.Bytes  `json:"root"`
	Status            hexutil.Uint   `json:"status"`
}

// CxReceipt represents a CxReceipt that will serialize to the RPC representation of a CxReceipt
type CxReceipt struct {
	BlockHash   common.Hash  `json:"blockHash"`
	BlockNumber *hexutil.Big `json:"blockNumber"`
	TxHash      common.Hash  `json:"hash"`
	From        string       `json:"from"`
	To          string       `json:"to"`
	ShardID     uint32       `json:"shardID"`
	ToShardID   uint32       `json:"toShardID"`
	Amount      *hexutil.Big `json:"value"`
}

// NewTransaction returns a transaction that will serialize to the RPC
// representation, with the given location metadata set (if available).
// Note that all txs on Harmony are replay protected (post EIP155 epoch).
func NewTransaction(
	tx *types.Transaction, blockHash common.Hash,
	blockNumber uint64, timestamp uint64, index uint64,
) (*Transaction, error) {
	from, err := tx.SenderAddress()
	if err != nil {
		return nil, err
	}
	v, r, s := tx.RawSignatureValues()

	result := &Transaction{
		Gas:       hexutil.Uint64(tx.GasLimit()),
		GasPrice:  (*hexutil.Big)(tx.GasPrice()),
		Hash:      tx.Hash(),
		Input:     hexutil.Bytes(tx.Data()),
		Nonce:     hexutil.Uint64(tx.Nonce()),
		Value:     (*hexutil.Big)(tx.Value()),
		ShardID:   tx.ShardID(),
		ToShardID: tx.ToShardID(),
		Timestamp: hexutil.Uint64(timestamp),
		V:         (*hexutil.Big)(v),
		R:         (*hexutil.Big)(r),
		S:         (*hexutil.Big)(s),
	}
	if blockHash != (common.Hash{}) {
		result.BlockHash = blockHash
		result.BlockNumber = (*hexutil.Big)(new(big.Int).SetUint64(blockNumber))
		result.TransactionIndex = hexutil.Uint(index)
	}

	result.From, result.To, err = rpc_utils.ConvertAddresses(&from, tx.To(), false)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// NewReceipt returns an ETH transaction transaction that will serialize to the RPC representation.
func NewReceipt(
	tx interface{}, blockHash common.Hash, blockNumber, blockIndex uint64, receipt *types.Receipt,
) (interface{}, error) {
	plainTx, ok := tx.(*types.Transaction)
	if ok {
		return NewTxReceipt(plainTx, blockHash, blockNumber, blockIndex, receipt)
	}
	return nil, fmt.Errorf("unknown transaction type for RPC receipt")
}

// NewTxReceipt returns a transaction receipt that will serialize to the RPC representation
func NewTxReceipt(
	tx *types.Transaction, blockHash common.Hash, blockNumber, blockIndex uint64, receipt *types.Receipt,
) (*TxReceipt, error) {
	// Set correct to & from address
	senderAddr, err := tx.SenderAddress()
	if err != nil {
		return nil, err
	}

	sender, receiver, err := rpc_utils.ConvertAddresses(&senderAddr, tx.To(), false)
	if err != nil {
		return nil, err
	}

	// Declare receipt
	txReceipt := &TxReceipt{
		BlockHash:         blockHash,
		TransactionHash:   tx.Hash(),
		BlockNumber:       hexutil.Uint64(blockNumber),
		TransactionIndex:  hexutil.Uint64(blockIndex),
		GasUsed:           hexutil.Uint64(receipt.GasUsed),
		CumulativeGasUsed: hexutil.Uint64(receipt.CumulativeGasUsed),
		Logs:              receipt.Logs,
		LogsBloom:         receipt.Bloom,
		ShardID:           tx.ShardID(),
		From:              sender,
		To:                receiver,
		Root:              receipt.PostState,
		Status:            hexutil.Uint(receipt.Status),
	}

	// Set empty array for empty logs
	if receipt.Logs == nil {
		txReceipt.Logs = []*types.Log{}
	}

	// If the ContractAddress is 20 0x0 bytes, assume it is not a contract creation
	if receipt.ContractAddress != (common.Address{}) {
		txReceipt.ContractAddress = receipt.ContractAddress
	}
	return txReceipt, nil
}

// NewBlock converts the given block to the RPC output which depends on fullTx. If inclTx is true transactions are
// returned. When fullTx is true the returned block contains full transaction details, otherwise it will only contain
// transaction hashes.
func NewBlock(b *types.Block, blockArgs *rpc_common.BlockArgs, leader string) (interface{}, error) {
	if strings.HasPrefix(leader, "one1") {
		// Handle hex address
		addr, err := internal_common.Bech32ToAddress(leader)
		if err != nil {
			return nil, err
		}
		leader = addr.String()
	}

	if blockArgs.FullTx {
		return NewBlockWithFullTx(b, blockArgs, leader)
	}
	return NewBlockWithTxHash(b, blockArgs, leader)
}

// NewBlockWithTxHash ..
func NewBlockWithTxHash(
	b *types.Block, blockArgs *rpc_common.BlockArgs, leader string,
) (*BlockWithTxHash, error) {
	if blockArgs.FullTx {
		return nil, fmt.Errorf("block args specifies full tx, but requested RPC block with only tx hash")
	}

	head := b.Header()
	blk := &BlockWithTxHash{
		Number:           (*hexutil.Big)(head.Number()),
		ViewID:           (*hexutil.Big)(head.ViewID()),
		Epoch:            (*hexutil.Big)(head.Epoch()),
		Hash:             b.Hash(),
		ParentHash:       head.ParentHash(),
		Nonce:            0, // Remove this because we don't have it in our header
		MixHash:          head.MixDigest(),
		LogsBloom:        head.Bloom(),
		StateRoot:        head.Root(),
		Miner:            leader,
		Difficulty:       0, // Remove this because we don't have it in our header
		ExtraData:        hexutil.Bytes(head.Extra()),
		Size:             hexutil.Uint64(b.Size()),
		GasLimit:         hexutil.Uint64(head.GasLimit()),
		GasUsed:          hexutil.Uint64(head.GasUsed()),
		Timestamp:        hexutil.Uint64(head.Time().Uint64()),
		TransactionsRoot: head.TxHash(),
		ReceiptsRoot:     head.ReceiptHash(),
		Uncles:           []common.Hash{},
		Transactions:     []common.Hash{},
	}

	for _, tx := range b.Transactions() {
		blk.Transactions = append(blk.Transactions, tx.Hash())
	}

	if blockArgs.WithSigners {
		blk.Signers = blockArgs.Signers
	}
	return blk, nil
}

// NewBlockWithFullTx ..
func NewBlockWithFullTx(
	b *types.Block, blockArgs *rpc_common.BlockArgs, leader string,
) (*BlockWithFullTx, error) {
	if !blockArgs.FullTx {
		return nil, fmt.Errorf("block args specifies NO full tx, but requested RPC block with full tx")
	}

	head := b.Header()
	blk := &BlockWithFullTx{
		Number:           (*hexutil.Big)(head.Number()),
		ViewID:           (*hexutil.Big)(head.ViewID()),
		Epoch:            (*hexutil.Big)(head.Epoch()),
		Hash:             b.Hash(),
		ParentHash:       head.ParentHash(),
		Nonce:            0, // Remove this because we don't have it in our header
		MixHash:          head.MixDigest(),
		LogsBloom:        head.Bloom(),
		StateRoot:        head.Root(),
		Miner:            leader,
		Difficulty:       0, // Remove this because we don't have it in our header
		ExtraData:        hexutil.Bytes(head.Extra()),
		Size:             hexutil.Uint64(b.Size()),
		GasLimit:         hexutil.Uint64(head.GasLimit()),
		GasUsed:          hexutil.Uint64(head.GasUsed()),
		Timestamp:        hexutil.Uint64(head.Time().Uint64()),
		TransactionsRoot: head.TxHash(),
		ReceiptsRoot:     head.ReceiptHash(),
		Uncles:           []common.Hash{},
		Transactions:     []*Transaction{},
	}

	for _, tx := range b.Transactions() {
		fmtTx, err := NewTransactionFromBlockHash(b, tx.Hash())
		if err != nil {
			return nil, err
		}
		blk.Transactions = append(blk.Transactions, fmtTx)
	}

	if blockArgs.WithSigners {
		blk.Signers = blockArgs.Signers
	}

	return blk, nil
}

// NewTransactionFromBlockHash returns a transaction that will serialize to the RPC representation.
func NewTransactionFromBlockHash(b *types.Block, hash common.Hash) (*Transaction, error) {
	for idx, tx := range b.Transactions() {
		if tx.Hash() == hash {
			return NewTransactionFromBlockIndex(b, uint64(idx))
		}
	}
	return nil, fmt.Errorf("tx %v not found in block %v", hash, b.Hash().String())
}

// NewTransactionFromBlockIndex returns a transaction that will serialize to the RPC representation.
func NewTransactionFromBlockIndex(b *types.Block, index uint64) (*Transaction, error) {
	txs := b.Transactions()
	if index >= uint64(len(txs)) {
		return nil, fmt.Errorf(
			"tx index %v greater than or equal to number of transactions on block %v", index, b.Hash().String(),
		)
	}
	return NewTransaction(txs[index], b.Hash(), b.NumberU64(), b.Time().Uint64(), index)
}
