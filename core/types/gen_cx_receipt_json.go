package types

import (
	"encoding/json"
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/block"
	internal_common "github.com/harmony-one/harmony/internal/common"
)

// MarshalJSON marshals as JSON.
func (r CXReceipt) MarshalJSON() ([]byte, error) {
	// CXReceipt represents a receipt for cross-shard transaction
	type CXReceipt struct {
		TxHash    common.Hash `json:"txHash"     gencodec:"required"`
		From      string      `json:"from"`
		To        string      `json:"to"`
		ShardID   uint32      `json:"shardID"`
		ToShardID uint32      `json:"toShardID"`
		Amount    *big.Int    `json:"amount"`
	}
	var enc CXReceipt
	enc.Amount = r.Amount
	enc.ShardID = r.ShardID
	enc.ToShardID = r.ToShardID
	enc.TxHash = r.TxHash
	address, err := internal_common.AddressToBech32(r.From)
	if err != nil {
		return nil, err
	}
	enc.From = address
	if r.To == nil {
		address = ""
	} else {
		address, err = internal_common.AddressToBech32(*r.To)
	}
	if err != nil {
		return nil, err
	}
	enc.To = address
	return json.Marshal(&enc)
}

// UnmarshalJSON unmarshals from JSON.
func (r *CXReceipt) UnmarshalJSON(input []byte) error {
	type CXReceipt struct {
		TxHash    *common.Hash `json:"txHash"     gencodec:"required"`
		From      string       `json:"from"`
		To        string       `json:"to"`
		ShardID   uint32       `json:"shardID"`
		ToShardID uint32       `json:"toShardID"`
		Amount    *big.Int     `json:"amount"`
	}
	var dec CXReceipt
	var err error
	if err = json.Unmarshal(input, &dec); err != nil {
		return err
	}
	if dec.TxHash == nil {
		return errors.New("missing required field 'txHash' for CXReceipt")
	}
	r.TxHash = *dec.TxHash
	if dec.From == "" {
		return errors.New("missing required field 'from' for CXReceipt")
	}
	r.From, err = internal_common.Bech32ToAddress(dec.From)
	if err != nil {
		return err
	}
	r.ShardID = dec.ShardID
	to, err := internal_common.Bech32ToAddress(dec.To)
	if err != nil {
		return err
	}
	*r.To = to
	r.ToShardID = dec.ToShardID
	if dec.Amount != nil {
		r.Amount = dec.Amount
	}
	return nil
}

// MarshalJSON marshals as JSON.
func (r CXMerkleProof) MarshalJSON() ([]byte, error) {
	type CXMerkleProof struct {
		BlockNum      *big.Int      `json:"blockNum"`
		BlockHash     common.Hash   `json:"blockHash"   gencodec:"required"`
		ShardID       uint32        `json:"shardID"`
		CXReceiptHash common.Hash   `json:"receiptHash" gencodec:"required"`
		ShardIDs      []uint32      `json:"shardIDs"`
		CXShardHashes []common.Hash `json:"shardHashes" gencodec:"required"`
	}
	var enc CXMerkleProof
	enc.BlockNum = r.BlockNum
	enc.BlockHash = r.BlockHash
	enc.ShardID = r.ShardID
	enc.CXReceiptHash = r.CXReceiptHash
	enc.ShardIDs = r.ShardIDs
	enc.CXShardHashes = r.CXShardHashes
	return json.Marshal(&enc)
}

// UnmarshalJSON unmarshals from JSON.
func (r *CXMerkleProof) UnmarshalJSON(input []byte) error {
	type CXMerkleProof struct {
		BlockNum      *big.Int       `json:"blockNum"`
		BlockHash     *common.Hash   `json:"blockHash"   gencodec:"required"`
		ShardID       uint32         `json:"shardID"`
		CXReceiptHash *common.Hash   `json:"receiptHash" gencodec:"required"`
		ShardIDs      []uint32       `json:"shardIDs"`
		CXShardHashes []*common.Hash `json:"shardHashes" gencodec:"required"`
	}
	var dec CXMerkleProof
	if err := json.Unmarshal(input, &dec); err != nil {
		return err
	}
	if dec.BlockHash == nil {
		return errors.New("missing required field 'blockHash' for CXMerkleProof")
	}
	if dec.BlockNum == nil {
		return errors.New("missing required field 'blockNum' for CXMerkleProof")
	}
	if dec.CXReceiptHash == nil {
		return errors.New("missing required field 'cxReceiptHash' for CXMerkleProof")
	}
	r.BlockHash = *dec.BlockHash
	r.BlockNum = dec.BlockNum
	r.CXReceiptHash = *dec.CXReceiptHash
	r.ShardID = dec.ShardID
	r.ShardIDs = dec.ShardIDs
	for _, cxShardHash := range dec.CXShardHashes {
		r.CXShardHashes = append(r.CXShardHashes, *cxShardHash)
	}
	return nil
}

// MarshalJSON marshals as JSON.
func (r CXReceiptsProof) MarshalJSON() ([]byte, error) {
	type CXReceiptsProof struct {
		Receipts     []*CXReceipt   `json:"receipts"     gencodec:"required"`
		MerkleProof  *CXMerkleProof `json:"merkleProof"  gencodec:"required"`
		Header       *block.Header  `json:"header"       gencoded:"required"`
		CommitSig    []byte         `json:"commitSig"`
		CommitBitmap []byte         `json:"commitBitmap"`
	}
	var enc CXReceiptsProof
	enc.Receipts = r.Receipts
	enc.MerkleProof = r.MerkleProof
	enc.Header = r.Header
	enc.CommitSig = r.CommitSig
	enc.CommitBitmap = r.CommitBitmap
	return json.Marshal(&enc)
}

// UnmarshalJSON unmarshals from JSON.
func (r *CXReceiptsProof) UnmarshalJSON(input []byte) error {
	type CXReceiptsProof struct {
		Receipts     []*CXReceipt   `json:"receipts"     gencodec:"required"`
		MerkleProof  *CXMerkleProof `json:"merkleProof"  gencodec:"required"`
		Header       *block.Header  `json:"header"       gencoded:"required"`
		CommitSig    []byte         `json:"commitSig"`
		CommitBitmap []byte         `json:"commitBitmap"`
	}
	var dec CXReceiptsProof
	if err := json.Unmarshal(input, &dec); err != nil {
		return err
	}
	if dec.MerkleProof == nil {
		return errors.New("missing required field 'merkleProof' for CXReceiptsProof")
	}
	if dec.Header == nil {
		return errors.New("missing required field 'header' for CXReceipt")
	}
	r.MerkleProof = dec.MerkleProof
	r.Header = dec.Header
	r.Receipts = dec.Receipts
	r.CommitSig = dec.CommitSig
	r.CommitBitmap = dec.CommitBitmap
	return nil
}
