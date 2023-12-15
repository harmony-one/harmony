package v2

import (
	"fmt"
	"math/big"
	"strings"

	"github.com/pkg/errors"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/harmony-one/harmony/block"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/crypto/bls"
	internal_common "github.com/harmony-one/harmony/internal/common"
	staking "github.com/harmony-one/harmony/staking/types"
)

// BlockWithTxHash represents a block that will serialize to the RPC representation of a block
// having ONLY transaction hashes in the Transaction & Staking transaction fields.
type BlockWithTxHash struct {
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
	VRF              common.Hash    `json:"vrf"`
	VRFProof         hexutil.Bytes  `json:"vrfProof"`
	Timestamp        *big.Int       `json:"timestamp"`
	TransactionsRoot common.Hash    `json:"transactionsRoot"`
	ReceiptsRoot     common.Hash    `json:"receiptsRoot"`
	Uncles           []common.Hash  `json:"uncles"`
	Transactions     []common.Hash  `json:"transactions"`
	EthTransactions  []common.Hash  `json:"transactionsInEthHash"`
	StakingTxs       []common.Hash  `json:"stakingTransactions"`
	Signers          []string       `json:"signers,omitempty"`
}

// BlockHeader represents a block header that will serialize to the RPC representation of a block header
type BlockHeader struct {
	ParentHash           common.Hash    `json:"parentHash"`
	Miner                common.Address `json:"miner"`
	StateRoot            common.Hash    `json:"stateRoot"`
	TransactionsRoot     common.Hash    `json:"transactionsRoot"`
	ReceiptsRoot         common.Hash    `json:"receiptsRoot"`
	OutgoingReceiptsRoot common.Hash    `json:"outgoingReceiptsRoot"`
	IncomingReceiptsRoot common.Hash    `json:"incomingReceiptsRoot"`
	LogsBloom            ethtypes.Bloom `json:"logsBloom"`
	Number               *big.Int       `json:"number"`
	GasLimit             uint64         `json:"gasLimit"`
	GasUsed              uint64         `json:"gasUsed"`
	Timestamp            *big.Int       `json:"timestamp"`
	ExtraData            hexutil.Bytes  `json:"extraData"`
	MixHash              common.Hash    `json:"mixHash"`
	ViewID               *big.Int       `json:"viewID"`
	Epoch                *big.Int       `json:"epoch"`
	ShardID              uint32         `json:"shardID"`
	LastCommitSignature  hexutil.Bytes  `json:"lastCommitSignature"`
	LastCommitBitmap     hexutil.Bytes  `json:"lastCommitBitmap"`
	Vrf                  hexutil.Bytes  `json:"vrf"`
	Vdf                  hexutil.Bytes  `json:"vdf"`
	ShardState           hexutil.Bytes  `json:"shardState"`
	CrossLink            hexutil.Bytes  `json:"crossLink"`
	Slashes              hexutil.Bytes  `json:"slashes"`
}

// BlockWithFullTx represents a block that will serialize to the RPC representation of a block
// having FULL transactions in the Transaction & Staking transaction fields.
type BlockWithFullTx struct {
	Number           *big.Int              `json:"number"`
	ViewID           *big.Int              `json:"viewID"`
	Epoch            *big.Int              `json:"epoch"`
	Hash             common.Hash           `json:"hash"`
	ParentHash       common.Hash           `json:"parentHash"`
	Nonce            uint64                `json:"nonce"`
	MixHash          common.Hash           `json:"mixHash"`
	LogsBloom        ethtypes.Bloom        `json:"logsBloom"`
	StateRoot        common.Hash           `json:"stateRoot"`
	Miner            string                `json:"miner"`
	Difficulty       uint64                `json:"difficulty"`
	ExtraData        hexutil.Bytes         `json:"extraData"`
	Size             uint64                `json:"size"`
	GasLimit         uint64                `json:"gasLimit"`
	GasUsed          uint64                `json:"gasUsed"`
	VRF              common.Hash           `json:"vrf"`
	VRFProof         hexutil.Bytes         `json:"vrfProof"`
	Timestamp        *big.Int              `json:"timestamp"`
	TransactionsRoot common.Hash           `json:"transactionsRoot"`
	ReceiptsRoot     common.Hash           `json:"receiptsRoot"`
	Uncles           []common.Hash         `json:"uncles"`
	Transactions     []*Transaction        `json:"transactions"`
	StakingTxs       []*StakingTransaction `json:"stakingTransactions"`
	Signers          []string              `json:"signers,omitempty"`
}

