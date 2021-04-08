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
	parityTraceJS = "blockTracer"
)

type PublicParityTracerService struct {
	*PublicTracerService
}

func (s *PublicParityTracerService) Transaction(ctx context.Context, hash common.Hash) (interface{}, error) {
	return s.TraceTransaction(ctx, hash, &hmy.TraceConfig{Tracer: &parityTraceJS})
}

// trace_block RPC
func (s *PublicParityTracerService) Block(ctx context.Context, number rpc.BlockNumber) (interface{}, error) {
	block := s.hmy.BlockChain.GetBlockByNumber(uint64(number))
	if block == nil {
		return nil, nil
	}
	results, err := s.hmy.TraceBlock(ctx, block, &hmy.TraceConfig{Tracer: &parityTraceJS})
	if err != nil {
		return results, err
	}
	var resultArray = make([]interface{}, 0)
	for _, result := range results {
		raw, ok := result.Result.(json.RawMessage)
		if !ok {
			return results, errors.New("expected json.RawMessage")
		}
		var subArray []interface{}
		json.Unmarshal(raw, &subArray)
		resultArray = append(resultArray, subArray...)
	}
	return resultArray, nil
}
