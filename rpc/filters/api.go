package filters

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/harmony-one/harmony/internal/utils"

	"github.com/harmony-one/harmony/internal/tikv/redis_helper"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/block"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/eth/rpc"
	hmy_rpc "github.com/harmony-one/harmony/rpc"
)

var (
	deadline = 5 * time.Minute // consider a filter inactive if it has not been polled for within deadline
)

const (
	rpcGetLogsLimit = 1024
)

// filter is a helper struct that holds meta information over the filter type
// and associated subscription in the event system.
type filter struct {
	typ      Type
	deadline *time.Timer // filter is inactive when deadline triggers
	hashes   []common.Hash
	crit     FilterCriteria
	logs     []*types.Log
	s        *Subscription // associated subscription in event system
}

// PublicFilterAPI offers support to create and manage filters. This will allow external clients to retrieve various
// information related to the Ethereum protocol such als blocks, transactions and logs.
type PublicFilterAPI struct {
	backend   Backend
	quit      chan struct{}
	events    *EventSystem
	filtersMu sync.Mutex
	filters   map[rpc.ID]*filter
	namespace string
	shardID   uint32
}

// NewPublicFilterAPI returns a new PublicFilterAPI instance.
func NewPublicFilterAPI(backend Backend, lightMode bool, namespace string, shardID uint32) rpc.API {
	api := &PublicFilterAPI{
		backend:   backend,
		events:    NewEventSystem(backend, lightMode, namespace == "eth"),
		filters:   make(map[rpc.ID]*filter),
		namespace: namespace,
		shardID:   shardID,
	}
	go api.timeoutLoop()

	return rpc.API{
		Namespace: namespace,
		Version:   hmy_rpc.APIVersion,
		Service:   api,
		Public:    true,
	}
}

func (api *PublicFilterAPI) isEth() bool {
	return api.namespace == "eth"
}

// timeoutLoop runs every 5 minutes and deletes filters that have not been recently used.
// Tt is started when the api is created.
func (api *PublicFilterAPI) timeoutLoop() {
	// TODO ek – infinite loop; add shutdown/cleanup logic
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		<-ticker.C
		api.filtersMu.Lock()
		for id, f := range api.filters {
			select {
			case <-f.deadline.C:
				f.s.Unsubscribe()
				delete(api.filters, id)
			default:
				continue
			}
		}
		api.filtersMu.Unlock()
	}
}

// NewPendingTransactionFilter creates a filter that fetches pending transaction hashes
// as transactions enter the pending state.
//
// It is part of the filter package because this filter can be used through the
// `eth_getFilterChanges` polling method that is also used for log filters.
//
// https://github.com/ethereum/wiki/wiki/JSON-RPC#eth_newpendingtransactionfilter
func (api *PublicFilterAPI) NewPendingTransactionFilter() rpc.ID {
	timer := hmy_rpc.DoMetricRPCRequest(hmy_rpc.NewPendingTransactionFilter)
	defer hmy_rpc.DoRPCRequestDuration(hmy_rpc.NewPendingTransactionFilter, timer)

	var (
		pendingTxs   = make(chan []common.Hash)
		pendingTxSub = api.events.SubscribePendingTxs(pendingTxs)
	)

	api.filtersMu.Lock()
	api.filters[pendingTxSub.ID] = &filter{typ: PendingTransactionsSubscription, deadline: time.NewTimer(deadline), hashes: make([]common.Hash, 0), s: pendingTxSub}
	api.filtersMu.Unlock()

	go func() {
		for {
			select {
			case ph := <-pendingTxs:
				api.filtersMu.Lock()
				if f, found := api.filters[pendingTxSub.ID]; found {
					f.hashes = append(f.hashes, ph...)
				}
				api.filtersMu.Unlock()
			case <-pendingTxSub.Err():
				api.filtersMu.Lock()
				delete(api.filters, pendingTxSub.ID)
				api.filtersMu.Unlock()
				return
			}
		}
	}()

	return pendingTxSub.ID
}

