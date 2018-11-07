package blockchain

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"log"
	"math"

	"github.com/dedis/kyber"
	"github.com/dedis/kyber/sign/schnorr"
	"github.com/simple-rules/harmony-benchmark/crypto"
)

var (
	// zeroHash is the zero value for a Hash and is defined as
	// a package level variable to avoid the need to create a new instance
	// every time a check is needed.
	zeroHash TxID
)

const (
	// DefaultCoinbaseValue is the default value of coinbase transaction.
	DefaultCoinbaseValue = 1
	// DefaultNumUtxos is the default value of number Utxos.
	DefaultNumUtxos = 100
)

// Transaction is the struct of a Transaction.
type Transaction struct {
	ID        [32]byte // 32 byte hash
	TxInput   []TXInput
	TxOutput  []TXOutput
	PublicKey [32]byte
	Signature [64]byte

	Proofs []CrossShardTxProof // The proofs for crossShard tx unlock-to-commit/abort
}

// TXOutput is the struct of transaction output in a transaction.
type TXOutput struct {
	Amount  int      // TODO: Switch to big int or uint32
	Address [20]byte // last 20 bytes of the hash of public key
	ShardID uint32   // The Id of the shard where this UTXO belongs
}

// TxID structure type.
type TxID = [32]byte

// OutPoint defines a data type that is used to track previous
// transaction outputs.
// TxID is the transaction id
// Index is the index of the transaction ouput in the previous transaction
type OutPoint struct {
	TxID  TxID
	Index uint32
}

// NewOutPoint returns a new transaction outpoint point with the
// provided txID and index.
func NewOutPoint(txID *TxID, index uint32) *OutPoint {
	return &OutPoint{
		TxID:  *txID,
		Index: index,
	}
}

// TXInput is the struct of transaction input (a UTXO) in a transaction.
type TXInput struct {
	PreviousOutPoint OutPoint
	Address          [20]byte // TODO: @minh do we really need this?
	ShardID          uint32   // The Id of the shard where this UTXO belongs
}

// NewTXInput returns a new transaction input with the provided
// previous outpoint point, output address and shardID
func NewTXInput(prevOut *OutPoint, address [20]byte, shardID uint32) *TXInput {
	return &TXInput{
		PreviousOutPoint: *prevOut,
		Address:          address,
		ShardID:          shardID,
	}
}

// CrossShardTxProof is the proof of accept or reject in the cross shard transaction locking phase.
// This is created by the shard leader, filled with proof signatures after consensus, and returned back to the client.
// One proof structure is only tied to one shard. Therefore, the utxos in the proof are all with the same shard.
type CrossShardTxProof struct {
	Accept    bool      // false means proof-of-reject, true means proof-of-accept
	TxID      [32]byte  // Id of the transaction which this proof is on
	TxInput   []TXInput // The list of Utxo that this proof is on. They should be in the same shard.
	BlockHash [32]byte  // The hash of the block where the proof is registered
	// Signatures
}

// CrossShardTxAndProof is the proof of accept or reject in the cross shard transaction locking phase.
// This is a internal data structure that doesn't go across network
type CrossShardTxAndProof struct {
	Transaction *Transaction       // The cross shard tx
	Proof       *CrossShardTxProof // The proof
}

// SetID sets ID of a transaction (32 byte hash of the whole transaction)
func (tx *Transaction) SetID() {
	var encoded bytes.Buffer
	var hash [32]byte

	enc := gob.NewEncoder(&encoded)
	err := enc.Encode(tx)
	if err != nil {
		log.Panic(err)
	}
	hash = sha256.Sum256(encoded.Bytes())
	tx.ID = hash
}

// Sign signs the given transaction with a private key.
func (tx *Transaction) Sign(priKey kyber.Scalar) error {
	signature, err := schnorr.Sign(crypto.Ed25519Curve, priKey, tx.GetContentToVerify())
	if err != nil {
		log.Panic(err)
	}

	copy(tx.Signature[:], signature)
	return err
}

// IsCrossShard returns if the transaction is a cross transation.
func (tx *Transaction) IsCrossShard() bool {
	shardIDs := make(map[uint32]bool)
	for _, value := range tx.TxInput {
		shardIDs[value.ShardID] = true
	}
	for _, value := range tx.TxOutput {
		shardIDs[value.ShardID] = true
	}
	return len(shardIDs) > 1
}

// GetContentToVerify gets content to verify.
func (tx *Transaction) GetContentToVerify() []byte {
	tempTx := *tx
	tempTx.Signature = [64]byte{}
	tempTx.Proofs = []CrossShardTxProof{}

	return tempTx.Serialize()
}

