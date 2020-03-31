package genesis

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/btcsuite/btcutil/bech32"
	ethCommon "github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/bls/ffi/go/bls"

	"github.com/harmony-one/harmony/internal/common"
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
		fmt.Printf("   {Index: \" %v \", Address: \"%v\", BLSPublicKey: \"%v\"},\n", index, one, bls[i])
		index++
	}
}

func TestCommitteeAccounts(test *testing.T) {
	testAccounts(test, FoundationalNodeAccounts)
	testAccounts(test, FoundationalNodeAccountsV1)
	testAccounts(test, FoundationalNodeAccountsV1_1)
	testAccounts(test, FoundationalNodeAccountsV1_2)
	testAccounts(test, FoundationalNodeAccountsV1_3)
	testAccounts(test, FoundationalNodeAccountsV1_4)
	testAccounts(test, FoundationalNodeAccountsV1_5)
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
		err = pubKey.DeserializeHexStr(account.BLSPublicKey)
		if err != nil {
			test.Error("Account bls public key", account.BLSPublicKey, "is not valid:", err)
		}
	}
}

func testDeployAccounts(t *testing.T, accounts []DeployAccount) {
	indicesByAddress := make(map[ethCommon.Address][]int)
	indicesByKey := make(map[string][]int)
	for index, account := range accounts {
		if strings.TrimSpace(account.Index) != strconv.Itoa(index) {
			t.Errorf("account %+v at index %v has wrong index string",
				account, index)
		}
		if address, err := common.Bech32ToAddress(account.Address); err != nil {
			t.Errorf("account %+v at index %v has invalid address (%s)",
				account, index, err)
		} else {
			indicesByAddress[address] = append(indicesByAddress[address], index)
		}
		pubKey := bls.PublicKey{}
		if err := pubKey.DeserializeHexStr(account.BLSPublicKey); err != nil {
			t.Errorf("account %+v at index %v has invalid public key (%s)",
				account, index, err)
		} else {
			pubKeyStr := pubKey.SerializeToHexStr()
			indicesByKey[pubKeyStr] = append(indicesByKey[pubKeyStr], index)
		}
	}
	for address, indices := range indicesByAddress {
		if len(indices) > 1 {
			t.Errorf("account address %s appears in multiple rows: %v",
				common.MustAddressToBech32(address), indices)
		}
	}
	for pubKey, indices := range indicesByKey {
		if len(indices) > 1 {
			t.Errorf("BLS public key %s appears in multiple rows: %v",
				pubKey, indices)
		}
	}
}
