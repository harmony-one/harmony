package txgen

import (
	"math/big"
	"math/rand"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/node"
)

// Settings is the settings for TX generation.
type Settings struct {
	NumOfAddress      int
	CrossShard        bool
	MaxNumTxsPerBatch int
	CrossShardRatio   int
}

// GenerateSimulatedTransactionsAccount generates simulated transaction for account model.
func GenerateSimulatedTransactionsAccount(shardID int, dataNodes []*node.Node, setting Settings) (types.Transactions, types.Transactions) {
	_ = setting // TODO: take use of settings
	node := dataNodes[shardID]
	txs := make([]*types.Transaction, 100)
	for i := 0; i < 100; i++ {
		baseNonce := node.Worker.GetCurrentState().GetNonce(crypto.PubkeyToAddress(node.TestBankKeys[i].PublicKey))
		for j := 0; j < 1; j++ {
			randomUserAddress := crypto.PubkeyToAddress(node.TestBankKeys[rand.Intn(100)].PublicKey)
			randAmount := rand.Float32()
			tx, _ := types.SignTx(types.NewTransaction(baseNonce+uint64(j), randomUserAddress, uint32(shardID), big.NewInt(int64(params.Ether*randAmount)), params.TxGas, nil, nil), types.HomesteadSigner{}, node.TestBankKeys[i])
			txs[i*1+j] = tx
		}
	}
	return txs, nil
}
