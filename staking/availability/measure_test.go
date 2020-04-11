package availability

import (
	"errors"
	"fmt"
	"math/big"
	"reflect"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/shard"
)

func TestBlockSigners(t *testing.T) {
	tests := []struct {
		numSlots               int
		verified               []int
		numPayable, numMissing int
	}{
		{0, []int{}, 0, 0},
		{1, []int{}, 0, 1},
		{1, []int{0}, 1, 0},
		{8, []int{}, 0, 8},
		{8, []int{0}, 1, 7},
		{8, []int{7}, 1, 7},
		{8, []int{1, 3, 5, 7}, 4, 4},
		{8, []int{0, 2, 4, 6}, 4, 4},
		{8, []int{0, 1, 2, 3, 4, 5, 6, 7}, 8, 0},
		{13, []int{0, 1, 4, 5, 6, 9, 12}, 7, 6},
		// TODO: add a real data test case given numSlots of a committee and
		//  number of payable of a certain block
	}
	for i, test := range tests {
		cmt := makeTestCommittee(test.numSlots, 0)
		bm, err := indexesToBitMap(test.verified, test.numSlots)
		if err != nil {
			t.Fatalf("test %d: %v", i, err)
		}
		pSlots, mSlots, err := BlockSigners(bm, cmt)
		if err != nil {
			t.Fatalf("test %d: %v", i, err)
		}
		if len(pSlots) != test.numPayable || len(mSlots) != test.numMissing {
			t.Errorf("test %d: unexpected result: # pSlots %d/%d, # mSlots %d/%d",
				i, len(pSlots), test.numPayable, len(mSlots), test.numMissing)
			continue
		}
		if err := checkPayableAndMissing(cmt, test.verified, pSlots, mSlots); err != nil {
			t.Errorf("test %d: %v", i, err)
		}
	}
}

func checkPayableAndMissing(cmt *shard.Committee, idxs []int, pSlots, mSlots shard.SlotList) error {
	if len(pSlots)+len(mSlots) != len(cmt.Slots) {
		return fmt.Errorf("slots number not expected: %d(payable) + %d(missing) != %d(committee)",
			len(pSlots), len(mSlots), len(cmt.Slots))
	}
	pIndex, mIndex, iIndex := 0, 0, 0
	for i, slot := range cmt.Slots {
		if iIndex >= len(idxs) || i != idxs[iIndex] {
			// the slot should be missing and we shall check mSlots[mIndex] == slot
			if mIndex >= len(mSlots) || !reflect.DeepEqual(slot, mSlots[mIndex]) {
				return fmt.Errorf("addr %v missed from missing slots", slot.EcdsaAddress.String())
			}
			mIndex += 1
		} else {
			// check pSlots[pIndex] == slot
			if pIndex >= len(pSlots) || !reflect.DeepEqual(slot, pSlots[pIndex]) {
				return fmt.Errorf("addr %v missed from payable slots", slot.EcdsaAddress.String())
			}
			pIndex += 1
			iIndex += 1
		}
	}
	return nil
}

func TestBlockSigners_BitmapOverflow(t *testing.T) {
	tests := []struct {
		numSlots  int
		numBitmap int
		err       error
	}{
		{16, 16, nil},
		{16, 14, nil},
		{16, 8, errors.New("bitmap size too small")},
		{16, 24, errors.New("bitmap size too large")},
	}
	for i, test := range tests {
		cmt := makeTestCommittee(test.numSlots, 0)
		bm, _ := indexesToBitMap([]int{}, test.numBitmap)
		_, _, err := BlockSigners(bm, cmt)
		if (err == nil) != (test.err == nil) {
			t.Errorf("Test %d: BlockSigners got err [%v], expect [%v]", i, err, test.err)
		}
	}
}

func makeTestCommittee(n int, shardID uint32) *shard.Committee {
	slots := make(shard.SlotList, 0, n)
	for i := 0; i != n; i++ {
		slots = append(slots, makeTestSlot(i))
	}
	return &shard.Committee{
		ShardID: shardID,
		Slots:   slots,
	}
}

func makeTestSlot(seed int) shard.Slot {
	addr := common.BigToAddress(new(big.Int).SetInt64(int64(seed)))
	var blsKey shard.BLSPublicKey
	copy(blsKey[:], bls.RandPrivateKey().GetPublicKey().Serialize())

	return shard.Slot{
		EcdsaAddress: addr,
		BLSPublicKey: blsKey,
	}
}

// indexesToBitMap convert the indexes to bitmap. The conversion follows the little-
// endian order.
func indexesToBitMap(idxs []int, n int) ([]byte, error) {
	bSize := (n + 7) >> 3
	res := make([]byte, bSize)
	for _, idx := range idxs {
		byt := idx >> 3
		if byt >= bSize {
			return nil, fmt.Errorf("overflow index when converting to bitmap: %v/%v", byt, bSize)
		}
		msk := byte(1) << uint(idx&7)
		res[byt] ^= msk
	}
	return res, nil
}
