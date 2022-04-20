package redis_helper

import (
	"context"
	"fmt"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/utils"
	stakingTypes "github.com/harmony-one/harmony/staking/types"
	"github.com/pkg/errors"
	"io"
)

type BlockUpdate struct {
	BlkNum uint64
	Logs   []*types.Log
}

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

type TxPoolUpdate struct {
	typ   string
	Local bool
	Tx    types.PoolTransaction
}

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

func PublishTxPoolUpdate(shardID uint32, tx types.PoolTransaction, local bool) error {
	txu := &TxPoolUpdate{Local: local, Tx: tx}
	msg, err := rlp.EncodeToBytes(txu)
	if err != nil {
		return err
	}
	return redisInstance.Publish(context.Background(), fmt.Sprintf("txpool_update_%d", shardID), msg).Err()
}
