package shard

import (
	"bytes"
	"testing"

	"github.com/ethereum/go-ethereum/common"
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

	if bytes.Compare(h1, h2) == 0 {
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
	shardState1 := State{com1, com2}
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

	shardState2 := State{com3, com4}
	h2 := shardState2.Hash()

	if bytes.Compare(h1[:], h2[:]) != 0 {
		t.Error("shardState1 and shardState2 should have equal hash")
	}
}
