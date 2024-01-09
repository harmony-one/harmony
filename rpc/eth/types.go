package eth

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/harmony-one/harmony/core/types"
	hmytypes "github.com/harmony-one/harmony/core/types"
)

// Block represents a basic block which is further amended by BlockWithTxHash or BlockWithFullTx
type Block struct {
	Number           *hexutil.Big        `json:"number"`
	Hash             common.Hash         `json:"hash"`
	ParentHash       common.Hash         `json:"parentHash"`
	Nonce            hmytypes.BlockNonce `json:"nonce"`
	MixHash          common.Hash         `json:"mixHash"`
	UncleHash        common.Hash         `json:"sha3Uncles"`
	LogsBloom        ethtypes.Bloom      `json:"logsBloom"`
	StateRoot        common.Hash         `json:"stateRoot"`
	Miner            common.Address      `json:"miner"`
	Difficulty       *hexutil.Big        `json:"difficulty"`
	ExtraData        hexutil.Bytes       `json:"extraData"`
	Size             hexutil.Uint64      `json:"size"`
	GasLimit         hexutil.Uint64      `json:"gasLimit"`
	GasUsed          hexutil.Uint64      `json:"gasUsed"`
	VRF              common.Hash         `json:"vrf"`
	VRFProof         hexutil.Bytes       `json:"vrfProof"`
	Timestamp        hexutil.Uint64      `json:"timestamp"`
	TransactionsRoot common.Hash         `json:"transactionsRoot"`
	ReceiptsRoot     common.Hash         `json:"receiptsRoot"`
	Uncles           []common.Hash       `json:"uncles"`
}

// BlockWithTxHash represents a block that will serialize to the RPC representation of a block
// having ONLY transaction hashes in the Transaction fields.
type BlockWithTxHash struct {
	*Block
	Transactions []common.Hash `json:"transactions"`
	Signers      []string      `json:"signers,omitempty"`
}

// BlockWithFullTx represents a block that will serialize to the RPC representation of a block
// having FULL transactions in the Transaction fields.
type BlockWithFullTx struct {
	*Block
	Transactions []*Transaction `json:"transactions"`
	Signers      []string       `json:"signers,omitempty"`
}

// Transaction represents a transaction that will serialize to the RPC representation of a transaction
type Transaction struct {
	BlockHash        *common.Hash    `json:"blockHash"`
	BlockNumber      *hexutil.Big    `json:"blockNumber"`
	From             common.Address  `json:"from"`
	Timestamp        hexutil.Uint64  `json:"timestamp"` // Not exposed by Ethereum anymore
	Gas              hexutil.Uint64  `json:"gas"`
	GasPrice         *hexutil.Big    `json:"gasPrice"`
	Hash             common.Hash     `json:"hash"`
	Input            hexutil.Bytes   `json:"input"`
	Nonce            hexutil.Uint64  `json:"nonce"`
	To               *common.Address `json:"to"`
	TransactionIndex *hexutil.Uint64 `json:"transactionIndex"`
	Value            *hexutil.Big    `json:"value"`
	V                *hexutil.Big    `json:"v"`
	R                *hexutil.Big    `json:"r"`
	S                *hexutil.Big    `json:"s"`
}

// NewTransaction returns a transaction that will serialize to the RPC
// representation, with the given location metadata set (if available).
// Note that all txs on Harmony are replay protected (post EIP155 epoch).
func NewTransaction(
	from common.Address, tx *types.EthTransaction, blockHash common.Hash,
	blockNumber uint64, timestamp uint64, index uint64,
) (*Transaction, error) {
	v, r, s := tx.RawSignatureValues()

	result := &Transaction{
		From:      from,
		Gas:       hexutil.Uint64(tx.GasLimit()),
		GasPrice:  (*hexutil.Big)(tx.GasPrice()),
		Hash:      tx.Hash(),
		Input:     hexutil.Bytes(tx.Data()),
		Nonce:     hexutil.Uint64(tx.Nonce()),
		To:        tx.To(),
		Value:     (*hexutil.Big)(tx.Value()),
		Timestamp: hexutil.Uint64(timestamp),
		V:         (*hexutil.Big)(v),
		R:         (*hexutil.Big)(r),
		S:         (*hexutil.Big)(s),
	}
	if blockHash != (common.Hash{}) {
		result.BlockHash = &blockHash
		result.BlockNumber = (*hexutil.Big)(new(big.Int).SetUint64(blockNumber))
		result.TransactionIndex = (*hexutil.Uint64)(&index)
	}
	return result, nil
}
func NewTransactionFromTransaction(
	tx *types.Transaction, blockHash common.Hash,
	blockNumber uint64, timestamp uint64, index uint64,
) (*Transaction, error) {
	from, err := tx.SenderAddress()
	if err != nil {
		return nil, fmt.Errorf("unable to get sender address: %w", err)
	}
	v, r, s := tx.RawSignatureValues()

	result := &Transaction{
		From:      from,
		Gas:       hexutil.Uint64(tx.GasLimit()),
		GasPrice:  (*hexutil.Big)(tx.GasPrice()),
		Hash:      tx.Hash(),
		Input:     hexutil.Bytes(tx.Data()),
		Nonce:     hexutil.Uint64(tx.Nonce()),
		To:        tx.To(),
		Value:     (*hexutil.Big)(tx.Value()),
		Timestamp: hexutil.Uint64(timestamp),
		V:         (*hexutil.Big)(v),
		R:         (*hexutil.Big)(r),
		S:         (*hexutil.Big)(s),
	}
	if blockHash != (common.Hash{}) {
		result.BlockHash = &blockHash
		result.BlockNumber = (*hexutil.Big)(new(big.Int).SetUint64(blockNumber))
		result.TransactionIndex = (*hexutil.Uint64)(&index)
	}
	return result, nil
}

