package node

import (
	"crypto/ecdsa"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"
	"github.com/harmony-one/harmony/core"
	"math/big"
	"math/rand"
	"strings"
)

// CreateGenesisAllocWithTestingAddresses create the genesis block allocation that contains deterministically
// generated testing addressess with tokens.
// TODO: Consider to remove it later when moving to production.a
func (node *Node) CreateGenesisAllocWithTestingAddresses(numAddress int) core.GenesisAlloc {
	rand.Seed(0)
	len := 1000000
	bytes := make([]byte, len)
	for i := 0; i < len; i++ {
		bytes[i] = byte(rand.Intn(100))
	}
	reader := strings.NewReader(string(bytes))
	genesisAloc := make(core.GenesisAlloc)
	for i := 0; i < numAddress; i++ {
		testBankKey, _ := ecdsa.GenerateKey(crypto.S256(), reader)
		testBankAddress := crypto.PubkeyToAddress(testBankKey.PublicKey)
		testBankFunds := big.NewInt(1000)
		testBankFunds = testBankFunds.Mul(testBankFunds, big.NewInt(params.Ether))
		genesisAloc[testBankAddress] = core.GenesisAccount{Balance: testBankFunds}
		node.TestBankKeys = append(node.TestBankKeys, testBankKey)
	}
	return genesisAloc
}
