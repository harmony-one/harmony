package effective

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"testing"
)

const eposTestingFile = "epos.json"

var (
	testingSlots slotsData
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
}

func TestMedian(t *testing.T) {
	//
}

func TestEffectiveStake(t *testing.T) {
	//
}
func TestApply(t *testing.T) {
	//
}
