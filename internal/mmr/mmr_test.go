package mmr

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/harmony-one/harmony/internal/mmr/db"
	"github.com/zmitton/go-merklemountainrange/position"
)

type MmrProofX struct {
	Root     []byte
	Width    uint64
	Index    uint64
	Peaks    [][]byte
	Siblings [][]byte
}

func TestMMR(t *testing.T) {
	memoryBasedDb1 := db.NewMemorybaseddb(0, map[int64][]byte{})
	memoryBasedMmr1 := New(memoryBasedDb1, big.NewInt(0))

	for i := 0; i < 32768; i++ {
		hash := Keccak256([]byte(hexutil.EncodeUint64(uint64(i))))
		memoryBasedMmr1.Append(hash, int64(i))
	}

	// val, _ := memoryBasedMmr1.GetUnverified(3)
	// fmt.Println(fmt.Sprintf("%x", val))

	// fmt.Println(memoryBasedMmr1.GetLeafLength())
	// fmt.Println(memoryBasedMmr1.Nodes())

	l := memoryBasedMmr1.GetLeafLength()
	for i := int64(0); i < l; i++ {
		// if i != 3 {
		// 	continue
		// }
		fmt.Println("-------------", i, l)
		fmt.Println(position.ProofPositions([]int64{i}, l))
		fmt.Println("Node: ", fmt.Sprintf("%x", memoryBasedMmr1.Get(i)))
		fmt.Println("Position: ", position.GetNodePosition(i).Index)
		root, width, index, peaks, siblings := memoryBasedMmr1.GetMerkleProof(i)
		fmt.Println("Root: ", fmt.Sprintf("%x", root))
		fmt.Println("width: ", width)
		fmt.Println("index: ", index)
		fmt.Println("Peaks: ", fmt.Sprintf("%x", peaks))
		fmt.Println("Siblings: ", fmt.Sprintf("%x", siblings))
		proof := MmrProofX{
			Root:     root,
			Width:    uint64(width),
			Index:    uint64(index),
			Peaks:    peaks,
			Siblings: siblings,
		}
		b, err := rlp.EncodeToBytes(proof)
		fmt.Println("====================================================", len(b), err)
		// break
		// indices := []int64{i}
		// positions := position.ProofPositions(indices, l)
		// delete(positions, indices[0])
		// fmt.Println(i, positions)
		// peakPositions := position.PeakPositions(l - 1)
		// fmt.Println(i, peakPositions)
		// for i := range peakPositions {
		// 	fmt.Println(fmt.Sprintf("%x", memoryBasedMmr1.NodeValue(peakPositions[i])))
		// }
		// fmt.Println(i, fmt.Sprintf("%x", memoryBasedMmr1.GetRoot()))
	}

	// t.Run("#Append to in-memory mmr", func(t *testing.T) {
	// 	leafLength := fileBasedDb1.GetLeafLength()
	// 	for i := int64(0); i < leafLength; i++ {
	// 		leaf, _ := fileBasedMmr1.GetUnverified(i)
	// 		memoryBasedMmr1.Append(leaf, i)
	// 	}
	// })

	// fmt.Println(string(fileBasedMmr1.Serialize()))

	// sum := sha256.Sum256([]byte("hello world123\n"))

	// bech, err := bech32.ConvertAndEncode("shasum", sum[:])

	// if err != nil {
	// 	t.Error(err)
	// }
	// hrp, data, err := bech32.DecodeAndConvert(bech)

	// if err != nil {
	// 	t.Error(err)
	// }
	// if hrp != "shasum" {
	// 	t.Error("Invalid hrp")
	// }
	// if !bytes.Equal(data, sum[:]) {
	// 	t.Error("Invalid decode")
	// }
}
