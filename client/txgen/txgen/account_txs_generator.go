package txgen

import (
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/node"
	"math/big"
)

type TxGenSettings struct {
	NumOfAddress      int
	CrossShard        bool
	MaxNumTxsPerBatch int
	CrossShardRatio   int
}

func GenerateSimulatedTransactionsAccount(shardID int, dataNodes []*node.Node, setting TxGenSettings) (types.Transactions, types.Transactions) {
	_ = setting // TODO: take use of settings
	node := dataNodes[shardID]
	txs := make([]*types.Transaction, 1000)
	for i := 0; i < 100; i++ {
		baseNonce := node.Worker.GetCurrentState().GetNonce(crypto.PubkeyToAddress(node.TestBankKeys[i].PublicKey))
		node.Worker.UpdateCurrent()
		for j := 0; j < 10; j++ {
			randomUserKey, _ := crypto.GenerateKey()
			randomUserAddress := crypto.PubkeyToAddress(randomUserKey.PublicKey)
			tx, _ := types.SignTx(types.NewTransaction(baseNonce+uint64(j), randomUserAddress, big.NewInt(1000), params.TxGas, nil, nil), types.HomesteadSigner{}, node.TestBankKeys[i])
			txs[i*10+j] = tx
		}
	}
	return txs, nil
}
