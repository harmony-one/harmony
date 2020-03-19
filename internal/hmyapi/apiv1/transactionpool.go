package apiv1

import (
	"context"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/harmony-one/harmony/accounts"
	"github.com/harmony-one/harmony/block"
	"github.com/harmony-one/harmony/core/rawdb"
	"github.com/harmony-one/harmony/core/types"
	internal_common "github.com/harmony-one/harmony/internal/common"
	staking "github.com/harmony-one/harmony/staking/types"
	"github.com/pkg/errors"
)

var (
	// ErrInvalidChainID when ChainID of signer does not match that of running node
	errInvalidChainID = errors.New("invalid chain id for signer")
)

// TxHistoryArgs is struct to make GetTransactionsHistory request
type TxHistoryArgs struct {
	Address   string `json:"address"`
	PageIndex uint32 `json:"pageIndex"`
	PageSize  uint32 `json:"pageSize"`
	FullTx    bool   `json:"fullTx"`
	TxType    string `json:"txType"`
	Order     string `json:"order"`
}

// PublicTransactionPoolAPI exposes methods for the RPC interface
type PublicTransactionPoolAPI struct {
	b         Backend
	nonceLock *AddrLocker
}

// NewPublicTransactionPoolAPI creates a new RPC service with methods specific for the transaction pool.
func NewPublicTransactionPoolAPI(b Backend, nonceLock *AddrLocker) *PublicTransactionPoolAPI {
	return &PublicTransactionPoolAPI{b, nonceLock}
}

// GetTransactionsHistory returns the list of transactions hashes that involve a particular address.
func (s *PublicTransactionPoolAPI) GetTransactionsHistory(ctx context.Context, args TxHistoryArgs) (map[string]interface{}, error) {
	address := args.Address
	var err error
	if strings.HasPrefix(args.Address, "one1") {
		address = args.Address
	} else {
		addr := internal_common.ParseAddr(args.Address)
		address, err = internal_common.AddressToBech32(addr)
		if err != nil {
			return nil, err
		}
	}
	hashes, err := s.b.GetTransactionsHistory(address, args.TxType, args.Order)
	if err != nil {
		return nil, err
	}
	length, args := ReturnPagination(len(hashes), args)
	result := hashes[args.PageSize*args.PageIndex : length]
	if !args.FullTx {
		return map[string]interface{}{"transactions": result}, nil
	}
	txs := []*RPCTransaction{}
	for _, hash := range result {
		tx := s.GetTransactionByHash(ctx, hash)
		if tx == nil {
			continue
		}
		txs = append(txs, tx)
	}
	return map[string]interface{}{"transactions": txs}, nil
}

// GetCrossShardTransactionsHistory returns the list of cross shard transactions that involve a particular address.
func (s *PublicTransactionPoolAPI) GetCrossShardTransactionsHistory(ctx context.Context, args TxHistoryArgs) (map[string]interface{}, error) {
	address := args.Address
	var err error
	if strings.HasPrefix(args.Address, "one1") {
		address = args.Address
	} else {
		addr := internal_common.ParseAddr(args.Address)
		address, err = internal_common.AddressToBech32(addr)
		if err != nil {
			return nil, err
		}
	}
	crossTxs, err := s.b.GetCrossShardTransactionsHistory(address)
	if err != nil {
		return nil, err
	}
	length, args := ReturnPagination(len(crossTxs), args)
	result := crossTxs[args.PageSize*args.PageIndex : length]
	return map[string]interface{}{"transactions": result}, nil
}

// GetBlockTransactionCountByNumber returns the number of transactions in the block with the given block number.
func (s *PublicTransactionPoolAPI) GetBlockTransactionCountByNumber(ctx context.Context, blockNr rpc.BlockNumber) *hexutil.Uint {
	if block, _ := s.b.BlockByNumber(ctx, blockNr); block != nil {
		n := hexutil.Uint(len(block.Transactions()))
		return &n
	}
	return nil
}

// GetBlockTransactionCountByHash returns the number of transactions in the block with the given hash.
func (s *PublicTransactionPoolAPI) GetBlockTransactionCountByHash(ctx context.Context, blockHash common.Hash) *hexutil.Uint {
	if block, _ := s.b.GetBlock(ctx, blockHash); block != nil {
		n := hexutil.Uint(len(block.Transactions()))
		return &n
	}
	return nil
}

