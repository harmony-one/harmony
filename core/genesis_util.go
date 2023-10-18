package core

import (
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"sort"

	"github.com/ethereum/go-ethereum/rlp"
)

// GenesisItem represents one genesis block transaction
type GenesisItem struct {
	Addr    *big.Int
	Balance *big.Int
}

// genesisConfig is a format compatible with genesis_block.json file
type genesisConfig map[string]map[string]string

func encodeGenesisConfig(ga []GenesisItem) string {
	by, err := rlp.EncodeToBytes(ga)
	if err != nil {
		panic("ops")
	}

	return string(by)
}

func parseGenesisConfigFile(fileName string) genesisConfig {
	input, err := os.ReadFile(fileName)
	if err != nil {
		panic(fmt.Sprintf("cannot open genesisblock config file %v, err %v\n", fileName, err))
	}
	var gc genesisConfig
	err = json.Unmarshal(input, &gc)
	if err != nil {
		panic(fmt.Sprintf("cannot parse json file %v, err %v\n", fileName, err))
	}
	return gc
}

// StringToBigInt converts a string to BigInt
func StringToBigInt(s string, base int) *big.Int {
	z := new(big.Int)
	z, ok := z.SetString(s, base)
	if !ok {
		panic(fmt.Sprintf("%v cannot convert to bigint with base %v", s, base))
	}
	return z
}

func convertToGenesisItems(gc genesisConfig) []GenesisItem {
	gi := []GenesisItem{}
	for k, v := range gc {
		gi = append(gi, GenesisItem{StringToBigInt(k, 16), StringToBigInt(v["nano"], 10)})
	}
	sort.Slice(gi, func(i, j int) bool {
		return gi[i].Addr.Cmp(gi[j].Addr) == -1
	})
	return gi
}

// EncodeGenesisConfig converts json file into binary format for genesis block
func EncodeGenesisConfig(fileName string) string {
	gc := parseGenesisConfigFile(fileName)
	gi := convertToGenesisItems(gc)
	by := encodeGenesisConfig(gi)
	return string(by)
}