// NewReceipt returns the RPC data for a new receipt
func NewReceipt(senderAddr common.Address, tx *types.EthTransaction, blockHash common.Hash, blockNumber, blockIndex uint64, receipt *types.Receipt) (map[string]interface{}, error) {
	ethTxHash := tx.Hash()
	for i := range receipt.Logs {
		// Override log txHash with receipt's
		receipt.Logs[i].TxHash = ethTxHash
	}

	fields := map[string]interface{}{
		"blockHash":         blockHash,
		"blockNumber":       hexutil.Uint64(blockNumber),
		"transactionHash":   ethTxHash,
		"transactionIndex":  hexutil.Uint64(blockIndex),
		"from":              senderAddr,
		"to":                tx.To(),
		"gasUsed":           hexutil.Uint64(receipt.GasUsed),
		"cumulativeGasUsed": hexutil.Uint64(receipt.CumulativeGasUsed),
		"contractAddress":   nil,
		"logs":              receipt.Logs,
		"logsBloom":         receipt.Bloom,
	}

	// Assign receipt status or post state.
	if len(receipt.PostState) > 0 {
		fields["root"] = hexutil.Bytes(receipt.PostState)
	} else {
		fields["status"] = hexutil.Uint(receipt.Status)
	}
	if receipt.Logs == nil {
		fields["logs"] = [][]*types.Log{}
	}
	// If the ContractAddress is 20 0x0 bytes, assume it is not a contract creation
	if receipt.ContractAddress != (common.Address{}) {
		fields["contractAddress"] = receipt.ContractAddress
	}

	return fields, nil
}

func newBlock(b *types.Block) *Block {
	head := b.Header()

	vrfAndProof := head.Vrf()
	vrf := common.Hash{}
	vrfProof := []byte{}
	if len(vrfAndProof) == 32+96 {
		copy(vrf[:], vrfAndProof[:32])
		vrfProof = vrfAndProof[32:]
	}
	return &Block{
		Number:           (*hexutil.Big)(head.Number()),
		Hash:             b.Hash(),
		ParentHash:       head.ParentHash(),
		Nonce:            hmytypes.BlockNonce{}, // Legacy comment from hmy -> eth RPC porting: "Remove this because we don't have it in our header"
		MixHash:          head.MixDigest(),
		UncleHash:        hmytypes.CalcUncleHash(b.Uncles()),
		LogsBloom:        head.Bloom(),
		StateRoot:        head.Root(),
		Difficulty:       (*hexutil.Big)(big.NewInt(0)), // Legacy comment from hmy -> eth RPC porting: "Remove this because we don't have it in our header"
		ExtraData:        hexutil.Bytes(head.Extra()),
		Size:             hexutil.Uint64(b.Size()),
		GasLimit:         hexutil.Uint64(head.GasLimit()),
		GasUsed:          hexutil.Uint64(head.GasUsed()),
		VRF:              vrf,
		VRFProof:         vrfProof,
		Timestamp:        hexutil.Uint64(head.Time().Uint64()),
		TransactionsRoot: head.TxHash(),
		ReceiptsRoot:     head.ReceiptHash(),
		Uncles:           []common.Hash{},
	}
}

func blockWithTxHashFromBlock(b *types.Block) *BlockWithTxHash {
	blk := newBlock(b)
	blkWithTxs := &BlockWithTxHash{
		Block:        blk,
		Transactions: []common.Hash{},
	}

	for _, tx := range b.Transactions() {
		blkWithTxs.Transactions = append(blkWithTxs.Transactions, tx.ConvertToEth().Hash())
	}
	return blkWithTxs
}

func blockWithFullTxFromBlock(b *types.Block) (*BlockWithFullTx, error) {
	blk := newBlock(b)
	blkWithTxs := &BlockWithFullTx{
		Block:        blk,
		Transactions: []*Transaction{},
	}

	for idx, tx := range b.Transactions() {
		from, err := tx.SenderAddress()
		if err != nil {
			return nil, err
		}
		fmtTx, err := NewTransaction(from, tx.ConvertToEth(), b.Hash(), b.NumberU64(), b.Time().Uint64(), uint64(idx))
		if err != nil {
			return nil, err
		}
		blkWithTxs.Transactions = append(blkWithTxs.Transactions, fmtTx)
	}
	return blkWithTxs, nil
}

// NewTransactionFromBlockIndex returns a transaction that will serialize to the RPC representation.
func NewTransactionFromBlockIndex(b *types.Block, index uint64) (*Transaction, error) {
	txs := b.Transactions()
	if index >= uint64(len(txs)) {
		return nil, fmt.Errorf(
			"tx index %v greater than or equal to number of transactions on block %v", index, b.Hash().String(),
		)
	}
	tx := txs[index].ConvertToEth()
	from, err := tx.SenderAddress()
	if err != nil {
		return nil, err
	}
	return NewTransaction(from, tx, b.Hash(), b.NumberU64(), b.Time().Uint64(), index)
}
