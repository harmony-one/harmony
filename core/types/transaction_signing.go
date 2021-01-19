// Copyright 2016 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package types

import (
	"crypto/ecdsa"
	"errors"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"

	"github.com/harmony-one/harmony/crypto/hash"
	"github.com/harmony-one/harmony/internal/params"
)

// Constants for transaction signing.
var (
	ErrInvalidChainID = errors.New("invalid chain id for signer")
)

// TxType - the type of transaction - Harmony or Ethereum.
const (
	HarmonyTx TransactionType = iota
	EthereumTx
)

// sigCache is used to cache the derived sender and contains
// the signer used to derive it.
type sigCache struct {
	signer Signer
	from   common.Address
}

// MakeSigner returns a Signer based on the given chain config and epoch number.
func MakeSigner(config *params.ChainConfig, epochNumber *big.Int) Signer {
	var signer Signer
	switch {
	case config.IsEIP155(epochNumber):
		signer = NewEIP155Signer(config.ChainID)
	default:
		signer = FrontierSigner{}
	}
	return signer
}

// SignTx signs the Harmony transaction using the given signer and private key
func SignTx(tx *Transaction, s Signer, prv *ecdsa.PrivateKey) (*Transaction, error) {
	return SignTxForTxType(tx, HarmonyTx, s, prv)
}

// SignEthTx signs the Ethereum transaction using the given signer and private key
func SignEthTx(tx *Transaction, s Signer, prv *ecdsa.PrivateKey) (*Transaction, error) {
	return SignTxForTxType(tx, EthereumTx, s, prv)
}

// SignTxForTxType signs the Harmony or Ethereum transaction using the given signer and private key
func SignTxForTxType(tx *Transaction, txType TransactionType, s Signer, prv *ecdsa.PrivateKey) (*Transaction, error) {
	h := s.GenerateHash(tx, txType)
	sig, err := crypto.Sign(h[:], prv)
	if err != nil {
		return nil, err
	}
	return tx.WithSignature(s, sig)
}

// Sender returns the address for a Harmony tranasction derived from the signature (V, R, S) using secp256k1
// elliptic curve and an error if it failed deriving or upon an incorrect
// signature.
//
// Sender may cache the address, allowing it to be used regardless of
// signing method. The cache is invalidated if the cached signer does
// not match the signer used in the current call.
func Sender(signer Signer, tx TransactionInterface) (common.Address, error) {
	return DeriveSender(signer, tx, HarmonyTx)
}

// EthSender returns the address for an Ethereum transaction derived from the signature (V, R, S) using secp256k1
// elliptic curve and an error if it failed deriving or upon an incorrect
// signature.
//
// Sender may cache the address, allowing it to be used regardless of
// signing method. The cache is invalidated if the cached signer does
// not match the signer used in the current call.
func EthSender(signer Signer, tx TransactionInterface) (common.Address, error) {
	return DeriveSender(signer, tx, EthereumTx)
}

// DeriveSender returns the address for either a Harmony or an Ethereum transaction derived from the signature (V, R, S) using secp256k1
// elliptic curve and an error if it failed deriving or upon an incorrect
// signature.
//
// Sender may cache the address, allowing it to be used regardless of
// signing method. The cache is invalidated if the cached signer does
// not match the signer used in the current call.
func DeriveSender(signer Signer, tx TransactionInterface, txType TransactionType) (common.Address, error) {
	if sc := tx.From().Load(); sc != nil {
		sigCache := sc.(sigCache)
		// If the signer used to derive from in a previous
		// call is not the same as used current, invalidate
		// the cache.
		if sigCache.signer.Equal(signer) {
			return sigCache.from, nil
		}
	}

	addr, err := signer.DeriveSender(tx, txType)
	if err != nil {
		return common.Address{}, err
	}
	tx.From().Store(sigCache{signer: signer, from: addr})
	return addr, nil
}

// Signer encapsulates transaction signature handling. Note that this interface is not a
// stable API and may change at any time to accommodate new protocol rules.
type Signer interface {
	// Sender returns the sender address of the Harmony transaction.
	Sender(tx TransactionInterface) (common.Address, error)
	// EthSender returns the sender address of the Ethereum transaction.
	EthSender(tx TransactionInterface) (common.Address, error)
	// DeriveSender returns the sender address of the Harmony or Ethereum transaction.
	DeriveSender(tx TransactionInterface, txType TransactionType) (common.Address, error)
	// SignatureValues returns the raw R, S, V values corresponding to the
	// given signature.
	SignatureValues(tx TransactionInterface, sig []byte) (r, s, v *big.Int, err error)
	// Hash returns the Harmony hash to be signed.
	Hash(tx TransactionInterface) common.Hash
	// EthHash returns the Ethereum hash to be signed.
	EthHash(tx TransactionInterface) common.Hash
	// Equal returns true if the given signer is the same as the receiver.
	GenerateHash(tx TransactionInterface, txType TransactionType) common.Hash
	// Equal returns true if the given signer is the same as the receiver.
	Equal(Signer) bool
}