// GetTransactionByBlockNumberAndIndex returns the transaction for the given block number and index.
func (s *PublicTransactionPoolAPI) GetTransactionByBlockNumberAndIndex(ctx context.Context, blockNr rpc.BlockNumber, index hexutil.Uint) *RPCTransaction {
	if block, _ := s.b.BlockByNumber(ctx, blockNr); block != nil {
		return newRPCTransactionFromBlockIndex(block, uint64(index))
	}
	return nil
}

// GetTransactionByBlockHashAndIndex returns the transaction for the given block hash and index.
func (s *PublicTransactionPoolAPI) GetTransactionByBlockHashAndIndex(ctx context.Context, blockHash common.Hash, index hexutil.Uint) *RPCTransaction {
	if block, _ := s.b.GetBlock(ctx, blockHash); block != nil {
		return newRPCTransactionFromBlockIndex(block, uint64(index))
	}
	return nil
}

// GetTransactionByHash returns the transaction for the given hash
func (s *PublicTransactionPoolAPI) GetTransactionByHash(ctx context.Context, hash common.Hash) *RPCTransaction {
	// Try to return an already finalized transaction
	tx, blockHash, blockNumber, index := rawdb.ReadTransaction(s.b.ChainDb(), hash)
	block, _ := s.b.GetBlock(ctx, blockHash)
	if block == nil {
		return nil
	}
	if tx != nil {
		return newRPCTransaction(tx, blockHash, blockNumber, block.Time().Uint64(), index)
	}
	// No finalized transaction, try to retrieve it from the pool
	if tx = s.b.GetPoolTransaction(hash); tx != nil {
		return newRPCPendingTransaction(tx)
	}
	// Transaction unknown, return as such
	return nil
}

// GetTransactionCount returns the number of transactions the given address has sent for the given block number
func (s *PublicTransactionPoolAPI) GetTransactionCount(ctx context.Context, addr string, blockNr rpc.BlockNumber) (*hexutil.Uint64, error) {
	address := internal_common.ParseAddr(addr)
	// Ask transaction pool for the nonce which includes pending transactions
	if blockNr == rpc.PendingBlockNumber {
		nonce, err := s.b.GetPoolNonce(ctx, address)
		if err != nil {
			return nil, err
		}
		return (*hexutil.Uint64)(&nonce), nil
	}
	// Resolve block number and use its state to ask for the nonce
	state, _, err := s.b.StateAndHeaderByNumber(ctx, blockNr)
	if state == nil || err != nil {
		return nil, err
	}
	nonce := state.GetNonce(address)
	return (*hexutil.Uint64)(&nonce), state.Error()
}

// SendTransaction creates a transaction for the given argument, sign it and submit it to the
// transaction pool.
func (s *PublicTransactionPoolAPI) SendTransaction(ctx context.Context, args SendTxArgs) (common.Hash, error) {
	// Look up the wallet containing the requested signer
	account := accounts.Account{Address: args.From}

	wallet, err := s.b.AccountManager().Find(account)
	if err != nil {
		return common.Hash{}, err
	}

	if args.Nonce == nil {
		// Hold the addresse's mutex around signing to prevent concurrent assignment of
		// the same nonce to multiple accounts.
		s.nonceLock.LockAddr(args.From)
		defer s.nonceLock.UnlockAddr(args.From)
	}

	// Set some sanity defaults and terminate on failure
	if err := args.setDefaults(ctx, s.b); err != nil {
		return common.Hash{}, err
	}
	// Assemble the transaction and sign with the wallet
	tx := args.toTransaction()

	signed, err := wallet.SignTx(account, tx, s.b.ChainConfig().ChainID)
	if err != nil {
		return common.Hash{}, err
	}
	return SubmitTransaction(ctx, s.b, signed)
}

// SendRawStakingTransaction will add the signed transaction to the transaction pool.
// The sender is responsible for signing the transaction and using the correct nonce.
func (s *PublicTransactionPoolAPI) SendRawStakingTransaction(
	ctx context.Context, encodedTx hexutil.Bytes,
) (common.Hash, error) {
	tx := new(staking.StakingTransaction)
	if err := rlp.DecodeBytes(encodedTx, tx); err != nil {
		return common.Hash{}, err
	}
	c := s.b.ChainConfig().ChainID
	if tx.ChainID().Cmp(c) != 0 {
		e := errors.Wrapf(errInvalidChainID, "current chain id:%s", c.String())
		return common.Hash{}, e
	}
	return SubmitStakingTransaction(ctx, s.b, tx)
}

