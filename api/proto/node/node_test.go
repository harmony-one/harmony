package node

import (
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/params"
	"github.com/harmony-one/harmony/core/state"
	"github.com/harmony-one/harmony/core/types"
)

var (
	senderPriKey, _   = crypto.GenerateKey()
	receiverPriKey, _ = crypto.GenerateKey()
	receiverAddress   = crypto.PubkeyToAddress(receiverPriKey.PublicKey)

	amountBigInt = big.NewInt(8000000000000000000)
)

func TestSerializeBlockchainSyncMessage(t *testing.T) {
	h1 := common.HexToHash("123")
	h2 := common.HexToHash("abc")

	hash := [][]byte{
		h1.Bytes(),
		h2.Bytes(),
	}

	msg := BlockchainSyncMessage{
		BlockHeight: 2,
		BlockHashes: hash,
	}

	serializedByte := SerializeBlockchainSyncMessage(&msg)
	dMsg, err := DeserializeBlockchainSyncMessage(serializedByte)

	if err != nil || strings.Compare(msg.String(), dMsg.String()) != 0 {
		t.Errorf("Failed to serialize/deserialize blockchain sync message\n")
	}
}

func TestConstructTransactionListMessageAccount(t *testing.T) {
	tx, _ := types.SignTx(types.NewTransaction(100, receiverAddress, uint32(0), amountBigInt, params.TxGas, nil, nil), types.HomesteadSigner{}, senderPriKey)
	transactions := types.Transactions{tx}
	buf := ConstructTransactionListMessageAccount(transactions)
	if len(buf) == 0 {
		t.Error("Failed to contruct transaction list message")
	}
}

func TestConstructRequestTransactionsMessage(t *testing.T) {
	txIDs := [][]byte{
		{1, 2},
		{3, 4},
	}

	buf := ConstructRequestTransactionsMessage(txIDs)

	if len(buf) == 0 {
		t.Error("Failed to contruct request transaction message")
	}
}

func TestConstructStopMessage(t *testing.T) {
	buf := ConstructStopMessage()

	if len(buf) == 0 {
		t.Error("Failed to contruct STOP message")
	}
}

func TestConstructBlocksSyncMessage(t *testing.T) {

	db := ethdb.NewMemDatabase()
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(db))

	root := statedb.IntermediateRoot(false)
	head := &types.Header{
		Number:  new(big.Int).SetUint64(uint64(10000)),
		Nonce:   types.EncodeNonce(uint64(10000)),
		ShardID: types.EncodeShardID(uint32(0)),
		Time:    new(big.Int).SetUint64(uint64(100000)),
		Root:    root,
	}
	head.GasLimit = 10000000000
	head.Difficulty = params.GenesisDifficulty

	statedb.Commit(false)
	statedb.Database().TrieDB().Commit(root, true)

	block1 := types.NewBlock(head, nil, nil)

	blocks := []*types.Block{
		block1,
	}

	buf := ConstructBlocksSyncMessage(blocks)

	if len(buf) == 0 {
		t.Error("Failed to contruct block sync message")
	}

}

func TestRoleTypeToString(t *testing.T) {
	validator := ValidatorRole
	client := ClientRole
	unknown := RoleType(3)

	if strings.Compare(validator.String(), "Validator") != 0 {
		t.Error("Validator role String mismatch")
	}
	if strings.Compare(client.String(), "Client") != 0 {
		t.Error("Validator role String mismatch")
	}
	if strings.Compare(unknown.String(), "Unknown") != 0 {
		t.Error("Validator role String mismatch")
	}
}

func TestInfoToString(t *testing.T) {
	info := Info{
		IP:     "127.0.0.1",
		Port:   "81",
		PeerID: "peer",
	}
	var expected string
	expected = `IP:"127.0.0.1" Port:"81" PeerID:"peer"`
	if strings.EqualFold(info.String(), expected) {
		t.Errorf("Info string mismatch: %s", info.String())
	}
}
