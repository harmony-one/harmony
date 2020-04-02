package shard

import (
	"bytes"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/harmony-one/harmony/numeric"
)

var (
	blsPubKey1  = [48]byte{}
	blsPubKey2  = [48]byte{}
	blsPubKey3  = [48]byte{}
	blsPubKey4  = [48]byte{}
	blsPubKey5  = [48]byte{}
	blsPubKey6  = [48]byte{}
	blsPubKey11 = [48]byte{}
	blsPubKey22 = [48]byte{}
)

const (
	json1 = `[{"shard-id":0,"member-count":4,"subcommittee":[{"bls-pubkey":"72616e646f6d206b65792031000000000000000000000000000000000000000000000000000000000000000000000000","effective-stake":null,"ecdsa-address":"one1qqqqqqqqqqqqqqqqqqqqqqq423ktdf2pznf238"},{"bls-pubkey":"72616e646f6d206b65792032000000000000000000000000000000000000000000000000000000000000000000000000","effective-stake":null,"ecdsa-address":"one1qqqqqqqqqqqqqqqqqqqqqqq423ktdf2pznf238"},{"bls-pubkey":"72616e646f6d206b65792033000000000000000000000000000000000000000000000000000000000000000000000000","effective-stake":null,"ecdsa-address":"one1qqqqqqqqqqqqqqqqqqqqqqq423ktdf2pznf238"},{"bls-pubkey":"72616e646f6d206b65792034000000000000000000000000000000000000000000000000000000000000000000000000","effective-stake":null,"ecdsa-address":"one1qqqqqqqqqqqqqqqqqqqqqqq423ktdf2pznf238"}]},{"shard-id":1,"member-count":2,"subcommittee":[{"bls-pubkey":"72616e646f6d206b65792035000000000000000000000000000000000000000000000000000000000000000000000000","effective-stake":null,"ecdsa-address":"one1qqqqqqqqqqqqqqqqqqqqqqq423ktdf2pznf238"},{"bls-pubkey":"72616e646f6d206b65792036000000000000000000000000000000000000000000000000000000000000000000000000","effective-stake":null,"ecdsa-address":"one1qqqqqqqqqqqqqqqqqqqqqqq423ktdf2pznf238"}]}]`
	json2 = `[{"shard-id":0,"member-count":5,"subcommittee":[{"bls-pubkey":"72616e646f6d206b65792031000000000000000000000000000000000000000000000000000000000000000000000000","effective-stake":null,"ecdsa-address":"one1qqqqqqqqqqqqqqqqqqqqqqq423ktdf2pznf238"},{"bls-pubkey":"72616e646f6d206b65792032000000000000000000000000000000000000000000000000000000000000000000000000","effective-stake":null,"ecdsa-address":"one1qqqqqqqqqqqqqqqqqqqqqqq423ktdf2pznf238"},{"bls-pubkey":"72616e646f6d206b65792033000000000000000000000000000000000000000000000000000000000000000000000000","effective-stake":"10.000000000000000000","ecdsa-address":"one1qqqqqqqqqqqqqqqqqqqqqqq423ktdf2pznf238"},{"bls-pubkey":"72616e646f6d206b65792035000000000000000000000000000000000000000000000000000000000000000000000000","effective-stake":null,"ecdsa-address":"one1qqqqqqqqqqqqqqqqqqqqqqq423ktdf2pznf238"},{"bls-pubkey":"72616e646f6d206b65792031000000000000000000000000000000000000000000000000000000000000000000000000","effective-stake":"45.123000000000000000","ecdsa-address":"one1qqqqqqqqqqqqqqqqqqqqqqq423ktdf2pznf238"}]},{"shard-id":1,"member-count":4,"subcommittee":[{"bls-pubkey":"72616e646f6d206b65792032000000000000000000000000000000000000000000000000000000000000000000000000","effective-stake":"10.000000000000000000","ecdsa-address":"one1qqqqqqqqqqqqqqqqqqqqqqq423ktdf2pznf238"},{"bls-pubkey":"72616e646f6d206b65792033000000000000000000000000000000000000000000000000000000000000000000000000","effective-stake":null,"ecdsa-address":"one1qqqqqqqqqqqqqqqqqqqqqqq423ktdf2pznf238"},{"bls-pubkey":"72616e646f6d206b65792034000000000000000000000000000000000000000000000000000000000000000000000000","effective-stake":null,"ecdsa-address":"one1qqqqqqqqqqqqqqqqqqqqqqq423ktdf2pznf238"},{"bls-pubkey":"72616e646f6d206b65792035000000000000000000000000000000000000000000000000000000000000000000000000","effective-stake":"45.123000000000000000","ecdsa-address":"one1qqqqqqqqqqqqqqqqqqqqqqq423ktdf2pznf238"}]}]`
)