// NewPendingTransactions creates a subscription that is triggered each time a transaction
// enters the transaction pool and was signed from one of the transactions this nodes manages.
func (api *PublicFilterAPI) NewPendingTransactions(ctx context.Context) (*rpc.Subscription, error) {
	timer := hmy_rpc.DoMetricRPCRequest(hmy_rpc.NewPendingTransactions)
	defer hmy_rpc.DoRPCRequestDuration(hmy_rpc.NewPendingTransactions, timer)

	notifier, supported := rpc.NotifierFromContext(ctx)
	if !supported {
		return &rpc.Subscription{}, rpc.ErrNotificationsUnsupported
	}

	rpcSub := notifier.CreateSubscription()

	go func() {
		txHashes := make(chan []common.Hash, 128)
		pendingTxSub := api.events.SubscribePendingTxs(txHashes)

		for {
			select {
			case hashes := <-txHashes:
				// To keep the original behaviour, send a single tx hash in one notification.
				// TODO(dm) Send a batch of tx hashes in one notification
				for _, h := range hashes {
					_ = notifier.Notify(rpcSub.ID, h)
				}
			case <-rpcSub.Err():
				pendingTxSub.Unsubscribe()
				return
			case <-notifier.Closed():
				pendingTxSub.Unsubscribe()
				return
			}
		}
	}()

	return rpcSub, nil
}

// NewBlockFilter creates a filter that fetches blocks that are imported into the chain.
// It is part of the filter package since polling goes with eth_getFilterChanges.
//
// https://github.com/ethereum/wiki/wiki/JSON-RPC#eth_newblockfilter
func (api *PublicFilterAPI) NewBlockFilter() rpc.ID {
	timer := hmy_rpc.DoMetricRPCRequest(hmy_rpc.NewBlockFilter)
	defer hmy_rpc.DoRPCRequestDuration(hmy_rpc.NewBlockFilter, timer)

	var (
		headers   = make(chan *block.Header)
		headerSub = api.events.SubscribeNewHeads(headers)
	)

	api.filtersMu.Lock()
	api.filters[headerSub.ID] = &filter{typ: BlocksSubscription, deadline: time.NewTimer(deadline), hashes: make([]common.Hash, 0), s: headerSub}
	api.filtersMu.Unlock()

	go func() {
		for {
			select {
			case h := <-headers:
				api.filtersMu.Lock()
				if f, found := api.filters[headerSub.ID]; found {
					f.hashes = append(f.hashes, h.Hash())
				}
				api.filtersMu.Unlock()
			case <-headerSub.Err():
				api.filtersMu.Lock()
				delete(api.filters, headerSub.ID)
				api.filtersMu.Unlock()
				return
			}
		}
	}()

	return headerSub.ID
}

// NewHeads send a notification each time a new (header) block is appended to the chain.
func (api *PublicFilterAPI) NewHeads(ctx context.Context) (*rpc.Subscription, error) {
	timer := hmy_rpc.DoMetricRPCRequest(hmy_rpc.NewHeads)
	defer hmy_rpc.DoRPCRequestDuration(hmy_rpc.NewHeads, timer)
	notifier, supported := rpc.NotifierFromContext(ctx)
	if !supported {
		return &rpc.Subscription{}, rpc.ErrNotificationsUnsupported
	}

	rpcSub := notifier.CreateSubscription()

	go func() {
		headers := make(chan *block.Header)
		headersSub := api.events.SubscribeNewHeads(headers)

		for {
			select {
			case h := <-headers:
				_ = notifier.Notify(rpcSub.ID, h)
			case <-rpcSub.Err():
				headersSub.Unsubscribe()
				return
			case <-notifier.Closed():
				headersSub.Unsubscribe()
				return
			}
		}
	}()

	return rpcSub, nil
}

// GetFilterChanges returns the logs for the filter with the given id since
// last time it was called. This can be used for polling.
//
// For pending transaction and block filters the result is []common.Hash.
// (pending)Log filters return []Log.
//
// https://github.com/ethereum/wiki/wiki/JSON-RPC#eth_getfilterchanges
func (api *PublicFilterAPI) GetFilterChanges(id rpc.ID) (interface{}, error) {
	timer := hmy_rpc.DoMetricRPCRequest(hmy_rpc.GetFilterChanges)
	defer hmy_rpc.DoRPCRequestDuration(hmy_rpc.GetFilterChanges, timer)

	api.filtersMu.Lock()
	defer api.filtersMu.Unlock()

	if f, found := api.filters[id]; found {
		if !f.deadline.Stop() {
			// timer expired but filter is not yet removed in timeout loop
			// receive timer value and reset timer
			<-f.deadline.C
		}
		f.deadline.Reset(deadline)

		switch f.typ {
		case PendingTransactionsSubscription, BlocksSubscription:
			hashes := f.hashes
			f.hashes = nil
			return returnHashes(hashes), nil
		case LogsSubscription:
			logs := f.logs
			f.logs = nil
			return returnLogs(logs), nil
		}
	}

	return []interface{}{}, fmt.Errorf("filter not found")
}