// EIP155Signer implements Signer using the EIP155 rules.
type EIP155Signer struct {
	chainID, chainIDMul *big.Int
}

// NewEIP155Signer creates a EIP155Signer given chainID.
func NewEIP155Signer(chainID *big.Int) EIP155Signer {
	if chainID == nil {
		chainID = new(big.Int)
	}
	return EIP155Signer{
		chainID:    chainID,
		chainIDMul: new(big.Int).Mul(chainID, big.NewInt(2)),
	}
}

// Equal checks if the given EIP155Signer is equal to another Signer.
func (s EIP155Signer) Equal(s2 Signer) bool {
	eip155, ok := s2.(EIP155Signer)
	return ok && eip155.chainID.Cmp(s.chainID) == 0
}

var big8 = big.NewInt(8)

// Sender returns the sender address of the given Harmony signer.
func (s EIP155Signer) Sender(tx TransactionInterface) (common.Address, error) {
	return s.DeriveSender(tx, HarmonyTx)
}

// EthSender returns the sender address of the given Ethereum signer.
func (s EIP155Signer) EthSender(tx TransactionInterface) (common.Address, error) {
	return s.DeriveSender(tx, EthereumTx)
}

// DeriveSender returns the sender address of the given Harmony or Ethereum signer.
func (s EIP155Signer) DeriveSender(tx TransactionInterface, txType TransactionType) (common.Address, error) {
	if !tx.Protected() {
		return HomesteadSigner{}.DeriveSender(tx, txType)
	}
	if tx.ChainID().Cmp(s.chainID) != 0 {
		return common.Address{}, ErrInvalidChainID
	}
	V := new(big.Int).Sub(tx.V(), s.chainIDMul)
	V.Sub(V, big8)
	return recoverPlain(s.GenerateHash(tx, txType), tx.R(), tx.S(), V, true)
}

// SignatureValues returns signature values. This signature
// needs to be in the [R || S || V] format where V is 0 or 1.
func (s EIP155Signer) SignatureValues(tx TransactionInterface, sig []byte) (R, S, V *big.Int, err error) {
	R, S, V, err = HomesteadSigner{}.SignatureValues(tx, sig)
	if err != nil {
		return nil, nil, nil, err
	}
	if s.chainID.Sign() != 0 {
		V = big.NewInt(int64(sig[64] + 35))
		V.Add(V, s.chainIDMul)
	}
	return R, S, V, nil
}

// Hash returns the Harmony hash to be signed by the sender.
// It does not uniquely identify the transaction.
func (s EIP155Signer) Hash(tx TransactionInterface) common.Hash {
	return generateHash(tx, HarmonyTx, s.chainID)
}

// EthHash returns the Ethereum hash to be signed by the sender.
// It does not uniquely identify the transaction.
func (s EIP155Signer) EthHash(tx TransactionInterface) common.Hash {
	return generateHash(tx, EthereumTx, s.chainID)
}

// GenerateHash returns the Harmony or Ethereum hash to be signed by the sender.
// It does not uniquely identify the transaction.
func (s EIP155Signer) GenerateHash(tx TransactionInterface, txType TransactionType) common.Hash {
	return generateHash(tx, txType, s.chainID)
}

// HomesteadSigner implements TransactionInterface using the
// homestead rules.
type HomesteadSigner struct{ FrontierSigner }

// Equal checks if it is equal to s2 signer.
func (hs HomesteadSigner) Equal(s2 Signer) bool {
	_, ok := s2.(HomesteadSigner)
	return ok
}

// SignatureValues returns signature values. This signature
// needs to be in the [R || S || V] format where V is 0 or 1.
func (hs HomesteadSigner) SignatureValues(tx TransactionInterface, sig []byte) (r, s, v *big.Int, err error) {
	return hs.FrontierSigner.SignatureValues(tx, sig)
}

// Sender returns the address of the Harmony sender.
func (hs HomesteadSigner) Sender(tx TransactionInterface) (common.Address, error) {
	return hs.DeriveSender(tx, HarmonyTx)
}

// EthSender returns the address of the Ethereum sender.
func (hs HomesteadSigner) EthSender(tx TransactionInterface) (common.Address, error) {
	return hs.DeriveSender(tx, EthereumTx)
}

// DeriveSender returns the sender address of the given Harmony or Ethereum signer.
func (hs HomesteadSigner) DeriveSender(tx TransactionInterface, txType TransactionType) (common.Address, error) {
	return recoverPlain(hs.GenerateHash(tx, txType), tx.R(), tx.S(), tx.V(), true)
}

