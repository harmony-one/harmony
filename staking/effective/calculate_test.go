package effective

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/big"
	"math/rand"
	"sort"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/numeric"
	"github.com/harmony-one/harmony/shard"
)

const eposTestingFile = "epos.json"

var (
	testingNumber    = 20
	testingSlots     slotsData
	testingPurchases []SlotPurchase
	expectedMedian   numeric.Dec
	maxAccountGen    = int64(98765654323123134)
	accountGen       = rand.New(rand.NewSource(1337))
	maxKeyGen        = int64(98765654323123134)
	keyGen           = rand.New(rand.NewSource(42))
	maxStakeGen      = int64(200)
	stakeGen         = rand.New(rand.NewSource(541))
)

type slotsData struct {
	EPOSedSlot []string `json:"slots"`
}

func init() {
	input, err := ioutil.ReadFile(eposTestingFile)
	if err != nil {
		panic(
			fmt.Sprintf("cannot open genesisblock config file %v, err %v\n",
				eposTestingFile,
				err,
			))
	}

	t := slotsData{}
	oops := json.Unmarshal(input, &t)
	if oops != nil {
		fmt.Println(oops.Error())
		panic("Could not unmarshal slots data into memory")
	}
	testingPurchases = generateRandomSlots(testingNumber)
}

func generateRandomSlots(num int) []SlotPurchase {
	randomSlots := []SlotPurchase{}
	for i := 0; i < num; i++ {
		addr := common.Address{}
		addr.SetBytes(big.NewInt(int64(accountGen.Int63n(maxAccountGen))).Bytes())
		secretKey := bls.SecretKey{}
		secretKey.Deserialize(big.NewInt(int64(keyGen.Int63n(maxKeyGen))).Bytes())
		key := shard.BlsPublicKey{}
		key.FromLibBLSPublicKey(secretKey.GetPublicKey())
		stake := numeric.NewDecFromBigInt(big.NewInt(int64(stakeGen.Int63n(maxStakeGen))))
		randomSlots = append(randomSlots, SlotPurchase{addr, key, stake})
	}
	return randomSlots
}

func TestMedian(t *testing.T) {
	copyPurchases := append([]SlotPurchase{}, testingPurchases...)
	sort.SliceStable(copyPurchases,
		func(i, j int) bool {
			return copyPurchases[i].Stake.LTE(copyPurchases[j].Stake)
		})
	numPurchases := len(copyPurchases) / 2
	if len(copyPurchases)%2 == 0 {
		expectedMedian = copyPurchases[numPurchases-1].Stake.Add(
			copyPurchases[numPurchases].Stake,
		).Quo(two)
	} else {
		expectedMedian = copyPurchases[numPurchases].Stake
	}
	med := Median(testingPurchases)
	if !med.Equal(expectedMedian) {
		t.Errorf("Expected: %s, Got: %s", expectedMedian.String(), med.String())
	}
}

func TestEffectiveStake(t *testing.T) {
	for _, val := range testingPurchases {
		expectedStake := numeric.MaxDec(
			numeric.MinDec(numeric.OneDec().Add(c).Mul(expectedMedian), val.Stake),
			numeric.OneDec().Sub(c).Mul(expectedMedian))
		calculatedStake := effectiveStake(expectedMedian, val.Stake)
		if !expectedStake.Equal(calculatedStake) {
			t.Errorf(
				"Expected: %s, Got: %s", expectedStake.String(), calculatedStake.String(),
			)
		}
	}
}

func TestApply(t *testing.T) {
	//
}
