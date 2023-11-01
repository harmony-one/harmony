package sync

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math/big"
	"time"
	"unsafe"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	protobuf "github.com/golang/protobuf/proto"
	"github.com/harmony-one/harmony/block"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/p2p/stream/protocols/sync/message"
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

func (tch *testChainHelper) getNodeData(hs []common.Hash) ([][]byte, error) {
	data := makeTestNodeData(len(hs))
	return data, nil
}

func (tch *testChainHelper) getReceipts(hs []common.Hash) ([]types.Receipts, error) {
	testReceipts := makeTestReceipts(len(hs), 3)
	receipts := make([]types.Receipts, len(hs))
	for i, _ := range hs {
		receipts[i] = testReceipts
	}
	return receipts, nil
}

func (ch *testChainHelper) getAccountRange(root common.Hash, origin common.Hash, limit common.Hash, bytes uint64) ([]*message.AccountData, [][]byte, error) {
	testAccountRanges, testProofs := makeTestAccountRanges(2)
	return testAccountRanges, testProofs, nil
}

func (ch *testChainHelper) getStorageRanges(root common.Hash, accounts []common.Hash, origin common.Hash, limit common.Hash, bytes uint64) ([]*message.StoragesData, [][]byte, error) {
	testSlots, testProofs := makeTestStorageRanges(2)
	return testSlots, testProofs, nil
}

func (ch *testChainHelper) getByteCodes(hs []common.Hash, bytes uint64) ([][]byte, error) {
	testByteCodes := makeTestByteCodes(2)
	return testByteCodes, nil
}

func (ch *testChainHelper) getTrieNodes(root common.Hash, paths []*message.TrieNodePathSet, bytes uint64, start time.Time) ([][]byte, error) {
	testTrieNodes := makeTestTrieNodes(2)
	return testTrieNodes, nil
}

func checkGetReceiptsResult(b []byte, hs []common.Hash) error {
	var msg = &syncpb.Message{}
	if err := protobuf.Unmarshal(b, msg); err != nil {
		return err
	}
	bhResp, err := msg.GetReceiptsResponse()
	if err != nil {
		return err
	}
	if len(hs) != len(bhResp.Receipts) {
		return errors.New("unexpected size")
	}
	return nil
}

