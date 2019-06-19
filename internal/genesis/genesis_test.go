package genesis

import (
	"bufio"
	"fmt"
	"os"
	"testing"
)

func TestString(t *testing.T) {
	_ = BeaconAccountPriKey()
}

func fileToLines(filePath string) (lines []string, err error) {
	f, err := os.Open(filePath)
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	err = scanner.Err()
	return
}

func testGenesisccounts(t *testing.T) {
	ones, err := fileToLines("one-acc.txt")
	if err != nil {
		t.Fatal("ReadFile failed", err)
	}

	bls, err := fileToLines("bls-pub.txt")
	if err != nil {
		t.Fatal("ReadFile failed", err)
	}

	index := 404
	for i, one := range ones {
		fmt.Printf("   {Index: \" %v \", Address: \"%v\", BlsPublicKey: \"%v\"},\n", index, one, bls[i])
		index++
	}
}
