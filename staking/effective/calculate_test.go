package effective

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"testing"
	"math/rand"
	"math/big"
	"bytes"
	"sort"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/numeric"
	"github.com/harmony-one/harmony/shard"
)

const eposTestingFile = "epos.json"
const slotTestingFile = "slots.json"

var (
	testingSlots slotsData
	testingPurchases Slots
	maxAccountGen = int64(98765654323123134)
	accountGen    = rand.New(rand.NewSource(1337))
	keyGen        = rand.New(rand.NewSource(42))
	maxStakeGen   = int64(200)
	stakeGen      = rand.New(rand.NewSource(541))
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

	// Generate random data, TODO maybe should test both even & odd number slots
	testingPurchases = generateRandomSlots(21)
}

func generateRandomSlots(num int) Slots {
	randomSlots := Slots{}
	for i := 0; i < num; i++ {
		addr := common.Address{}
		addr.SetBytes(big.NewInt(int64(accountGen.Int63n(maxAccountGen))).Bytes())
		key := shard.BlsPublicKey{}
		copy(key[:], randomKey())
		stake := numeric.NewDecFromBigInt(big.NewInt(int64(stakeGen.Int63n(maxStakeGen))))
		randomSlots = append(randomSlots, SlotPurchase{ addr, key, stake })
	}
	return randomSlots
}

// Generates random valid bytes for BlsPublicKey
func randomKey() []byte {
	buf := bytes.Buffer{}
	for i := 0; i < shard.PublicKeySizeInBytes; i++ {
		invalid := true
		newByte := 0
		for invalid {
			newByte = int(int64(48) + keyGen.Int63n(int64(43)))
			if newByte <= int(57) || newByte >= int(65) {
				invalid = false
			}
		}
		buf.Write([]byte{byte(newByte)})
	}
	return buf.Bytes()
}

func TestMedian(t *testing.T) {
	copyPurchases := append([]SlotPurchase{}, testingPurchases...)
	sort.SliceStable(copyPurchases,
									func(i, j int) bool { return copyPurchases[i].Dec.LTE(copyPurchases[j].Dec) })
	numPurchases := len(copyPurchases) / 2
	expectedResult := numeric.ZeroDec()
	if len(copyPurchases) % 2 == 0 {
		expectedResult = copyPurchases[numPurchases - 1].Dec.Add(copyPurchases[numPurchases].Dec).Quo(two)
	} else {
		expectedResult = copyPurchases[numPurchases].Dec
	}
	med := median(testingPurchases)
	if !med.Equal(expectedResult) {
		t.Error()
	}
}

func TestEffectiveStake(t *testing.T) {
	//
}
func TestApply(t *testing.T) {
	//
}