// returnHashes is a helper that will return an empty hash array case the given hash array is nil,
// otherwise the given hashes array is returned.
func returnHashes(hashes []common.Hash) []common.Hash {
	if hashes == nil {
		return []common.Hash{}
	}
	return hashes
}

// returnLogs is a helper that will return an empty log array in case the given logs array is nil,
// otherwise the given logs array is returned.
func returnLogs(logs []*types.Log) []*types.Log {
	if logs == nil {
		return []*types.Log{}
	}
	return logs
}

// Logs creates a subscription that fires for all new log that match the given filter criteria.
func (api *PublicFilterAPI) Logs(ctx context.Context, crit FilterCriteria) (*rpc.Subscription, error) {
	timer := hmy_rpc.DoMetricRPCRequest(hmy_rpc.Logs)
	defer hmy_rpc.DoRPCRequestDuration(hmy_rpc.Logs, timer)
	notifier, supported := rpc.NotifierFromContext(ctx)
	if !supported {
		return &rpc.Subscription{}, rpc.ErrNotificationsUnsupported
	}

	var (
		rpcSub      = notifier.CreateSubscription()
		matchedLogs = make(chan []*types.Log)
	)

	logsSub, err := api.events.SubscribeLogs(ethereum.FilterQuery(crit), matchedLogs, nil)
	if err != nil {
		return nil, err
	}

	go func() {

		for {
			select {
			case logs := <-matchedLogs:
				for _, log := range logs {
					_ = notifier.Notify(rpcSub.ID, &log)
				}
			case <-rpcSub.Err(): // client send an unsubscribe request
				logsSub.Unsubscribe()
				return
			case <-notifier.Closed(): // connection dropped
				logsSub.Unsubscribe()
				return
			}
		}
	}()

	return rpcSub, nil
}

// NewFilter creates a new filter and returns the filter id. It can be
// used to retrieve logs when the state changes. This method cannot be
// used to fetch logs that are already stored in the state.
//
// Default criteria for the from and to block are "latest".
// Using "latest" as block number will return logs for mined blocks.
// Using "pending" as block number returns logs for not yet mined (pending) blocks.
// In case logs are removed (chain reorg) previously returned logs are returned
// again but with the removed property set to true.
//
// In case "fromBlock" > "toBlock" an error is returned.
//
// https://github.com/ethereum/wiki/wiki/JSON-RPC#eth_newfilter
func (api *PublicFilterAPI) NewFilter(crit FilterCriteria) (rpc.ID, error) {
	timer := hmy_rpc.DoMetricRPCRequest(hmy_rpc.NewFilter)
	defer hmy_rpc.DoRPCRequestDuration(hmy_rpc.NewFilter, timer)

	id := rpc.NewID()
	if err := redis_helper.PublishNewFilterLogEvent(api.shardID, api.namespace, string(id), ethereum.FilterQuery(crit)); err != nil {
		return "", fmt.Errorf("sending filter logs to other readers: %w", err)
	}

	time.Sleep(time.Millisecond * 70) // to give time to the other readers to catch up
	return api.createFilter(ethereum.FilterQuery(crit), id)
}

func (api *PublicFilterAPI) createFilter(crit ethereum.FilterQuery, customId rpc.ID) (rpc.ID, error) {
	logs := make(chan []*types.Log)
	logsSub, err := api.events.SubscribeLogs(crit, logs, &customId)
	if err != nil {
		return "", err
	}

	api.filtersMu.Lock()
	api.filters[logsSub.ID] = &filter{
		typ:      LogsSubscription,
		crit:     FilterCriteria(crit),
		deadline: time.NewTimer(deadline),
		logs:     make([]*types.Log, 0),
		s:        logsSub,
	}
	api.filtersMu.Unlock()

	go func() {
		for {
			select {
			case l := <-logs:
				api.filtersMu.Lock()
				if f, found := api.filters[logsSub.ID]; found {
					f.logs = append(f.logs, l...)
				}
				api.filtersMu.Unlock()
			case <-logsSub.Err():
				api.filtersMu.Lock()
				delete(api.filters, logsSub.ID)
				api.filtersMu.Unlock()
				return
			}
		}
	}()

	return logsSub.ID, nil
}