// Transaction represents a transaction that will serialize to the RPC representation of a transaction
type Transaction struct {
	BlockHash        common.Hash   `json:"blockHash"`
	BlockNumber      *big.Int      `json:"blockNumber"`
	From             string        `json:"from"`
	Timestamp        uint64        `json:"timestamp"`
	Gas              uint64        `json:"gas"`
	GasPrice         *big.Int      `json:"gasPrice"`
	Hash             common.Hash   `json:"hash"`
	EthHash          common.Hash   `json:"ethHash"`
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

// StakingTransaction represents a transaction that will serialize to the RPC representation of a staking transaction
type StakingTransaction struct {
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

// CreateValidatorMsg represents a staking transaction's create validator directive that
// will serialize to the RPC representation
type CreateValidatorMsg struct {
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
	SlotKeySigs        []bls.SerializedSignature `json:"slotKeySigs"`
}

// EditValidatorMsg represents a staking transaction's edit validator directive that
// will serialize to the RPC representation
type EditValidatorMsg struct {
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
	SlotKeyToAddSig    *bls.SerializedSignature `json:"slotKeyToAddSig"`
}

// CollectRewardsMsg represents a staking transaction's collect rewards directive that
// will serialize to the RPC representation
type CollectRewardsMsg struct {
	DelegatorAddress string `json:"delegatorAddress"`
}

// DelegateMsg represents a staking transaction's delegate directive that
// will serialize to the RPC representation
type DelegateMsg struct {
	DelegatorAddress string   `json:"delegatorAddress"`
	ValidatorAddress string   `json:"validatorAddress"`
	Amount           *big.Int `json:"amount"`
}

// UndelegateMsg represents a staking transaction's delegate directive that
// will serialize to the RPC representation
type UndelegateMsg struct {
	DelegatorAddress string   `json:"delegatorAddress"`
	ValidatorAddress string   `json:"validatorAddress"`
	Amount           *big.Int `json:"amount"`
}

// TxReceipt represents a transaction receipt that will serialize to the RPC representation.
type TxReceipt struct {
	BlockHash         common.Hash     `json:"blockHash"`
	TransactionHash   common.Hash     `json:"transactionHash"`
	BlockNumber       uint64          `json:"blockNumber"`
	TransactionIndex  uint64          `json:"transactionIndex"`
	GasUsed           uint64          `json:"gasUsed"`
	CumulativeGasUsed uint64          `json:"cumulativeGasUsed"`
	ContractAddress   *common.Address `json:"contractAddress"`
	Logs              []*types.Log    `json:"logs"`
	LogsBloom         ethtypes.Bloom  `json:"logsBloom"`
	ShardID           uint32          `json:"shardID"`
	From              string          `json:"from"`
	To                string          `json:"to"`
	Root              hexutil.Bytes   `json:"root"`
	Status            uint            `json:"status"`
}

// StakingTxReceipt represents a staking transaction receipt that will serialize to the RPC representation.
type StakingTxReceipt struct {
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
	Root              hexutil.Bytes     `json:"root"`
	Status            uint              `json:"status"`
}

// CxReceipt represents a CxReceipt that will serialize to the RPC representation of a CxReceipt
type CxReceipt struct {
	BlockHash   common.Hash `json:"blockHash"`
	BlockNumber *big.Int    `json:"blockNumber"`
	TxHash      common.Hash `json:"hash"`
	From        string      `json:"from"`
	To          string      `json:"to"`
	ShardID     uint32      `json:"shardID"`
	ToShardID   uint32      `json:"toShardID"`
	Amount      *big.Int    `json:"value"`
}

// NewCxReceipt returns a CxReceipt that will serialize to the RPC representation
func NewCxReceipt(cx *types.CXReceipt, blockHash common.Hash, blockNumber uint64) (*CxReceipt, error) {
	result := &CxReceipt{
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
		Gas:       tx.GasLimit(),
		GasPrice:  tx.GasPrice(),
		Hash:      tx.Hash(),
		EthHash:   tx.ConvertToEth().Hash(),
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

// NewReceipt returns a transaction OR staking transaction that will serialize to the RPC representation
func NewReceipt(
	tx interface{}, blockHash common.Hash, blockNumber, blockIndex uint64, receipt *types.Receipt, eth bool,
) (interface{}, error) {
	plainTx, ok := tx.(*types.Transaction)
	if ok {
		return NewTxReceipt(plainTx, blockHash, blockNumber, blockIndex, receipt, eth)
	}
	stakingTx, ok := tx.(*staking.StakingTransaction)
	if ok {
		return NewStakingTxReceipt(stakingTx, blockHash, blockNumber, blockIndex, receipt)
	}
	return nil, fmt.Errorf("unknown transaction type for RPC receipt")
}

// NewTxReceipt returns a plain transaction receipt that will serialize to the RPC representation
func NewTxReceipt(
	tx *types.Transaction, blockHash common.Hash, blockNumber, blockIndex uint64, receipt *types.Receipt, eth bool,
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
		if eth {
			sender = senderAddr.String()
			receiver = tx.To().String()
		} else {
			sender, err = internal_common.AddressToBech32(senderAddr)
			if err != nil {
				return nil, err
			}
			receiver, err = internal_common.AddressToBech32(*tx.To())
			if err != nil {
				return nil, err
			}
		}
	}

	// Declare receipt
	txReceipt := &TxReceipt{
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
		Root:              receipt.PostState,
		Status:            uint(receipt.Status),
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
		txReceipt.ContractAddress = &receipt.ContractAddress
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
		BlockNumber:       blockNumber,
		TransactionIndex:  blockIndex,
		GasUsed:           receipt.GasUsed,
		CumulativeGasUsed: receipt.CumulativeGasUsed,
		Logs:              receipt.Logs,
		LogsBloom:         receipt.Bloom,
		Sender:            sender,
		Type:              tx.StakingType(),
		Root:              receipt.PostState,
		Status:            uint(receipt.Status),
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
func NewStakingTransaction(
	tx *staking.StakingTransaction, blockHash common.Hash,
	blockNumber uint64, timestamp uint64, index uint64, signed bool,
) (*StakingTransaction, error) {

	v, r, s := tx.RawSignatureValues()

	var rpcMsg interface{}
	switch tx.StakingType() {
	case staking.DirectiveCreateValidator:
		rawMsg, err := staking.RLPDecodeStakeMsg(tx.Data(), staking.DirectiveCreateValidator)
		if err != nil {
			return nil, errors.New(fmt.Sprintf("RLP decode error: %s", err.Error()))
		}
		msg, ok := rawMsg.(*staking.CreateValidator)
		if !ok {
			return nil, fmt.Errorf("could not decode staking message")
		}
		validatorAddress, err := internal_common.AddressToBech32(msg.ValidatorAddress)
		if err != nil {
			return nil, errors.New(fmt.Sprintf("convert validator address error: %s", err.Error()))
		}
		rpcMsg = &CreateValidatorMsg{
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
			SlotKeySigs:        msg.SlotKeySigs,
		}
	case staking.DirectiveEditValidator:
		rawMsg, err := staking.RLPDecodeStakeMsg(tx.Data(), staking.DirectiveEditValidator)
		if err != nil {
			return nil, errors.New(fmt.Sprintf("RLP decode error: %s", err.Error()))
		}
		msg, ok := rawMsg.(*staking.EditValidator)
		if !ok {
			return nil, fmt.Errorf("could not decode staking message")
		}
		validatorAddress, err := internal_common.AddressToBech32(msg.ValidatorAddress)
		if err != nil {
			return nil, errors.New(fmt.Sprintf("convert validator address error: %s", err.Error()))
		}
		// Edit validators txs need not have commission rates to edit
		commissionRate := &big.Int{}
		if msg.CommissionRate != nil {
			commissionRate = msg.CommissionRate.Int
		}
		rpcMsg = &EditValidatorMsg{
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
			SlotKeyToAddSig:    msg.SlotKeyToAddSig,
		}
	case staking.DirectiveCollectRewards:
		rawMsg, err := staking.RLPDecodeStakeMsg(tx.Data(), staking.DirectiveCollectRewards)
		if err != nil {
			return nil, errors.New(fmt.Sprintf("RLP decode error: %s", err.Error()))
		}
		msg, ok := rawMsg.(*staking.CollectRewards)
		if !ok {
			return nil, fmt.Errorf("could not decode staking message")
		}
		delegatorAddress, err := internal_common.AddressToBech32(msg.DelegatorAddress)
		if err != nil {
			return nil, errors.New(fmt.Sprintf("convert delegator address error: %s", err.Error()))
		}
		rpcMsg = &CollectRewardsMsg{DelegatorAddress: delegatorAddress}
	case staking.DirectiveDelegate:
		rawMsg, err := staking.RLPDecodeStakeMsg(tx.Data(), staking.DirectiveDelegate)
		if err != nil {
			return nil, errors.New(fmt.Sprintf("RLP decode error: %s", err.Error()))
		}
		msg, ok := rawMsg.(*staking.Delegate)
		if !ok {
			return nil, fmt.Errorf("could not decode staking message")
		}
		delegatorAddress, err := internal_common.AddressToBech32(msg.DelegatorAddress)
		if err != nil {
			return nil, errors.New(fmt.Sprintf("convert delegator address error: %s", err.Error()))
		}
		validatorAddress, err := internal_common.AddressToBech32(msg.ValidatorAddress)
		if err != nil {
			return nil, errors.New(fmt.Sprintf("convert validator address error: %s", err.Error()))
		}
		rpcMsg = &DelegateMsg{
			DelegatorAddress: delegatorAddress,
			ValidatorAddress: validatorAddress,
			Amount:           msg.Amount,
		}
	case staking.DirectiveUndelegate:
		rawMsg, err := staking.RLPDecodeStakeMsg(tx.Data(), staking.DirectiveUndelegate)
		if err != nil {
			return nil, errors.New(fmt.Sprintf("RLP decode error: %s", err.Error()))
		}
		msg, ok := rawMsg.(*staking.Undelegate)
		if !ok {
			return nil, fmt.Errorf("could not decode staking message")
		}
		delegatorAddress, err := internal_common.AddressToBech32(msg.DelegatorAddress)
		if err != nil {
			return nil, errors.New(fmt.Sprintf("convert delegator address error: %s", err.Error()))
		}
		validatorAddress, err := internal_common.AddressToBech32(msg.ValidatorAddress)
		if err != nil {
			return nil, err
		}
		rpcMsg = &UndelegateMsg{
			DelegatorAddress: delegatorAddress,
			ValidatorAddress: validatorAddress,
			Amount:           msg.Amount,
		}
	}

	result := &StakingTransaction{
		Gas:       tx.GasLimit(),
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

	if signed {
		from, err := tx.SenderAddress()
		if err != nil {
			return nil, errors.New(fmt.Sprintf("get sender address error: %s", err.Error()))
		}

		fromAddr, err := internal_common.AddressToBech32(from)
		if err != nil {
			return nil, err
		}
		result.From = fromAddr
	}

	return result, nil
}

// blockWithTxHashFromBlock return a block with only the transaction hash that will serialize to the RPC representation
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
		Number:           head.Number(),
		ViewID:           head.ViewID(),
		Epoch:            head.Epoch(),
		Hash:             b.Hash(),
		ParentHash:       head.ParentHash(),
		Nonce:            0, // Remove this because we don't have it in our header
		MixHash:          head.MixDigest(),
		LogsBloom:        head.Bloom(),
		StateRoot:        head.Root(),
		Difficulty:       0, // Remove this because we don't have it in our header
		ExtraData:        hexutil.Bytes(head.Extra()),
		Size:             uint64(b.Size()),
		GasLimit:         head.GasLimit(),
		GasUsed:          head.GasUsed(),
		VRF:              vrf,
		VRFProof:         vrfProof,
		Timestamp:        head.Time(),
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

// NewBlockWithFullTx return a block with the transaction that will serialize to the RPC representation
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
		Number:           head.Number(),
		ViewID:           head.ViewID(),
		Epoch:            head.Epoch(),
		Hash:             b.Hash(),
		ParentHash:       head.ParentHash(),
		Nonce:            0, // Remove this because we don't have it in our header
		MixHash:          head.MixDigest(),
		LogsBloom:        head.Bloom(),
		StateRoot:        head.Root(),
		Difficulty:       0, // Remove this because we don't have it in our header
		ExtraData:        hexutil.Bytes(head.Extra()),
		Size:             uint64(b.Size()),
		GasLimit:         head.GasLimit(),
		GasUsed:          head.GasUsed(),
		VRF:              vrf,
		VRFProof:         vrfProof,
		Timestamp:        head.Time(),
		TransactionsRoot: head.TxHash(),
		ReceiptsRoot:     head.ReceiptHash(),
		Uncles:           []common.Hash{},
		Transactions:     []*Transaction{},
		StakingTxs:       []*StakingTransaction{},
	}

	for _, tx := range b.Transactions() {
		fmtTx, err := NewTransactionFromHash(b, tx.Hash())
		if err != nil {
			return nil, err
		}
		blk.Transactions = append(blk.Transactions, fmtTx)
	}
	return blk, nil
}

func NewBlockHeader(
	head *block.Header,
) (*BlockHeader, error) {
	lastSig := head.LastCommitSignature()
	blk := &BlockHeader{
		ParentHash:           head.ParentHash(),
		Miner:                head.Coinbase(),
		StateRoot:            head.Root(),
		TransactionsRoot:     head.TxHash(),
		ReceiptsRoot:         head.ReceiptHash(),
		OutgoingReceiptsRoot: head.OutgoingReceiptHash(),
		IncomingReceiptsRoot: head.IncomingReceiptHash(),
		LogsBloom:            head.Bloom(),

		Number:    head.Number(),
		GasLimit:  head.GasLimit(),
		GasUsed:   head.GasUsed(),
		Timestamp: head.Time(),
		ExtraData: hexutil.Bytes(head.Extra()),
		MixHash:   head.MixDigest(),

		ViewID:  head.ViewID(),
		Epoch:   head.Epoch(),
		ShardID: head.ShardID(),

		LastCommitSignature: hexutil.Bytes(lastSig[:]),
		LastCommitBitmap:    head.LastCommitBitmap(),
		Vrf:                 head.Vrf(),
		Vdf:                 head.Vdf(),
		ShardState:          head.ShardState(),
		CrossLink:           head.CrossLinks(),
		Slashes:             head.Slashes(),
	}
	return blk, nil
}

// NewTransactionFromHash returns a transaction that will serialize to the RPC representation.
func NewTransactionFromHash(b *types.Block, hash common.Hash) (*Transaction, error) {
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
		rpcStk, err := NewStakingTransaction(raw, b.Hash(), b.NumberU64(), b.Time().Uint64(), uint64(idx), true)
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
	return NewStakingTransaction(txs[index], b.Hash(), b.NumberU64(), b.Time().Uint64(), index, true)
}