func checkGetNodeDataResult(b []byte, hs []common.Hash) error {
	var msg = &syncpb.Message{}
	if err := protobuf.Unmarshal(b, msg); err != nil {
		return err
	}
	bhResp, err := msg.GetNodeDataResponse()
	if err != nil {
		return err
	}
	if len(hs) != len(bhResp.DataBytes) {
		return errors.New("unexpected size")
	}
	return nil
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

// makeTestReceipts creates fake node data
func makeTestNodeData(n int) [][]byte {
	testData := make([][]byte, n)
	for i := 0; i < n; i++ {
		testData[i] = types.EmptyRootHash.Bytes()
	}
	return testData
}

// makeTestReceipts creates fake receipts
func makeTestReceipts(n int, nPerBlock int) []*types.Receipt {
	receipts := make([]*types.Receipt, nPerBlock)
	for i := 0; i < nPerBlock; i++ {
		receipts[i] = &types.Receipt{
			Status:            types.ReceiptStatusSuccessful,
			CumulativeGasUsed: 0x888888888,
			Logs:              make([]*types.Log, 5),
		}
	}
	return receipts
}

func makeTestAccountRanges(n int) ([]*message.AccountData, [][]byte) {
	accounts := make([]*message.AccountData, n)
	proofs := make([][]byte, n)
	for i := 0; i < n; i++ {
		accounts[i] = &message.AccountData{
			Hash: numberToHash(uint64(i * 2)).Bytes(),
			Body: numberToHash(uint64(i*2 + 1)).Bytes(),
		}
	}
	for i := 0; i < n; i++ {
		proofs[i] = numberToHash(uint64(i)).Bytes()
	}
	return accounts, proofs
}

func makeTestStorageRanges(n int) ([]*message.StoragesData, [][]byte) {
	slots := make([]*message.StoragesData, n)
	proofs := make([][]byte, n)
	for i := 0; i < n; i++ {
		slots[i] = &message.StoragesData{
			Data: make([]*syncpb.StorageData, 2),
		}
		for j := 0; j < 2; j++ {
			slots[i].Data[j] = &message.StorageData{
				Hash: numberToHash(uint64(i * 2)).Bytes(),
				Body: numberToHash(uint64(i*2 + 1)).Bytes(),
			}
		}
	}
	for i := 0; i < n; i++ {
		proofs[i] = numberToHash(uint64(i)).Bytes()
	}
	return slots, proofs
}

func makeTestByteCodes(n int) [][]byte {
	byteCodes := make([][]byte, n)
	for i := 0; i < n; i++ {
		byteCodes[i] = numberToHash(uint64(i)).Bytes()
	}
	return byteCodes
}

func makeTestTrieNodes(n int) [][]byte {
	trieNodes := make([][]byte, n)
	for i := 0; i < n; i++ {
		trieNodes[i] = numberToHash(uint64(i)).Bytes()
	}
	return trieNodes
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

func decodeHashBytes(hs [][]byte) ([]common.Hash, error) {
	hashes := make([]common.Hash, 0)

	for _, h := range hs {
		var hash common.Hash
		if err := rlp.DecodeBytes(h, &hash); err != nil {
			return nil, err
		}
		hashes = append(hashes, hash)
	}
	return hashes, nil
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

func checkAccountRangeResult(bytes uint64, b []byte) error {
	var msg = &syncpb.Message{}
	if err := protobuf.Unmarshal(b, msg); err != nil {
		return err
	}
	gbResp, err := msg.GetAccountRangesResponse()
	if err != nil {
		return err
	}
	if len(gbResp.Accounts) == 0 {
		return errors.New("nil response from GetAccountRanges")
	}
	if len(gbResp.Proof) != len(gbResp.Accounts) {
		return errors.New("unexpected proofs")
	}
	if len(b) > int(bytes) {
		return errors.New("unexpected data bytes")
	}
	return nil
}

func checkStorageRangesResult(accounts []common.Hash, bytes uint64, b []byte) error {
	var msg = &syncpb.Message{}
	if err := protobuf.Unmarshal(b, msg); err != nil {
		return err
	}
	gbResp, err := msg.GetStorageRangesResponse()
	if err != nil {
		return err
	}
	if len(gbResp.Slots) == 0 {
		return errors.New("nil response from GetStorageRanges")
	}
	if len(gbResp.Slots) != len(gbResp.Proof) {
		return errors.New("unexpected proofs")
	}
	sz := unsafe.Sizeof(gbResp.Slots)
	if sz > uintptr(bytes) {
		return errors.New("unexpected slot bytes")
	}
	return nil
}

func checkByteCodesResult(hs []common.Hash, bytes uint64, b []byte) error {
	var msg = &syncpb.Message{}
	if err := protobuf.Unmarshal(b, msg); err != nil {
		return err
	}
	gbResp, err := msg.GetByteCodesResponse()
	if err != nil {
		return err
	}
	if len(gbResp.Codes) == 0 {
		return errors.New("nil response from GetByteCodes")
	}
	if len(gbResp.Codes) != len(hs) {
		return errors.New("unexpected byte codes")
	}
	sz := len(hs) * common.HashLength
	if sz > int(bytes) {
		return errors.New("unexpected data bytes")
	}
	return nil
}

func checkTrieNodesResult(hs []common.Hash, bytes uint64, b []byte) error {
	var msg = &syncpb.Message{}
	if err := protobuf.Unmarshal(b, msg); err != nil {
		return err
	}
	gbResp, err := msg.GetTrieNodesResponse()
	if err != nil {
		return err
	}
	if len(gbResp.Nodes) == 0 {
		return errors.New("nil response from checkGetTrieNodes")
	}
	if len(gbResp.Nodes) != len(hs) {
		return errors.New("unexpected byte codes")
	}
	sz := len(hs) * common.HashLength
	if sz > int(bytes) {
		return errors.New("unexpected data bytes")
	}
	return nil
}
