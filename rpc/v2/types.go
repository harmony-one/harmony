package v2

import (
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/crypto/bls"
	internal_common "github.com/harmony-one/harmony/internal/common"
	rpc_common "github.com/harmony-one/harmony/rpc/common"
	staking "github.com/harmony-one/harmony/staking/types"
)

// RPCBlock represents a block that will serialize to the RPC representation of a block
type RPCBlock struct {
	Number           *big.Int         `json:"number"`
	ViewID           *big.Int         `json:"viewID"`
	Epoch            *big.Int         `json:"epoch"`
	Hash             common.Hash      `json:"hash"`
	ParentHash       common.Hash      `json:"parentHash"`
	Nonce            types.BlockNonce `json:"nonce"`
	MixHash          common.Hash      `json:"mixHash"`
	LogsBloom        ethtypes.Bloom   `json:"logsBloom"`
	StateRoot        common.Hash      `json:"stateRoot"`
	Miner            string           `json:"miner"`
	Difficulty       *big.Int         `json:"difficulty"`
	ExtraData        []byte           `json:"extraData"`
	Size             uint64           `json:"size"`
	GasLimit         uint64           `json:"gasLimit"`
	GasUsed          uint64           `json:"gasUsed"`
	Timestamp        *big.Int         `json:"timestamp"`
	TransactionsRoot common.Hash      `json:"transactionsRoot"`
	ReceiptsRoot     common.Hash      `json:"receiptsRoot"`
	Transactions     []interface{}    `json:"transactions"`
	StakingTxs       []interface{}    `json:"stakingTxs"`
	Uncles           []common.Hash    `json:"uncles"`
	TotalDifficulty  *big.Int         `json:"totalDifficulty"`
	Signers          []string         `json:"signers"`
}

// RPCBlockWithTxHash represents a block that will serialize to the RPC representation of a block
// having ONLY transaction hashes in the Transaction & Staking transaction fields.
type RPCBlockWithTxHash struct {
	Number           *big.Int       `json:"number"`
	ViewID           *big.Int       `json:"viewID"`
	Epoch            *big.Int       `json:"epoch"`
	Hash             common.Hash    `json:"hash"`
	ParentHash       common.Hash    `json:"parentHash"`
	Nonce            uint64         `json:"nonce"`
	MixHash          common.Hash    `json:"mixHash"`
	LogsBloom        ethtypes.Bloom `json:"logsBloom"`
	StateRoot        common.Hash    `json:"stateRoot"`
	Miner            string         `json:"miner"`
	Difficulty       uint64         `json:"difficulty"`
	ExtraData        hexutil.Bytes  `json:"extraData"`
	Size             uint64         `json:"size"`
	GasLimit         uint64         `json:"gasLimit"`
	GasUsed          uint64         `json:"gasUsed"`
	Timestamp        *big.Int       `json:"timestamp"`
	TransactionsRoot common.Hash    `json:"transactionsRoot"`
	ReceiptsRoot     common.Hash    `json:"receiptsRoot"`
	Uncles           []common.Hash  `json:"uncles"`
	Transactions     []common.Hash  `json:"transactions"`
	StakingTxs       []common.Hash  `json:"stakingTransactions"`
	Signers          []string       `json:"signers,omitempty"`
}

// RPCBlockWithFullTx represents a block that will serialize to the RPC representation of a block
// having FULL transactions in the Transaction & Staking transaction fields.
type RPCBlockWithFullTx struct {
	Number           *big.Int                 `json:"number"`
	ViewID           *big.Int                 `json:"viewID"`
	Epoch            *big.Int                 `json:"epoch"`
	Hash             common.Hash              `json:"hash"`
	ParentHash       common.Hash              `json:"parentHash"`
	Nonce            uint64                   `json:"nonce"`
	MixHash          common.Hash              `json:"mixHash"`
	LogsBloom        ethtypes.Bloom           `json:"logsBloom"`
	StateRoot        common.Hash              `json:"stateRoot"`
	Miner            string                   `json:"miner"`
	Difficulty       uint64                   `json:"difficulty"`
	ExtraData        hexutil.Bytes            `json:"extraData"`
	Size             uint64                   `json:"size"`
	GasLimit         uint64                   `json:"gasLimit"`
	GasUsed          uint64                   `json:"gasUsed"`
	Timestamp        *big.Int                 `json:"timestamp"`
	TransactionsRoot common.Hash              `json:"transactionsRoot"`
	ReceiptsRoot     common.Hash              `json:"receiptsRoot"`
	Uncles           []common.Hash            `json:"uncles"`
	Transactions     []*RPCTransaction        `json:"transactions"`
	StakingTxs       []*RPCStakingTransaction `json:"stakingTransactions"`
	Signers          []string                 `json:"signers,omitempty"`
}

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
	BlockHash        common.Hash  `json:"blockHash"`
	BlockNumber      *big.Int     `json:"blockNumber"`
	From             string       `json:"from"`
	Timestamp        uint64       `json:"timestamp"`
	Gas              uint64       `json:"gas"`
	GasPrice         *big.Int     `json:"gasPrice"`
	Hash             common.Hash  `json:"hash"`
	Nonce            uint64       `json:"nonce"`
	TransactionIndex uint64       `json:"transactionIndex"`
	V                *hexutil.Big `json:"v"`
	R                *hexutil.Big `json:"r"`
	S                *hexutil.Big `json:"s"`
	Type             string       `json:"type"`
	Msg              interface{}  `json:"msg"`
}

