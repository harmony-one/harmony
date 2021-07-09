package rpc

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/harmony-one/harmony/hmy"
)

var (
	parityTraceGO = "ParityBlockTracer"
)

type PublicParityTracerService struct {
	*PublicTracerService
}

func (s *PublicParityTracerService) Transaction(ctx context.Context, hash common.Hash) (interface{}, error) {
	return s.TraceTransaction(ctx, hash, &hmy.TraceConfig{Tracer: &parityTraceGO})
}

// trace_block RPC
func (s *PublicParityTracerService) Block(ctx context.Context, number rpc.BlockNumber) (interface{}, error) {
	block := s.hmy.BlockChain.GetBlockByNumber(uint64(number))
	if block == nil {
		return nil, nil
	}
	if results, err := s.hmy.NodeAPI.GetTraceResultByHash(block.Hash()); err == nil {
		return results, nil
	}
	results, err := s.hmy.TraceBlock(ctx, block, &hmy.TraceConfig{Tracer: &parityTraceGO})
	if err != nil {
		return results, err
	}
	var resultArray = make([]json.RawMessage, 0)
	for _, result := range results {
		raw, ok := result.Result.([]json.RawMessage)
		if !ok {
			return results, errors.New("tracer bug:expected []json.RawMessage")
		}
		resultArray = append(resultArray, raw...)
	}
	return resultArray, nil
}

type FilterOption struct {
	FromBlock   rpc.BlockNumber  // Quantity or Tag - (optional) From this block.
	ToBlock     rpc.BlockNumber  // Quantity or Tag - (optional) To this block.
	FromAddress []common.Address //  Array - (optional) Sent from these addresses.
	ToAddress   []common.Address // Address - (optional) Sent to these addresses.
	After       uint             // Quantity - (optional) The offset trace number
	Count       uint             // Quantity - (optional) Integer number of traces to display in a batch.
}

// trace_filter RPC
func (s *PublicParityTracerService) Filter(ctx context.Context, filter FilterOption) (interface{}, error) {
	var results []json.RawMessage
	fromBlock := filter.FromBlock
	toBlock := filter.ToBlock
	if toBlock == 0 {
		toBlock = rpc.BlockNumber(s.hmy.CurrentBlock().NumberU64())
	}
	fromAddresses := make(map[common.Address]bool, len(filter.FromAddress))
	toAddresses := make(map[common.Address]bool, len(filter.ToAddress))
	for _, addr := range filter.FromAddress {
		fromAddresses[addr] = true
	}
	for _, addr := range filter.ToAddress {
		toAddresses[addr] = true
	}
	limit := filter.After + filter.Count
	for number := fromBlock; number <= toBlock; number++ {
		if limit > 0 && len(results) >= int(limit) {
			break
		}
		block := s.hmy.BlockChain.GetBlockByNumber(uint64(number))
		if resp, err := s.hmy.NodeAPI.GetTraceResultWithFilter(block.Hash(), fromAddresses, toAddresses); err == nil {
			results = append(results, resp...)
		} else {
			return nil, err
		}
	}
	if limit == 0 {
		return results, nil
	}
	if limit > uint(len(results)) {
		limit = uint(len(results))
	}
	if filter.After > limit {
		return results[:0], nil
	}
	return results[filter.After:limit], nil
}