// SendRawTransaction will add the signed transaction to the transaction pool.
// The sender is responsible for signing the transaction and using the correct nonce.
func (s *PublicTransactionPoolAPI) SendRawTransaction(ctx context.Context, encodedTx hexutil.Bytes) (common.Hash, error) {
	tx := new(types.Transaction)
	if err := rlp.DecodeBytes(encodedTx, tx); err != nil {
		return common.Hash{}, err
	}
	c := s.b.ChainConfig().ChainID
	if tx.ChainID().Cmp(c) != 0 {
		e := errors.Wrapf(errInvalidChainID, "current chain id:%s", c.String())
		return common.Hash{}, e
	}
	return SubmitTransaction(ctx, s.b, tx)
}

// GetTransactionReceipt returns the transaction receipt for the given transaction hash.
func (s *PublicTransactionPoolAPI) GetTransactionReceipt(ctx context.Context, hash common.Hash) (map[string]interface{}, error) {
	tx, blockHash, blockNumber, index := rawdb.ReadTransaction(s.b.ChainDb(), hash)
	if tx == nil {
		return nil, nil
	}
	receipts, err := s.b.GetReceipts(ctx, blockHash)
	if err != nil {
		return nil, err
	}
	if len(receipts) <= int(index) {
		return nil, nil
	}
	receipt := receipts[index]

	var signer types.Signer = types.FrontierSigner{}
	if tx.Protected() {
		signer = types.NewEIP155Signer(tx.ChainID())
	}
	fields := map[string]interface{}{
		"blockHash":         blockHash,
		"blockNumber":       hexutil.Uint64(blockNumber),
		"transactionHash":   hash,
		"transactionIndex":  hexutil.Uint64(index),
		"shardID":           tx.ShardID(),
		"gasUsed":           hexutil.Uint64(receipt.GasUsed),
		"cumulativeGasUsed": hexutil.Uint64(receipt.CumulativeGasUsed),
		"contractAddress":   nil,
		"logs":              receipt.Logs,
		"logsBloom":         receipt.Bloom,
	}
	from, _ := types.Sender(signer, tx)
	fields["from"] = from
	fields["to"] = ""
	if tx.To() != nil {
		fields["to"], err = internal_common.AddressToBech32(*tx.To())
		if err != nil {
			return nil, err
		}
		fields["from"], err = internal_common.AddressToBech32(from)
		if err != nil {
			return nil, err
		}
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

// PendingTransactions returns the transactions that are in the transaction pool
func (s *PublicTransactionPoolAPI) PendingTransactions() ([]*RPCTransaction, error) {
	pending, err := s.b.GetPoolTransactions()
	if err != nil {
		return nil, err
	}
	transactions := make([]*RPCTransaction, len(pending))
	for i := range pending {
		transactions[i] = newRPCPendingTransaction(pending[i])
	}
	return transactions, nil
}

// GetCurrentTransactionErrorSink ..
func (s *PublicTransactionPoolAPI) GetCurrentTransactionErrorSink() []types.RPCTransactionError {
	return s.b.GetCurrentTransactionErrorSink()
}

// GetCXReceiptByHash returns the transaction for the given hash
func (s *PublicTransactionPoolAPI) GetCXReceiptByHash(ctx context.Context, hash common.Hash) *RPCCXReceipt {
	if cx, blockHash, blockNumber, _ := rawdb.ReadCXReceipt(s.b.ChainDb(), hash); cx != nil {
		return newRPCCXReceipt(cx, blockHash, blockNumber)
	}
	return nil
}

// GetPendingCrossLinks ..
func (s *PublicTransactionPoolAPI) GetPendingCrossLinks(ctx context.Context) []*block.Header {
	crossLinks := s.b.GetPendingCrossLinks()
	if crossLinks == nil {
		return make([]*block.Header, 0)
	}
	return crossLinks
}

// GetPendingCXReceipts ..
func (s *PublicTransactionPoolAPI) GetPendingCXReceipts(ctx context.Context) []*types.CXReceiptsProof {
	return s.b.GetPendingCXReceipts()
}