func (api *PublicFilterAPI) SyncNewFilterFromOtherReaders() {
	go redis_helper.SubscribeNewFilterLogEvent(api.shardID, api.namespace, func(id string, crit ethereum.FilterQuery) {
		if _, err := api.createFilter(crit, rpc.ID(id)); err != nil {
			utils.Logger().Warn().
				Uint32("shardID", api.shardID).
				Err(err).
				Msg("unable to sync filters from other readers")
		}
	})
}

// GetLogs returns logs matching the given argument that are stored within the state.
//
// https://github.com/ethereum/wiki/wiki/JSON-RPC#eth_getlogs
func (api *PublicFilterAPI) GetLogs(ctx context.Context, crit FilterCriteria) ([]*types.Log, error) {
	timer := hmy_rpc.DoMetricRPCRequest(hmy_rpc.GetLogs)
	defer hmy_rpc.DoRPCRequestDuration(hmy_rpc.GetLogs, timer)

	var filter *Filter
	if crit.BlockHash != nil {
		// Block filter requested, construct a single-shot filter
		filter = NewBlockFilter(api.backend, *crit.BlockHash, crit.Addresses, crit.Topics, api.isEth())
	} else {
		// Convert the RPC block numbers into internal representations
		begin := rpc.LatestBlockNumber.Int64()
		if crit.FromBlock != nil {
			begin = crit.FromBlock.Int64()
		}
		end := rpc.LatestBlockNumber.Int64()
		if crit.ToBlock != nil {
			end = crit.ToBlock.Int64()
		}
		if end >= begin && end-begin > rpcGetLogsLimit {
			return nil, fmt.Errorf("GetLogs query must be smaller than size %v", rpcGetLogsLimit)
		}

		// Construct the range filter
		filter = NewRangeFilter(api.backend, begin, end, crit.Addresses, crit.Topics, api.isEth())
	}
	// Run the filter and return all the logs
	logs, err := filter.Logs(ctx)
	if err != nil {
		hmy_rpc.DoMetricRPCQueryInfo(hmy_rpc.GetLogs, hmy_rpc.FailedNumber)
		return nil, err
	}
	return returnLogs(logs), err
}

// UninstallFilter removes the filter with the given filter id.
//
// https://github.com/ethereum/wiki/wiki/JSON-RPC#eth_uninstallfilter
func (api *PublicFilterAPI) UninstallFilter(id rpc.ID) bool {
	timer := hmy_rpc.DoMetricRPCRequest(hmy_rpc.UninstallFilter)
	defer hmy_rpc.DoRPCRequestDuration(hmy_rpc.UninstallFilter, timer)

	api.filtersMu.Lock()
	f, found := api.filters[id]
	if found {
		delete(api.filters, id)
	}
	api.filtersMu.Unlock()
	if found {
		f.s.Unsubscribe()
	}

	return found
}

// GetFilterLogs returns the logs for the filter with the given id.
// If the filter could not be found an empty array of logs is returned.
//
// https://github.com/ethereum/wiki/wiki/JSON-RPC#eth_getfilterlogs
func (api *PublicFilterAPI) GetFilterLogs(ctx context.Context, id rpc.ID) ([]*types.Log, error) {
	timer := hmy_rpc.DoMetricRPCRequest(hmy_rpc.GetFilterLogs)
	defer hmy_rpc.DoRPCRequestDuration(hmy_rpc.GetFilterLogs, timer)

	api.filtersMu.Lock()
	f, found := api.filters[id]
	api.filtersMu.Unlock()

	if !found || f.typ != LogsSubscription {
		return nil, fmt.Errorf("filter not found")
	}

	var filter *Filter
	if f.crit.BlockHash != nil {
		// Block filter requested, construct a single-shot filter
		filter = NewBlockFilter(api.backend, *f.crit.BlockHash, f.crit.Addresses, f.crit.Topics, api.isEth())
	} else {
		// Convert the RPC block numbers into internal representations
		begin := rpc.LatestBlockNumber.Int64()
		if f.crit.FromBlock != nil {
			begin = f.crit.FromBlock.Int64()
		}
		end := rpc.LatestBlockNumber.Int64()
		if f.crit.ToBlock != nil {
			end = f.crit.ToBlock.Int64()
		}
		// Construct the range filter
		filter = NewRangeFilter(api.backend, begin, end, f.crit.Addresses, f.crit.Topics, api.isEth())
	}
	// Run the filter and return all the logs
	logs, err := filter.Logs(ctx)
	if err != nil {
		hmy_rpc.DoMetricRPCQueryInfo(hmy_rpc.GetFilterLogs, hmy_rpc.FailedNumber)
		return nil, err
	}
	return returnLogs(logs), nil
}
