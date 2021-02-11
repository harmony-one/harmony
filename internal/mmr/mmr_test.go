package mmr

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"testing"

	"github.com/zmitton/go-merklemountainrange/db"
	"github.com/zmitton/go-merklemountainrange/position"
)

func TestMMR(t *testing.T) {
	memoryBasedDb1 := db.NewMemorybaseddb(0, map[int64][]byte{})
	memoryBasedMmr1 := New(Keccak256, memoryBasedDb1, big.NewInt(0), 0)

	blockHashes := []string{
		"61ce03ef5efa374b0d0d527ea38c3d13cb05cf765a4f898e91a5de1f6b224cdd",
		"43fb38c1bb9b26da76ce8188fa0f5835cc0636bed903bd710d221cd0c8403aa0",
		"c72b3724cae29c8e5e2e2902328dc3a3e7718d32dcf07996ceac104241fe8a4c",
		"f41f573a8cc4e8af006d5f7f418ca7ac2527f75bf7f202a9a0ec2c8a69c8bd18",
		"67616b79a0d9df8e1ccddfeed8876351cc8a6c448289eacef1f57d9c1e27df20",
	}

	for i := range blockHashes {
		hash := blockHashes[i]
		if data, err := hex.DecodeString(hash); err == nil {
			memoryBasedMmr1.Append(data, int64(i))
		}
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
		fmt.Println(position.ProofPositions([]int64{i}, l))
		fmt.Println("Node: ", fmt.Sprintf("%x", memoryBasedMmr1.Get(i)))
		fmt.Println("Position: ", position.GetNodePosition(i).Index)
		root, width, index, peaks, siblings := memoryBasedMmr1.GetMerkleProof(i)
		fmt.Println("Root: ", fmt.Sprintf("%x", root))
		fmt.Println("width: ", width)
		fmt.Println("index: ", index)
		fmt.Println("Peaks: ", fmt.Sprintf("%x", peaks))
		fmt.Println("Siblings: ", fmt.Sprintf("%x", siblings))
		fmt.Println("====================================================")
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
