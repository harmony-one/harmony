package genesis

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/btcsuite/btcutil/bech32"
	"github.com/harmony-one/bls/ffi/go/bls"
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

func TestCommitteeAccounts(test *testing.T) {
	testAccounts(test, FoundationalNodeAccounts)
	testAccounts(test, FoundationalNodeAccountsV0_1)
	testAccounts(test, FoundationalNodeAccountsV0_2)
	testAccounts(test, FoundationalNodeAccountsV0_3)
	testAccounts(test, FoundationalNodeAccountsV0_4)
	testAccounts(test, HarmonyAccounts)
	testAccounts(test, TNHarmonyAccounts)
	testAccounts(test, TNFoundationalAccounts)
	testAccounts(test, PangaeaAccounts)
}

func testAccounts(test *testing.T, accounts []DeployAccount) {
	index := 0
	for _, account := range accounts {
		accIndex, _ := strconv.Atoi(strings.Trim(account.Index, " "))
		if accIndex != index {
			test.Error("Account index", account.Index, "not in sequence")
		}
		index++

		_, _, err := bech32.Decode(account.Address)
		if err != nil {
			test.Error("Account address", account.Address, "is not valid:", err)
		}

		pubKey := bls.PublicKey{}
		err = pubKey.DeserializeHexStr(account.BlsPublicKey)
		if err != nil {
			test.Error("Account bls public key", account.BlsPublicKey, "is not valid:", err)
		}
	}
}
