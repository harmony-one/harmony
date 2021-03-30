package sync

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	protobuf "github.com/golang/protobuf/proto"
	"github.com/harmony-one/harmony/block"
	"github.com/harmony-one/harmony/core/types"
	syncpb "github.com/harmony-one/harmony/p2p/stream/protocols/sync/message"
)

type testChainHelper struct{}

func (tch *testChainHelper) getCurrentBlockNumber() uint64 {
	return 100
}

func (tch *testChainHelper) getBlocksByNumber(bns []uint64) ([]*types.Block, error) {
	blocks := make([]*types.Block, 0, len(bns))
	for _, bn := range bns {
		blocks = append(blocks, makeTestBlock(bn))
	}
	return blocks, nil
}

func (tch *testChainHelper) getBlockHashes(bns []uint64) []common.Hash {
	hs := make([]common.Hash, 0, len(bns))
	for _, bn := range bns {
		hs = append(hs, numberToHash(bn))
	}
	return hs
}

func (tch *testChainHelper) getBlocksByHashes(hs []common.Hash) ([]*types.Block, error) {
	bs := make([]*types.Block, 0, len(hs))
	for _, h := range hs {
		bn := hashToNumber(h)
		bs = append(bs, makeTestBlock(bn))
	}
	return bs, nil
}

func numberToHash(bn uint64) common.Hash {
	var h common.Hash
	binary.LittleEndian.PutUint64(h[:], bn)
	return h
}

func hashToNumber(h common.Hash) uint64 {
	return binary.LittleEndian.Uint64(h[:])
}

func checkBlocksResult(bns []uint64, b []byte) error {
	var msg = &syncpb.Message{}
	if err := protobuf.Unmarshal(b, msg); err != nil {
		return err
	}
	gbResp, err := msg.GetBlocksByNumberResponse()
	if err != nil {
		return err
	}
	if len(gbResp.BlocksBytes) == 0 {
		return errors.New("nil response from GetBlocksByNumber")
	}
	blocks, err := decodeBlocksBytes(gbResp.BlocksBytes)
	if err != nil {
		return err
	}
	if len(blocks) != len(bns) {
		return errors.New("unexpected blocks number")
	}
	for i, bn := range bns {
		blk := blocks[i]
		if bn != blk.NumberU64() {
			return errors.New("unexpected number of a block")
		}
	}
	return nil
}

func makeTestBlock(bn uint64) *types.Block {
	header := testHeader.Copy()
	header.SetNumber(big.NewInt(int64(bn)))
	return types.NewBlockWithHeader(&block.Header{Header: header})
}

func decodeBlocksBytes(bbs [][]byte) ([]*types.Block, error) {
	blocks := make([]*types.Block, 0, len(bbs))

	for _, bb := range bbs {
		var block *types.Block
		if err := rlp.DecodeBytes(bb, &block); err != nil {
			return nil, err
		}
		blocks = append(blocks, block)
	}
	return blocks, nil
}

func checkBlockNumberResult(b []byte) error {
	var msg = &syncpb.Message{}
	if err := protobuf.Unmarshal(b, msg); err != nil {
		return err
	}
	gnResp, err := msg.GetBlockNumberResponse()
	if err != nil {
		return err
	}
	if gnResp.Number != testCurBlockNumber {
		return fmt.Errorf("unexpected block number: %v / %v", gnResp.Number, testCurBlockNumber)
	}
	return nil
}

func checkBlockHashesResult(b []byte, bns []uint64) error {
	var msg = &syncpb.Message{}
	if err := protobuf.Unmarshal(b, msg); err != nil {
		return err
	}
	bhResp, err := msg.GetBlockHashesResponse()
	if err != nil {
		return err
	}
	got := bhResp.Hashes
	if len(got) != len(bns) {
		return errors.New("unexpected size")
	}
	for i, bn := range bns {
		expect := numberToHash(bn)
		if !bytes.Equal(expect[:], got[i]) {
			return errors.New("unexpected hash")
		}
	}
	return nil
}

func checkBlocksByHashesResult(b []byte, hs []common.Hash) error {
	var msg = &syncpb.Message{}
	if err := protobuf.Unmarshal(b, msg); err != nil {
		return err
	}
	bhResp, err := msg.GetBlocksByHashesResponse()
	if err != nil {
		return err
	}
	if len(hs) != len(bhResp.BlocksBytes) {
		return errors.New("unexpected size")
	}
	for i, h := range hs {
		num := hashToNumber(h)
		var blk *types.Block
		if err := rlp.DecodeBytes(bhResp.BlocksBytes[i], &blk); err != nil {
			return err
		}
		if blk.NumberU64() != num {
			return fmt.Errorf("unexpected number %v != %v", blk.NumberU64(), num)
		}
	}
	return nil
}