// NewCoinbaseTX creates a new coinbase transaction
func NewCoinbaseTX(toAddress [20]byte, data string, shardID uint32) *Transaction {
	if data == "" {
		data = fmt.Sprintf("Reward to '%b'", toAddress)
	}

	txin := NewTXInput(NewOutPoint(&TxID{}, math.MaxUint32), toAddress, shardID)
	outputs := []TXOutput{}
	for i := 0; i < DefaultNumUtxos; i++ {
		outputs = append(outputs, TXOutput{DefaultCoinbaseValue, toAddress, shardID})
	}
	tx := Transaction{ID: [32]byte{}, TxInput: []TXInput{*txin}, TxOutput: outputs, Proofs: nil}
	// TODO: take care of the signature of coinbase transaction.
	tx.SetID()
	return &tx
}

// Used for debuging.
func (txInput *TXInput) String() string {
	res := fmt.Sprintf("TxID: %v, ", hex.EncodeToString(txInput.PreviousOutPoint.TxID[:]))
	res += fmt.Sprintf("TxOutputIndex: %v, ", txInput.PreviousOutPoint.Index)
	res += fmt.Sprintf("Address: %v, ", txInput.Address)
	res += fmt.Sprintf("ShardID: %v", txInput.ShardID)
	return res
}

// Used for debuging.
func (txOutput *TXOutput) String() string {
	res := fmt.Sprintf("Amount: %v, ", txOutput.Amount)
	res += fmt.Sprintf("Address: %v", txOutput.Address)
	res += fmt.Sprintf("ShardID: %v", txOutput.ShardID)
	return res
}

// Used for debuging.
func (proof *CrossShardTxProof) String() string {
	res := fmt.Sprintf("Accept: %v, ", proof.Accept)
	res += fmt.Sprintf("TxId: %v, ", hex.EncodeToString(proof.TxID[:]))
	res += fmt.Sprintf("BlockHash: %v, ", hex.EncodeToString(proof.BlockHash[:]))
	res += fmt.Sprintf("TxInput:\n")
	for id, value := range proof.TxInput {
		res += fmt.Sprintf("%v: %v\n", id, value.String())
	}
	return res
}

// Used for debuging.
func (tx *Transaction) String() string {
	res := fmt.Sprintf("ID: %v\n", hex.EncodeToString(tx.ID[:]))
	res += fmt.Sprintf("TxInput:\n")
	for id, value := range tx.TxInput {
		res += fmt.Sprintf("%v: %v\n", id, value.String())
	}
	res += fmt.Sprintf("TxOutput:\n")
	for id, value := range tx.TxOutput {
		res += fmt.Sprintf("%v: %v\n", id, value.String())
	}
	for id, value := range tx.Proofs {
		res += fmt.Sprintf("Proof:\n")
		res += fmt.Sprintf("%v: %v\n", id, value.String())
	}
	res += fmt.Sprintf("PublicKey: %v\n", hex.EncodeToString(tx.PublicKey[:]))
	res += fmt.Sprintf("Sig: %v\n", hex.EncodeToString(tx.Signature[:]))
	return res
}

// Serialize return serialized bytes of the transaction.
func (tx *Transaction) Serialize() []byte {
	buffer := bytes.NewBuffer([]byte{})
	buffer.Write(tx.ID[:])
	for _, value := range tx.TxInput {
		buffer.Write(value.Serialize())
	}
	for _, value := range tx.TxOutput {
		buffer.Write(value.Serialize())
	}
	for _, value := range tx.Proofs {
		buffer.Write(value.Serialize())
	}
	buffer.Write(tx.PublicKey[:])
	buffer.Write(tx.Signature[:])
	return buffer.Bytes()
}

// Serialize return serialized bytes of the TXInput.
func (txInput *TXInput) Serialize() []byte {
	buffer := bytes.NewBuffer([]byte{})
	buffer.Write(txInput.Address[:])

	fourBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(fourBytes, txInput.ShardID)
	buffer.Write(fourBytes)

	binary.BigEndian.PutUint32(fourBytes, txInput.PreviousOutPoint.Index)
	buffer.Write(fourBytes)

	buffer.Write(txInput.PreviousOutPoint.TxID[:])
	return buffer.Bytes()
}

// Serialize return serialized bytes of the TXOutput.
func (txOutput *TXOutput) Serialize() []byte {
	buffer := bytes.NewBuffer([]byte{})
	buffer.Write(txOutput.Address[:])

	fourBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(fourBytes, txOutput.ShardID)
	buffer.Write(fourBytes)

	binary.BigEndian.PutUint32(fourBytes, uint32(txOutput.Amount)) // TODO(RJ): make amount a bigInt
	buffer.Write(fourBytes)

	return buffer.Bytes()
}

// Serialize returns serialized bytes of the CrossShardTxProof.
func (proof *CrossShardTxProof) Serialize() []byte {
	buffer := bytes.NewBuffer([]byte{})
	buffer.Write(proof.TxID[:])
	buffer.Write(proof.BlockHash[:])
	for _, value := range proof.TxInput {
		buffer.Write(value.Serialize())
	}
	if proof.Accept {
		buffer.WriteByte(byte(1))
	} else {
		buffer.WriteByte(byte(0))
	}
	return buffer.Bytes()
}
