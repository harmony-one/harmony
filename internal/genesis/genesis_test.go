package genesis

import (
	"bufio"
	"fmt"
	"os"
	"strings"
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
	lines, err := fileToLines("/home/ec2-user/tmp/alloneaccounts.txt")
	if err != nil {
		t.Fatal("ReadFile failed", err)
	}

	for i, line := range lines {
		fmt.Printf("   {Index: \"%v\", Address: \"%v\", BlsPublicKey: \"%v\"},\n", GenesisAccounts[i].Index, strings.Trim(line, " "), GenesisAccounts[i].BlsPublicKey)
	}
}