func init() {
	copy(blsPubKey1[:], []byte("random key 1"))
	copy(blsPubKey2[:], []byte("random key 2"))
	copy(blsPubKey3[:], []byte("random key 3"))
	copy(blsPubKey4[:], []byte("random key 4"))
	copy(blsPubKey5[:], []byte("random key 5"))
	copy(blsPubKey6[:], []byte("random key 6"))
	copy(blsPubKey11[:], []byte("random key 11"))
	copy(blsPubKey22[:], []byte("random key 22"))
}

func TestGetHashFromNodeList(t *testing.T) {
	l1 := []Slot{
		{common.Address{0x11}, blsPubKey1, nil},
		{common.Address{0x22}, blsPubKey2, nil},
		{common.Address{0x33}, blsPubKey3, nil},
	}
	l2 := []Slot{
		{common.Address{0x22}, blsPubKey2, nil},
		{common.Address{0x11}, blsPubKey1, nil},
		{common.Address{0x33}, blsPubKey3, nil},
	}
	h1 := GetHashFromNodeList(l1)
	h2 := GetHashFromNodeList(l2)

	if bytes.Equal(h1, h2) {
		t.Error("node list l1 and l2 should be different")
	}
}

func TestHash(t *testing.T) {
	com1 := Committee{
		ShardID: 22,
		Slots: []Slot{
			{common.Address{0x12}, blsPubKey11, nil},
			{common.Address{0x23}, blsPubKey22, nil},
			{common.Address{0x11}, blsPubKey1, nil},
		},
	}
	com2 := Committee{
		ShardID: 2,
		Slots: []Slot{
			{common.Address{0x44}, blsPubKey4, nil},
			{common.Address{0x55}, blsPubKey5, nil},
			{common.Address{0x66}, blsPubKey6, nil},
		},
	}
	shardState1 := State{nil, []Committee{com1, com2}}
	h1 := shardState1.Hash()

	com3 := Committee{
		ShardID: 2,
		Slots: []Slot{
			{common.Address{0x44}, blsPubKey4, nil},
			{common.Address{0x55}, blsPubKey5, nil},
			{common.Address{0x66}, blsPubKey6, nil},
		},
	}
	com4 := Committee{
		ShardID: 22,
		Slots: []Slot{
			{common.Address{0x12}, blsPubKey11, nil},
			{common.Address{0x23}, blsPubKey22, nil},
			{common.Address{0x11}, blsPubKey1, nil},
		},
	}

	shardState2 := State{nil, []Committee{com3, com4}}
	h2 := shardState2.Hash()

	if !bytes.Equal(h1[:], h2[:]) {
		t.Error("shardState1 and shardState2 should have equal hash")
	}
}

func TestCompatibilityOldShardStateIntoNew(t *testing.T) {

	junkA := common.BigToAddress(big.NewInt(23452345345345))
	stake1 := numeric.NewDec(10)
	stake2 := numeric.MustNewDecFromStr("45.123")

	preStakingState := StateLegacy{
		CommitteeLegacy{ShardID: 0, Slots: SlotListLegacy{
			SlotLegacy{junkA, blsPubKey1},
			SlotLegacy{junkA, blsPubKey2},
			SlotLegacy{junkA, blsPubKey3},
			SlotLegacy{junkA, blsPubKey4},
		}},
		CommitteeLegacy{ShardID: 1, Slots: SlotListLegacy{
			SlotLegacy{junkA, blsPubKey5},
			SlotLegacy{junkA, blsPubKey6},
		}},
	}

	postStakingState := State{nil, []Committee{
		Committee{ShardID: 0, Slots: SlotList{
			Slot{junkA, blsPubKey1, nil},
			Slot{junkA, blsPubKey2, nil},
			Slot{junkA, blsPubKey3, &stake1},
			Slot{junkA, blsPubKey5, nil},
			Slot{junkA, blsPubKey1, &stake2},
		}},
		Committee{ShardID: 1, Slots: SlotList{
			Slot{junkA, blsPubKey2, &stake1},
			Slot{junkA, blsPubKey3, nil},
			Slot{junkA, blsPubKey4, nil},
			Slot{junkA, blsPubKey5, &stake2},
		}},
	}}

	preStakingStateBytes, _ := rlp.EncodeToBytes(preStakingState)
	postStakingStateBytes, _ := rlp.EncodeToBytes(postStakingState)
	decodeJunk := []byte{0x10, 0x12, 0x04, 0x4}
	// Decode old shard state into new shard state
	a, err1 := DecodeWrapper(preStakingStateBytes)
	b, err2 := DecodeWrapper(postStakingStateBytes)
	_, err3 := DecodeWrapper(decodeJunk)

	if err1 != nil {
		t.Errorf("Could not decode old format")
	}
	if err2 != nil {
		t.Errorf("Could not decode new format")
	}

	if a.String() != json1 {
		t.Error("old shard state into new shard state as JSON is not equal")
	}

	if b.String() != json2 {
		t.Error("new shard state into new shard state as JSON is not equal")
	}

	if err3 == nil {
		t.Errorf("Should have caused error %v", err3)
	}

}
