package redis_helper

import (
	"context"
	"fmt"
	"io"

	"github.com/ethereum/go-ethereum"

	"github.com/ethereum/go-ethereum/rlp"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/utils"
	stakingTypes "github.com/harmony-one/harmony/staking/types"
	"github.com/pkg/errors"
)

// BlockUpdate block update event
type BlockUpdate struct {
	BlkNum uint64
	Logs   []*types.Log
}

// NewFilterUpdated new filter update event
type NewFilterUpdated struct {
	ID             string
	FilterCriteria ethereum.FilterQuery
}

// SubscribeShardUpdate subscribe block update event
func SubscribeShardUpdate(shardID uint32, cb func(blkNum uint64, logs []*types.Log)) {
	pubsub := redisInstance.Subscribe(context.Background(), fmt.Sprintf("shard_update_%d", shardID))
	for message := range pubsub.Channel() {
		block := &BlockUpdate{}
		err := rlp.DecodeBytes([]byte(message.Payload), block)
		if err != nil {
			utils.Logger().Info().Err(err).Msg("redis subscribe shard update error")
			continue
		}
		cb(block.BlkNum, block.Logs)
	}
}

// PublishShardUpdate publish block update event
func PublishShardUpdate(shardID uint32, blkNum uint64, logs []*types.Log) error {
	msg, err := rlp.EncodeToBytes(&BlockUpdate{
		BlkNum: blkNum,
		Logs:   logs,
	})
	if err != nil {
		return err
	}
	return redisInstance.Publish(context.Background(), fmt.Sprintf("shard_update_%d", shardID), msg).Err()
}

// SubscribeNewFilterLogEvent subscribe new filter log event from other readers
func SubscribeNewFilterLogEvent(shardID uint32, namespace string, cb func(id string, crit ethereum.FilterQuery)) {
	if redisInstance == nil {
		return
	}
	pubsub := redisInstance.
		Subscribe(context.Background(), fmt.Sprintf("%s_new_filter_log_%d", namespace, shardID))
	for message := range pubsub.Channel() {
		query := NewFilterUpdated{}
		err := rlp.DecodeBytes([]byte(message.Payload), &query)
		if err != nil {
			utils.Logger().Info().Err(err).Msg("redis subscribe new_filter_log_ error")
			continue
		}
		cb(query.ID, query.FilterCriteria)
	}
}

// PublishNewFilterLogEvent publish new filter log event from other readers
func PublishNewFilterLogEvent(shardID uint32, namespace, id string, crit ethereum.FilterQuery) error {
	if redisInstance == nil {
		return nil
	}

	ev := NewFilterUpdated{
		FilterCriteria: crit,
		ID:             id,
	}
	msg, err := rlp.EncodeToBytes(&ev)
	if err != nil {
		return err
	}
	return redisInstance.
		Publish(context.Background(), fmt.Sprintf("%s_new_filter_log_%d", namespace, shardID), msg).Err()
}

//TxPoolUpdate tx pool update event
type TxPoolUpdate struct {
	typ   string
	Local bool
	Tx    types.PoolTransaction
}

// DecodeRLP decode struct from binary stream
func (t *TxPoolUpdate) DecodeRLP(stream *rlp.Stream) error {
	if err := stream.Decode(&t.typ); err != nil {
		return err
	}
	if err := stream.Decode(&t.Local); err != nil {
		return err
	}

	switch t.typ {
	case "types.EthTransaction":
		var tmp = &types.EthTransaction{}
		if err := stream.Decode(tmp); err != nil {
			return err
		}
		t.Tx = tmp
	case "types.Transaction":
		var tmp = &types.Transaction{}
		if err := stream.Decode(tmp); err != nil {
			return err
		}
		t.Tx = tmp
	case "stakingTypes.StakingTransaction":
		var tmp = &stakingTypes.StakingTransaction{}
		if err := stream.Decode(tmp); err != nil {
			return err
		}
		t.Tx = tmp
	default:
		return errors.New("unknown txpool type")
	}
	return nil
}

// EncodeRLP encode struct to binary stream
func (t *TxPoolUpdate) EncodeRLP(w io.Writer) error {
	switch t.Tx.(type) {
	case *types.EthTransaction:
		t.typ = "types.EthTransaction"
	case *types.Transaction:
		t.typ = "types.Transaction"
	case *stakingTypes.StakingTransaction:
		t.typ = "stakingTypes.StakingTransaction"
	}

	if err := rlp.Encode(w, t.typ); err != nil {
		return err
	}
	if err := rlp.Encode(w, t.Local); err != nil {
		return err
	}
	return rlp.Encode(w, t.Tx)
}

// SubscribeTxPoolUpdate subscribe tx pool update event
func SubscribeTxPoolUpdate(shardID uint32, cb func(tx types.PoolTransaction, local bool)) {
	pubsub := redisInstance.Subscribe(context.Background(), fmt.Sprintf("txpool_update_%d", shardID))
	for message := range pubsub.Channel() {
		txu := &TxPoolUpdate{}
		err := rlp.DecodeBytes([]byte(message.Payload), &txu)
		if err != nil {
			utils.Logger().Info().Err(err).Msg("redis subscribe shard update error")
			continue
		}
		cb(txu.Tx, txu.Local)
	}
}

// PublishTxPoolUpdate publish tx pool update event
func PublishTxPoolUpdate(shardID uint32, tx types.PoolTransaction, local bool) error {
	txu := &TxPoolUpdate{Local: local, Tx: tx}
	msg, err := rlp.EncodeToBytes(txu)
	if err != nil {
		return err
	}
	return redisInstance.Publish(context.Background(), fmt.Sprintf("txpool_update_%d", shardID), msg).Err()
}