// RPCCreateValidatorMsg represents a staking transaction's create validator directive that
// will serialize to the RPC representation
type RPCCreateValidatorMsg struct {
	ValidatorAddress   string                    `json:"validatorAddress"`
	CommissionRate     *big.Int                  `json:"commissionRate"`
	MaxCommissionRate  *big.Int                  `json:"maxCommissionRate"`
	MaxChangeRate      *big.Int                  `json:"maxChangeRate"`
	MinSelfDelegation  *big.Int                  `json:"minSelfDelegation"`
	MaxTotalDelegation *big.Int                  `json:"maxTotalDelegation"`
	Amount             *big.Int                  `json:"amount"`
	Name               string                    `json:"name"`
	Website            string                    `json:"website"`
	Identity           string                    `json:"identity"`
	SecurityContact    string                    `json:"securityContact"`
	Details            string                    `json:"details"`
	SlotPubKeys        []bls.SerializedPublicKey `json:"slotPubKeys"`
}

// RPCEditValidatorMsg represents a staking transaction's edit validator directive that
// will serialize to the RPC representation
type RPCEditValidatorMsg struct {
	ValidatorAddress   string                   `json:"validatorAddress"`
	CommissionRate     *big.Int                 `json:"commissionRate"`
	MinSelfDelegation  *big.Int                 `json:"minSelfDelegation"`
	MaxTotalDelegation *big.Int                 `json:"maxTotalDelegation"`
	Name               string                   `json:"name"`
	Website            string                   `json:"website"`
	Identity           string                   `json:"identity"`
	SecurityContact    string                   `json:"securityContact"`
	Details            string                   `json:"details"`
	SlotPubKeyToAdd    *bls.SerializedPublicKey `json:"slotPubKeyToAdd"`
	SlotPubKeyToRemove *bls.SerializedPublicKey `json:"slotPubKeyToRemove"`
}

// RPCCollectRewardsMsg represents a staking transaction's collect rewards directive that
// will serialize to the RPC representation
type RPCCollectRewardsMsg struct {
	DelegatorAddress string `json:"delegatorAddress"`
}

// RPCCollectRewardsMsg represents a staking transaction's delegate directive that
// will serialize to the RPC representation
type RPCDelegateMsg struct {
	DelegatorAddress string   `json:"delegatorAddress"`
	ValidatorAddress string   `json:"validatorAddress"`
	Amount           *big.Int `json:"amount"`
}

// RPCUndelegateMsg represents a staking transaction's delegate directive that
// will serialize to the RPC representation
type RPCUndelegateMsg struct {
	DelegatorAddress string   `json:"delegatorAddress"`
	ValidatorAddress string   `json:"validatorAddress"`
	Amount           *big.Int `json:"amount"`
}

type RPCTxReceipt struct {
	BlockHash         common.Hash    `json:"blockHash"`
	TransactionHash   common.Hash    `json:"transactionHash"`
	BlockNumber       uint64         `json:"blockNumber"`
	TransactionIndex  uint64         `json:"transactionIndex"`
	GasUsed           uint64         `json:"gasUsed"`
	CumulativeGasUsed uint64         `json:"cumulativeGasUsed"`
	ContractAddress   common.Address `json:"contractAddress"`
	Logs              []*types.Log   `json:"logs"`
	LogsBloom         ethtypes.Bloom `json:"logsBloom"`
	ShardID           uint32         `json:"shardID"`
	From              string         `json:"from"`
	To                string         `json:"to"`
	Root              hexutil.Bytes  `json:"root,omitempty"`
	Status            uint           `json:"status,omitempty"`
}

