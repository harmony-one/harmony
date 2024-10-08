package v1

import (
	"fmt"
	"math/big"
	"strings"

	"github.com/pkg/errors"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/crypto/bls"
	internal_common "github.com/harmony-one/harmony/internal/common"
	staking "github.com/harmony-one/harmony/staking/types"
)

// BlockWithTxHash represents a block that will serialize to the RPC representation of a block
// having ONLY transaction hashes in the Transaction & Staking transaction fields.
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
	VRF              common.Hash    `json:"vrf"`
	VRFProof         hexutil.Bytes  `json:"vrfProof"`
	Timestamp        hexutil.Uint64 `json:"timestamp"`
	TransactionsRoot common.Hash    `json:"transactionsRoot"`
	ReceiptsRoot     common.Hash    `json:"receiptsRoot"`
	Uncles           []common.Hash  `json:"uncles"`
	Transactions     []common.Hash  `json:"transactions"`
	EthTransactions  []common.Hash  `json:"transactionsInEthHash"`
	StakingTxs       []common.Hash  `json:"stakingTransactions"`
	Signers          []string       `json:"signers,omitempty"`
}

// BlockWithFullTx represents a block that will serialize to the RPC representation of a block
// having FULL transactions in the Transaction & Staking transaction fields.
type BlockWithFullTx struct {
	Number           *hexutil.Big          `json:"number"`
	ViewID           *hexutil.Big          `json:"viewID"`
	Epoch            *hexutil.Big          `json:"epoch"`
	Hash             common.Hash           `json:"hash"`
	ParentHash       common.Hash           `json:"parentHash"`
	Nonce            uint64                `json:"nonce"`
	MixHash          common.Hash           `json:"mixHash"`
	LogsBloom        ethtypes.Bloom        `json:"logsBloom"`
	StateRoot        common.Hash           `json:"stateRoot"`
	Miner            string                `json:"miner"`
	Difficulty       uint64                `json:"difficulty"`
	ExtraData        hexutil.Bytes         `json:"extraData"`
	Size             hexutil.Uint64        `json:"size"`
	GasLimit         hexutil.Uint64        `json:"gasLimit"`
	GasUsed          hexutil.Uint64        `json:"gasUsed"`
	VRF              common.Hash           `json:"vrf"`
	VRFProof         hexutil.Bytes         `json:"vrfProof"`
	Timestamp        hexutil.Uint64        `json:"timestamp"`
	TransactionsRoot common.Hash           `json:"transactionsRoot"`
	ReceiptsRoot     common.Hash           `json:"receiptsRoot"`
	Uncles           []common.Hash         `json:"uncles"`
	Transactions     []*Transaction        `json:"transactions"`
	StakingTxs       []*StakingTransaction `json:"stakingTransactions"`
	Signers          []string              `json:"signers,omitempty"`
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
	EthHash          common.Hash    `json:"ethHash"`
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

// StakingTransaction represents a staking transaction that will serialize to the
// RPC representation of a staking transaction
type StakingTransaction struct {
	BlockHash        common.Hash    `json:"blockHash"`
	BlockNumber      *hexutil.Big   `json:"blockNumber"`
	From             string         `json:"from"`
	Timestamp        hexutil.Uint64 `json:"timestamp"`
	Gas              hexutil.Uint64 `json:"gas"`
	GasPrice         *hexutil.Big   `json:"gasPrice"`
	Hash             common.Hash    `json:"hash"`
	Nonce            hexutil.Uint64 `json:"nonce"`
	TransactionIndex hexutil.Uint   `json:"transactionIndex"`
	V                *hexutil.Big   `json:"v"`
	R                *hexutil.Big   `json:"r"`
	S                *hexutil.Big   `json:"s"`
	Type             string         `json:"type"`
	Msg              interface{}    `json:"msg"`
}

// CreateValidatorMsg represents a staking transaction's create validator directive that
// will serialize to the RPC representation
type CreateValidatorMsg struct {
	ValidatorAddress   string                    `json:"validatorAddress"`
	CommissionRate     *hexutil.Big              `json:"commissionRate"`
	MaxCommissionRate  *hexutil.Big              `json:"maxCommissionRate"`
	MaxChangeRate      *hexutil.Big              `json:"maxChangeRate"`
	MinSelfDelegation  *hexutil.Big              `json:"minSelfDelegation"`
	MaxTotalDelegation *hexutil.Big              `json:"maxTotalDelegation"`
	Amount             *hexutil.Big              `json:"amount"`
	Name               string                    `json:"name"`
	Website            string                    `json:"website"`
	Identity           string                    `json:"identity"`
	SecurityContact    string                    `json:"securityContact"`
	Details            string                    `json:"details"`
	SlotPubKeys        []bls.SerializedPublicKey `json:"slotPubKeys"`
}

// EditValidatorMsg represents a staking transaction's edit validator directive that
// will serialize to the RPC representation
type EditValidatorMsg struct {
	ValidatorAddress   string                   `json:"validatorAddress"`
	CommissionRate     *hexutil.Big             `json:"commissionRate"`
	MinSelfDelegation  *hexutil.Big             `json:"minSelfDelegation"`
	MaxTotalDelegation *hexutil.Big             `json:"maxTotalDelegation"`
	Name               string                   `json:"name"`
	Website            string                   `json:"website"`
	Identity           string                   `json:"identity"`
	SecurityContact    string                   `json:"securityContact"`
	Details            string                   `json:"details"`
	SlotPubKeyToAdd    *bls.SerializedPublicKey `json:"slotPubKeyToAdd"`
	SlotPubKeyToRemove *bls.SerializedPublicKey `json:"slotPubKeyToRemove"`
}

// CollectRewardsMsg represents a staking transaction's collect rewards directive that
// will serialize to the RPC representation
type CollectRewardsMsg struct {
	DelegatorAddress string `json:"delegatorAddress"`
}

// DelegateMsg represents a staking transaction's delegate directive that
// will serialize to the RPC representation
type DelegateMsg struct {
	DelegatorAddress string       `json:"delegatorAddress"`
	ValidatorAddress string       `json:"validatorAddress"`
	Amount           *hexutil.Big `json:"amount"`
}

// UndelegateMsg represents a staking transaction's delegate directive that
// will serialize to the RPC representation
type UndelegateMsg struct {
	DelegatorAddress string       `json:"delegatorAddress"`
	ValidatorAddress string       `json:"validatorAddress"`
	Amount           *hexutil.Big `json:"amount"`
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

// StakingTxReceipt represents a staking transaction receipt that will serialize to the RPC representation.
type StakingTxReceipt struct {
	BlockHash         common.Hash    `json:"blockHash"`
	TransactionHash   common.Hash    `json:"transactionHash"`
	BlockNumber       hexutil.Uint64 `json:"blockNumber"`
	TransactionIndex  hexutil.Uint64 `json:"transactionIndex"`
	GasUsed           hexutil.Uint64 `json:"gasUsed"`
	CumulativeGasUsed hexutil.Uint64 `json:"cumulativeGasUsed"`
	ContractAddress   common.Address `json:"contractAddress"`
	Logs              []*types.Log   `json:"logs"`
	LogsBloom         ethtypes.Bloom `json:"logsBloom"`
	Sender            string         `json:"sender"`
	Type              hexutil.Uint64 `json:"type"`
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

// NewCxReceipt returns a CxReceipt that will serialize to the RPC representation
func NewCxReceipt(cx *types.CXReceipt, blockHash common.Hash, blockNumber uint64) (*CxReceipt, error) {
	result := &CxReceipt{
		BlockHash: blockHash,
		TxHash:    cx.TxHash,
		Amount:    (*hexutil.Big)(cx.Amount),
		ShardID:   cx.ShardID,
		ToShardID: cx.ToShardID,
	}
	if blockHash != (common.Hash{}) {
		result.BlockHash = blockHash
		result.BlockNumber = (*hexutil.Big)(new(big.Int).SetUint64(blockNumber))
	}

	fromAddr, err := internal_common.AddressToBech32(cx.From)
	if err != nil {
		return nil, err
	}
	toAddr := ""
	if cx.To != nil {
		if toAddr, err = internal_common.AddressToBech32(*cx.To); err != nil {
			return nil, err
		}
	}
	result.From = fromAddr
	result.To = toAddr

	return result, nil
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
		EthHash:   tx.ConvertToEth().Hash(),
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

	fromAddr, err := internal_common.AddressToBech32(from)
	if err != nil {
		return nil, err
	}
	toAddr := ""

	if tx.To() != nil {
		if toAddr, err = internal_common.AddressToBech32(*tx.To()); err != nil {
			return nil, err
		}
		result.From = fromAddr
	} else {
		result.From = strings.ToLower(from.Hex())
	}
	result.To = toAddr

	return result, nil
}

// NewReceipt returns a transaction OR staking transaction that will serialize to the RPC
// representation.
func NewReceipt(
	tx interface{}, blockHash common.Hash, blockNumber, blockIndex uint64, receipt *types.Receipt,
) (interface{}, error) {
	plainTx, ok := tx.(*types.Transaction)
	if ok {
		return NewTxReceipt(plainTx, blockHash, blockNumber, blockIndex, receipt)
	}
	stakingTx, ok := tx.(*staking.StakingTransaction)
	if ok {
		return NewStakingTxReceipt(stakingTx, blockHash, blockNumber, blockIndex, receipt)
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
	var sender, receiver string
	if tx.To() == nil {
		// Handle response type for contract receipts
		sender = senderAddr.String()
		receiver = ""
	} else {
		// Handle response type for regular transaction
		sender, err = internal_common.AddressToBech32(senderAddr)
		if err != nil {
			return nil, err
		}
		receiver, err = internal_common.AddressToBech32(*tx.To())
		if err != nil {
			return nil, err
		}
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

// NewStakingTxReceipt returns a staking transaction receipt that will serialize to the RPC representation
func NewStakingTxReceipt(
	tx *staking.StakingTransaction, blockHash common.Hash, blockNumber, blockIndex uint64, receipt *types.Receipt,
) (*StakingTxReceipt, error) {
	// Set correct sender
	senderAddr, err := tx.SenderAddress()
	if err != nil {
		return nil, err
	}
	sender, err := internal_common.AddressToBech32(senderAddr)
	if err != nil {
		return nil, err
	}

	// Declare receipt
	txReceipt := &StakingTxReceipt{
		BlockHash:         blockHash,
		TransactionHash:   tx.Hash(),
		BlockNumber:       hexutil.Uint64(blockNumber),
		TransactionIndex:  hexutil.Uint64(blockIndex),
		GasUsed:           hexutil.Uint64(receipt.GasUsed),
		CumulativeGasUsed: hexutil.Uint64(receipt.CumulativeGasUsed),
		Logs:              receipt.Logs,
		LogsBloom:         receipt.Bloom,
		Sender:            sender,
		Type:              hexutil.Uint64(tx.StakingType()),
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

// NewStakingTransaction returns a staking transaction that will serialize to the RPC
// representation, with the given location metadata set (if available).
func NewStakingTransaction(tx *staking.StakingTransaction, blockHash common.Hash,
	blockNumber uint64, timestamp uint64, index uint64,
) (*StakingTransaction, error) {
	from, err := tx.SenderAddress()
	if err != nil {
		return nil, err
	}
	v, r, s := tx.RawSignatureValues()

	var rpcMsg interface{}
	switch tx.StakingType() {
	case staking.DirectiveCreateValidator:
		rawMsg, err := staking.RLPDecodeStakeMsg(tx.Data(), staking.DirectiveCreateValidator)
		if err != nil {
			return nil, err
		}
		msg, ok := rawMsg.(*staking.CreateValidator)
		if !ok {
			return nil, fmt.Errorf("could not decode staking message")
		}
		validatorAddress, err := internal_common.AddressToBech32(msg.ValidatorAddress)
		if err != nil {
			return nil, err
		}
		rpcMsg = &CreateValidatorMsg{
			ValidatorAddress:   validatorAddress,
			CommissionRate:     (*hexutil.Big)(msg.CommissionRates.Rate.Int),
			MaxCommissionRate:  (*hexutil.Big)(msg.CommissionRates.MaxRate.Int),
			MaxChangeRate:      (*hexutil.Big)(msg.CommissionRates.MaxChangeRate.Int),
			MinSelfDelegation:  (*hexutil.Big)(msg.MinSelfDelegation),
			MaxTotalDelegation: (*hexutil.Big)(msg.MaxTotalDelegation),
			Amount:             (*hexutil.Big)(msg.Amount),
			Name:               msg.Description.Name,
			Website:            msg.Description.Website,
			Identity:           msg.Description.Identity,
			SecurityContact:    msg.Description.SecurityContact,
			Details:            msg.Description.Details,
			SlotPubKeys:        msg.SlotPubKeys,
		}
	case staking.DirectiveEditValidator:
		rawMsg, err := staking.RLPDecodeStakeMsg(tx.Data(), staking.DirectiveEditValidator)
		if err != nil {
			return nil, err
		}
		msg, ok := rawMsg.(*staking.EditValidator)
		if !ok {
			return nil, fmt.Errorf("could not decode staking message")
		}
		validatorAddress, err := internal_common.AddressToBech32(msg.ValidatorAddress)
		if err != nil {
			return nil, err
		}
		// Edit validators txs need not have commission rates to edit
		commissionRate := &hexutil.Big{}
		if msg.CommissionRate != nil {
			commissionRate = (*hexutil.Big)(msg.CommissionRate.Int)
		}
		rpcMsg = &EditValidatorMsg{
			ValidatorAddress:   validatorAddress,
			CommissionRate:     commissionRate,
			MinSelfDelegation:  (*hexutil.Big)(msg.MinSelfDelegation),
			MaxTotalDelegation: (*hexutil.Big)(msg.MaxTotalDelegation),
			Name:               msg.Description.Name,
			Website:            msg.Description.Website,
			Identity:           msg.Description.Identity,
			SecurityContact:    msg.Description.SecurityContact,
			Details:            msg.Description.Details,
			SlotPubKeyToAdd:    msg.SlotKeyToAdd,
			SlotPubKeyToRemove: msg.SlotKeyToRemove,
		}
	case staking.DirectiveCollectRewards:
		rawMsg, err := staking.RLPDecodeStakeMsg(tx.Data(), staking.DirectiveCollectRewards)
		if err != nil {
			return nil, err
		}
		msg, ok := rawMsg.(*staking.CollectRewards)
		if !ok {
			return nil, fmt.Errorf("could not decode staking message")
		}
		delegatorAddress, err := internal_common.AddressToBech32(msg.DelegatorAddress)
		if err != nil {
			return nil, err
		}
		rpcMsg = &CollectRewardsMsg{DelegatorAddress: delegatorAddress}
	case staking.DirectiveDelegate:
		rawMsg, err := staking.RLPDecodeStakeMsg(tx.Data(), staking.DirectiveDelegate)
		if err != nil {
			return nil, err
		}
		msg, ok := rawMsg.(*staking.Delegate)
		if !ok {
			return nil, fmt.Errorf("could not decode staking message")
		}
		delegatorAddress, err := internal_common.AddressToBech32(msg.DelegatorAddress)
		if err != nil {
			return nil, err
		}
		validatorAddress, err := internal_common.AddressToBech32(msg.ValidatorAddress)
		if err != nil {
			return nil, err
		}
		rpcMsg = &DelegateMsg{
			DelegatorAddress: delegatorAddress,
			ValidatorAddress: validatorAddress,
			Amount:           (*hexutil.Big)(msg.Amount),
		}
	case staking.DirectiveUndelegate:
		rawMsg, err := staking.RLPDecodeStakeMsg(tx.Data(), staking.DirectiveUndelegate)
		if err != nil {
			return nil, err
		}
		msg, ok := rawMsg.(*staking.Undelegate)
		if !ok {
			return nil, fmt.Errorf("could not decode staking message")
		}
		delegatorAddress, err := internal_common.AddressToBech32(msg.DelegatorAddress)
		if err != nil {
			return nil, err
		}
		validatorAddress, err := internal_common.AddressToBech32(msg.ValidatorAddress)
		if err != nil {
			return nil, err
		}
		rpcMsg = &UndelegateMsg{
			DelegatorAddress: delegatorAddress,
			ValidatorAddress: validatorAddress,
			Amount:           (*hexutil.Big)(msg.Amount),
		}
	}

	result := &StakingTransaction{
		Gas:       hexutil.Uint64(tx.GasLimit()),
		GasPrice:  (*hexutil.Big)(tx.GasPrice()),
		Hash:      tx.Hash(),
		Nonce:     hexutil.Uint64(tx.Nonce()),
		Timestamp: hexutil.Uint64(timestamp),
		V:         (*hexutil.Big)(v),
		R:         (*hexutil.Big)(r),
		S:         (*hexutil.Big)(s),
		Type:      tx.StakingType().String(),
		Msg:       rpcMsg,
	}
	if blockHash != (common.Hash{}) {
		result.BlockHash = blockHash
		result.BlockNumber = (*hexutil.Big)(new(big.Int).SetUint64(blockNumber))
		result.TransactionIndex = hexutil.Uint(index)
	}

	fromAddr, err := internal_common.AddressToBech32(from)
	if err != nil {
		return nil, err
	}
	result.From = fromAddr

	return result, nil
}

func blockWithTxHashFromBlock(b *types.Block) *BlockWithTxHash {
	head := b.Header()

	vrfAndProof := head.Vrf()
	vrf := common.Hash{}
	vrfProof := []byte{}
	if len(vrfAndProof) == 32+96 {
		copy(vrf[:], vrfAndProof[:32])
		vrfProof = vrfAndProof[32:]
	}
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
		Difficulty:       0, // Remove this because we don't have it in our header
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
		Transactions:     []common.Hash{},
		EthTransactions:  []common.Hash{},
		StakingTxs:       []common.Hash{},
	}

	for _, tx := range b.Transactions() {
		blk.Transactions = append(blk.Transactions, tx.Hash())
		blk.EthTransactions = append(blk.EthTransactions, tx.ConvertToEth().Hash())
	}
	return blk
}

func blockWithFullTxFromBlock(b *types.Block) (*BlockWithFullTx, error) {
	head := b.Header()

	vrfAndProof := head.Vrf()
	vrf := common.Hash{}
	vrfProof := []byte{}
	if len(vrfAndProof) == 32+96 {
		copy(vrf[:], vrfAndProof[:32])
		vrfProof = vrfAndProof[32:]
	}
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
		Difficulty:       0, // Remove this because we don't have it in our header
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
		Transactions:     []*Transaction{},
		StakingTxs:       []*StakingTransaction{},
	}

	for _, tx := range b.Transactions() {
		fmtTx, err := NewTransactionFromBlockHash(b, tx.Hash())
		if err != nil {
			return nil, err
		}
		blk.Transactions = append(blk.Transactions, fmtTx)
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

// StakingTransactionsFromBlock return rpc staking transactions from a block
func StakingTransactionsFromBlock(b *types.Block) ([]*StakingTransaction, error) {
	rawStakings := b.StakingTransactions()
	rpcStakings := make([]*StakingTransaction, 0, len(rawStakings))
	for idx, raw := range rawStakings {
		rpcStk, err := NewStakingTransaction(raw, b.Hash(), b.NumberU64(), b.Time().Uint64(), uint64(idx))
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse staking transaction %v", raw.Hash())
		}
		rpcStakings = append(rpcStakings, rpcStk)
	}
	return rpcStakings, nil
}

// NewStakingTransactionFromBlockHash returns a staking transaction that will serialize to the RPC representation.
func NewStakingTransactionFromBlockHash(b *types.Block, hash common.Hash) (*StakingTransaction, error) {
	for idx, tx := range b.StakingTransactions() {
		if tx.Hash() == hash {
			return NewStakingTransactionFromBlockIndex(b, uint64(idx))
		}
	}
	return nil, fmt.Errorf("tx %v not found in block %v", hash, b.Hash().String())
}

// NewStakingTransactionFromBlockIndex returns a staking transaction that will serialize to the RPC representation.
func NewStakingTransactionFromBlockIndex(b *types.Block, index uint64) (*StakingTransaction, error) {
	txs := b.StakingTransactions()
	if index >= uint64(len(txs)) {
		return nil, fmt.Errorf(
			"tx index %v greater than or equal to number of transactions on block %v", index, b.Hash().String(),
		)
	}
	return NewStakingTransaction(txs[index], b.Hash(), b.NumberU64(), b.Time().Uint64(), index)
}