// FrontierSigner ...
type FrontierSigner struct{}

// Equal checks if the s2 signer is equal to the given signer.
func (fs FrontierSigner) Equal(s2 Signer) bool {
	_, ok := s2.(FrontierSigner)
	return ok
}

// SignatureValues returns signature values. This signature
// needs to be in the [R || S || V] format where V is 0 or 1.
func (fs FrontierSigner) SignatureValues(tx TransactionInterface, sig []byte) (r, s, v *big.Int, err error) {
	if len(sig) != 65 {
		panic(fmt.Sprintf("wrong size for signature: got %d, want 65", len(sig)))
	}
	r = new(big.Int).SetBytes(sig[:32])
	s = new(big.Int).SetBytes(sig[32:64])
	v = new(big.Int).SetBytes([]byte{sig[64] + 27})
	return r, s, v, nil
}

// Hash returns the Harmony hash to be signed by the sender.
// It does not uniquely identify the transaction.
func (fs FrontierSigner) Hash(tx TransactionInterface) common.Hash {
	return generateHash(tx, HarmonyTx, nil)
}

// EthHash returns the Ethereum hash to be signed by the sender.
// It does not uniquely identify the transaction.
func (fs FrontierSigner) EthHash(tx TransactionInterface) common.Hash {
	return generateHash(tx, EthereumTx, nil)
}

// GenerateHash returns the Harmony or Ethereum hash to be signed by the sender.
// It does not uniquely identify the transaction.
func (fs FrontierSigner) GenerateHash(tx TransactionInterface, txType TransactionType) common.Hash {
	return generateHash(tx, txType, nil)
}

// Sender returns the sender address of the given Harmony transaction.
func (fs FrontierSigner) Sender(tx TransactionInterface) (common.Address, error) {
	return fs.DeriveSender(tx, HarmonyTx)
}

// EthSender returns the sender address of the given Ethereum transaction.
func (fs FrontierSigner) EthSender(tx TransactionInterface) (common.Address, error) {
	return fs.DeriveSender(tx, EthereumTx)
}

// DeriveSender returns the sender address of the given Harmony or Ethereum transaction.
func (fs FrontierSigner) DeriveSender(tx TransactionInterface, txType TransactionType) (common.Address, error) {
	return recoverPlain(fs.GenerateHash(tx, txType), tx.R(), tx.S(), tx.V(), false)
}

func recoverPlain(sighash common.Hash, R, S, Vb *big.Int, homestead bool) (common.Address, error) {
	if Vb.BitLen() > 8 {
		return common.Address{}, ErrInvalidSig
	}
	V := byte(Vb.Uint64() - 27)
	if !crypto.ValidateSignatureValues(V, R, S, homestead) {
		return common.Address{}, ErrInvalidSig
	}
	// encode the signature in uncompressed format
	r, s := R.Bytes(), S.Bytes()
	sig := make([]byte, 65)
	copy(sig[32-len(r):32], r)
	copy(sig[64-len(s):64], s)
	sig[64] = V
	// recover the public key from the signature
	pub, err := crypto.Ecrecover(sighash[:], sig)
	if err != nil {
		return common.Address{}, err
	}
	if len(pub) == 0 || pub[0] != 4 {
		return common.Address{}, errors.New("invalid public key")
	}
	var addr common.Address
	copy(addr[:], crypto.Keccak256(pub[1:])[12:])
	return addr, nil
}

// deriveChainID derives the chain id from the given v parameter
func deriveChainID(v *big.Int) *big.Int {
	if v.BitLen() <= 64 {
		v := v.Uint64()
		if v == 27 || v == 28 {
			return new(big.Int)
		}
		return new(big.Int).SetUint64((v - 35) / 2)
	}
	v = new(big.Int).Sub(v, big.NewInt(35))
	return v.Div(v, big.NewInt(2))
}

func generateHash(tx TransactionInterface, txType TransactionType, chainID *big.Int) common.Hash {
	var hashData []interface{}

	hashData = append(hashData, tx.Nonce())
	hashData = append(hashData, tx.Price())
	hashData = append(hashData, tx.GasLimit())

	if txType == HarmonyTx {
		hashData = append(hashData, tx.ShardID())
		hashData = append(hashData, tx.ToShardID())
	}

	hashData = append(hashData, tx.Recipient())
	hashData = append(hashData, tx.Amount())
	hashData = append(hashData, tx.Payload())

	if chainID != nil {
		hashData = append(hashData, chainID)
		hashData = append(hashData, uint(0))
		hashData = append(hashData, uint(0))
	}

	return hash.FromRLP(hashData)
}