type RPCStakingTxReceipt struct {
	BlockHash         common.Hash       `json:"blockHash"`
	TransactionHash   common.Hash       `json:"transactionHash"`
	BlockNumber       uint64            `json:"blockNumber"`
	TransactionIndex  uint64            `json:"transactionIndex"`
	GasUsed           uint64            `json:"gasUsed"`
	CumulativeGasUsed uint64            `json:"cumulativeGasUsed"`
	ContractAddress   common.Address    `json:"contractAddress"`
	Logs              []*types.Log      `json:"logs"`
	LogsBloom         ethtypes.Bloom    `json:"logsBloom"`
	Sender            string            `json:"sender"`
	Type              staking.Directive `json:"type"`
	Root              hexutil.Bytes     `json:"root,omitempty"`
	Status            uint              `json:"status,omitempty"`
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

// NewRPCCXReceipt returns a CXReceipt that will serialize to the RPC representation
func NewRPCCXReceipt(cx *types.CXReceipt, blockHash common.Hash, blockNumber uint64) (*RPCCXReceipt, error) {
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

// NewRPCTransaction returns a transaction that will serialize to the RPC
// representation, with the given location metadata set (if available).
// Note that all txs on Harmony are replay protected (post EIP155 epoch).
func NewRPCTransaction(
	tx *types.Transaction, blockHash common.Hash,
	blockNumber uint64, timestamp uint64, index uint64,
) (*RPCTransaction, error) {
	from, err := tx.SenderAddress()
	if err != nil {
		return nil, err
	}
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
		result.BlockNumber = new(big.Int).SetUint64(blockNumber)
		result.TransactionIndex = index
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

// NewRPCReceipt returns a transaction OR staking transaction that will serialize to the RPC representation
func NewRPCReceipt(
	tx interface{}, blockHash common.Hash, blockNumber, blockIndex uint64, receipt *types.Receipt,
) (interface{}, error) {
	plainTx, ok := tx.(*types.Transaction)
	if ok {
		return NewRPCTxReceipt(plainTx, blockHash, blockIndex, blockNumber, receipt)
	}
	stakingTx, ok := tx.(*staking.StakingTransaction)
	if ok {
		return NewRPCStakingTxReceipt(stakingTx, blockHash, blockIndex, blockNumber, receipt)
	}
	return nil, fmt.Errorf("unknown transaction type for RPC receipt")
}

// NewRPCTxReceipt returns a plain transaction receipt that will serialize to the RPC representation
func NewRPCTxReceipt(
	tx *types.Transaction, blockHash common.Hash, blockNumber, blockIndex uint64, receipt *types.Receipt,
) (*RPCTxReceipt, error) {
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
	txReceipt := &RPCTxReceipt{
		BlockHash:         blockHash,
		TransactionHash:   tx.Hash(),
		BlockNumber:       blockNumber,
		TransactionIndex:  blockIndex,
		GasUsed:           receipt.GasUsed,
		CumulativeGasUsed: receipt.CumulativeGasUsed,
		Logs:              receipt.Logs,
		LogsBloom:         receipt.Bloom,
		ShardID:           tx.ShardID(),
		From:              sender,
		To:                receiver,
	}

	// Set optionals
	if len(receipt.PostState) > 0 {
		txReceipt.Root = receipt.PostState
	} else {
		txReceipt.Status = uint(receipt.Status)
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

// NewRPCStakingTxReceipt returns a staking transaction receipt that will serialize to the RPC representation
func NewRPCStakingTxReceipt(
	tx *staking.StakingTransaction, blockHash common.Hash, blockNumber, blockIndex uint64, receipt *types.Receipt,
) (*RPCStakingTxReceipt, error) {
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
	txReceipt := &RPCStakingTxReceipt{
		BlockHash:         blockHash,
		TransactionHash:   tx.Hash(),
		BlockNumber:       blockNumber,
		TransactionIndex:  blockIndex,
		GasUsed:           receipt.GasUsed,
		CumulativeGasUsed: receipt.CumulativeGasUsed,
		Logs:              receipt.Logs,
		LogsBloom:         receipt.Bloom,
		Sender:            sender,
		Type:              tx.StakingType(),
	}

	// Set optionals
	if len(receipt.PostState) > 0 {
		txReceipt.Root = receipt.PostState
	} else {
		txReceipt.Status = uint(receipt.Status)
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

// NewRPCStakingTransaction returns a staking transaction that will serialize to the RPC
// representation, with the given location metadata set (if available).
func NewRPCStakingTransaction(
	tx *staking.StakingTransaction, blockHash common.Hash,
	blockNumber uint64, timestamp uint64, index uint64,
) (*RPCStakingTransaction, error) {
	from, err := tx.SenderAddress()
	if err != nil {
		return nil, nil
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
		rpcMsg = &RPCCreateValidatorMsg{
			ValidatorAddress:   validatorAddress,
			CommissionRate:     msg.CommissionRates.Rate.Int,
			MaxCommissionRate:  msg.CommissionRates.MaxRate.Int,
			MaxChangeRate:      msg.CommissionRates.MaxChangeRate.Int,
			MinSelfDelegation:  msg.MinSelfDelegation,
			MaxTotalDelegation: msg.MaxTotalDelegation,
			Amount:             msg.Amount,
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
		commissionRate := &big.Int{}
		if msg.CommissionRate != nil {
			commissionRate = msg.CommissionRate.Int
		}
		rpcMsg = &RPCEditValidatorMsg{
			ValidatorAddress:   validatorAddress,
			CommissionRate:     commissionRate,
			MinSelfDelegation:  msg.MinSelfDelegation,
			MaxTotalDelegation: msg.MaxTotalDelegation,
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
		rpcMsg = &RPCCollectRewardsMsg{DelegatorAddress: delegatorAddress}
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
		rpcMsg = &RPCDelegateMsg{
			DelegatorAddress: delegatorAddress,
			ValidatorAddress: validatorAddress,
			Amount:           msg.Amount,
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
		rpcMsg = &RPCUndelegateMsg{
			DelegatorAddress: delegatorAddress,
			ValidatorAddress: validatorAddress,
			Amount:           msg.Amount,
		}
	}

	result := &RPCStakingTransaction{
		Gas:       tx.Gas(),
		GasPrice:  tx.GasPrice(),
		Hash:      tx.Hash(),
		Nonce:     tx.Nonce(),
		Timestamp: timestamp,
		V:         (*hexutil.Big)(v),
		R:         (*hexutil.Big)(r),
		S:         (*hexutil.Big)(s),
		Type:      tx.StakingType().String(),
		Msg:       rpcMsg,
	}
	if blockHash != (common.Hash{}) {
		result.BlockHash = blockHash
		result.BlockNumber = new(big.Int).SetUint64(blockNumber)
		result.TransactionIndex = index
	}

	fromAddr, err := internal_common.AddressToBech32(from)
	if err != nil {
		return nil, err
	}
	result.From = fromAddr

	return result, nil
}

// NewRPCBlock converts the given block to the RPC output which depends on fullTx. If inclTx is true transactions are
// returned. When fullTx is true the returned block contains full transaction details, otherwise it will only contain
// transaction hashes.
func NewRPCBlock(b *types.Block, blockArgs *rpc_common.BlockArgs, leader string) (interface{}, error) {
	if blockArgs.FullTx {
		return NewRPCBlockWithFullTx(b, blockArgs, leader)
	}
	return NewRPCBlockWithTxHash(b, blockArgs, leader)
}

// NewRPCBlockWithTxHash return a block with only the transaction hash that will serialize to the RPC representation
func NewRPCBlockWithTxHash(
	b *types.Block, blockArgs *rpc_common.BlockArgs, leader string,
) (*RPCBlockWithTxHash, error) {
	if blockArgs.FullTx {
		return nil, fmt.Errorf("block args specifies full tx, but requested RPC block with only tx hash")
	}

	head := b.Header()
	blk := &RPCBlockWithTxHash{
		Number:           head.Number(),
		ViewID:           head.ViewID(),
		Epoch:            head.Epoch(),
		Hash:             b.Hash(),
		ParentHash:       head.ParentHash(),
		Nonce:            0, // Remove this because we don't have it in our header
		MixHash:          head.MixDigest(),
		LogsBloom:        head.Bloom(),
		StateRoot:        head.Root(),
		Miner:            leader,
		Difficulty:       0, // Remove this because we don't have it in our header
		ExtraData:        hexutil.Bytes(head.Extra()),
		Size:             uint64(b.Size()),
		GasLimit:         head.GasLimit(),
		GasUsed:          head.GasUsed(),
		Timestamp:        head.Time(),
		TransactionsRoot: head.TxHash(),
		ReceiptsRoot:     head.ReceiptHash(),
		Uncles:           []common.Hash{},
		Transactions:     []common.Hash{},
		StakingTxs:       []common.Hash{},
	}

	for _, tx := range b.Transactions() {
		blk.Transactions = append(blk.Transactions, tx.Hash())
	}

	if blockArgs.InclStaking {
		for _, stx := range b.StakingTransactions() {
			blk.StakingTxs = append(blk.StakingTxs, stx.Hash())
		}
	}

	if blockArgs.WithSigners {
		blk.Signers = blockArgs.Signers
	}
	return blk, nil
}

// NewRPCBlockWithFullTx return a block with the transaction that will serialize to the RPC representation
func NewRPCBlockWithFullTx(
	b *types.Block, blockArgs *rpc_common.BlockArgs, leader string,
) (*RPCBlockWithFullTx, error) {
	if !blockArgs.FullTx {
		return nil, fmt.Errorf("block args specifies NO full tx, but requested RPC block with full tx")
	}

	head := b.Header()
	blk := &RPCBlockWithFullTx{
		Number:           head.Number(),
		ViewID:           head.ViewID(),
		Epoch:            head.Epoch(),
		Hash:             b.Hash(),
		ParentHash:       head.ParentHash(),
		Nonce:            0, // Remove this because we don't have it in our header
		MixHash:          head.MixDigest(),
		LogsBloom:        head.Bloom(),
		StateRoot:        head.Root(),
		Miner:            leader,
		Difficulty:       0, // Remove this because we don't have it in our header
		ExtraData:        hexutil.Bytes(head.Extra()),
		Size:             uint64(b.Size()),
		GasLimit:         head.GasLimit(),
		GasUsed:          head.GasUsed(),
		Timestamp:        head.Time(),
		TransactionsRoot: head.TxHash(),
		ReceiptsRoot:     head.ReceiptHash(),
		Uncles:           []common.Hash{},
		Transactions:     []*RPCTransaction{},
		StakingTxs:       []*RPCStakingTransaction{},
	}

	for _, tx := range b.Transactions() {
		fmtTx, err := NewRPCTransactionFromBlockHash(b, tx.Hash())
		if err != nil {
			return nil, err
		}
		blk.Transactions = append(blk.Transactions, fmtTx)
	}

	if blockArgs.InclStaking {
		for _, stx := range b.StakingTransactions() {
			fmtStx, err := NewRPCStakingTransactionFromBlockHash(b, stx.Hash())
			if err != nil {
				return nil, err
			}
			blk.StakingTxs = append(blk.StakingTxs, fmtStx)
		}
	}

	if blockArgs.WithSigners {
		blk.Signers = blockArgs.Signers
	}
	return blk, nil
}

// NewRPCTransactionFromBlockHash returns a transaction that will serialize to the RPC representation.
func NewRPCTransactionFromBlockHash(b *types.Block, hash common.Hash) (*RPCTransaction, error) {
	for idx, tx := range b.Transactions() {
		if tx.Hash() == hash {
			return NewRPCTransactionFromBlockIndex(b, uint64(idx))
		}
	}
	return nil, fmt.Errorf("tx %v not found in block %v", hash, b.Hash().String())
}

// NewRPCTransactionFromBlockIndex returns a transaction that will serialize to the RPC representation.
func NewRPCTransactionFromBlockIndex(b *types.Block, index uint64) (*RPCTransaction, error) {
	txs := b.Transactions()
	if index >= uint64(len(txs)) {
		return nil, fmt.Errorf(
			"tx index %v greater than or equal to number of transactions on block %v", index, b.Hash().String(),
		)
	}
	return NewRPCTransaction(txs[index], b.Hash(), b.NumberU64(), b.Time().Uint64(), index)
}

// NewRPCStakingTransactionFromBlockHash returns a staking transaction that will serialize to the RPC representation.
func NewRPCStakingTransactionFromBlockHash(b *types.Block, hash common.Hash) (*RPCStakingTransaction, error) {
	for idx, tx := range b.StakingTransactions() {
		if tx.Hash() == hash {
			return NewRPCStakingTransactionFromBlockIndex(b, uint64(idx))
		}
	}
	return nil, fmt.Errorf("tx %v not found in block %v", hash, b.Hash().String())
}

// NewRPCStakingTransactionFromBlockIndex returns a staking transaction that will serialize to the RPC representation.
func NewRPCStakingTransactionFromBlockIndex(b *types.Block, index uint64) (*RPCStakingTransaction, error) {
	txs := b.StakingTransactions()
	if index >= uint64(len(txs)) {
		return nil, fmt.Errorf(
			"tx index %v greater than or equal to number of transactions on block %v", index, b.Hash().String(),
		)
	}
	return NewRPCStakingTransaction(txs[index], b.Hash(), b.NumberU64(), b.Time().Uint64(), index)
}
