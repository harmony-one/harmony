package statecheck

import (
	"crypto/sha256"
	"encoding/binary"
	"hash"

	"github.com/ethereum/go-ethereum/common"
)

// DigestAlgorithm names the logical state digest construction below.
//
//	A_i       = SHA256("HMY-PF-ACCT-V1" || leafKey || BE64(nonce) ||
//	                   BE64(len(bal)) || bal || storageRoot || codeHash ||
//	                   H_storage || H_code)
//	H_storage = SHA256("HMY-PF-STOR-V1" || (slotKey || BE64(len(value)) || value)*)
//	            in storage-trie order, or 32 zero bytes for an empty root
//	H_code    = SHA256("HMY-PF-CODE-V1" || BE64(len(code)) || code),
//	            or 32 zero bytes for the empty code hash
//	digest    = SHA256("HMY-PF-STATE-V1" || stateRoot || A_1 || ... || A_n)
//	            in account-trie order
//
// bal is the minimal big-endian big.Int.Bytes(); value is the decoded
// (logical) storage byte string, making the digest invariant across
// physical database layouts. Identical digests across validators'
// attachments give coordinators a free cross-check; the digest is an
// informational receipt field, never a gate.
const DigestAlgorithm = "preflight_state_digest_v1"

var zeroHash32 [32]byte

func be64(n uint64) []byte {
	var b [8]byte
	binary.BigEndian.PutUint64(b[:], n)
	return b[:]
}

// storageDigest accumulates H_storage in storage-trie order.
type storageDigest struct {
	h     hash.Hash
	empty bool
}

func newStorageDigest(emptyRoot bool) *storageDigest {
	d := &storageDigest{empty: emptyRoot}
	if !emptyRoot {
		d.h = sha256.New()
		d.h.Write([]byte("HMY-PF-STOR-V1"))
	}
	return d
}

func (d *storageDigest) addLeaf(slotKey []byte, value []byte) {
	d.h.Write(slotKey)
	d.h.Write(be64(uint64(len(value))))
	d.h.Write(value)
}

func (d *storageDigest) sum() [32]byte {
	if d.empty {
		return zeroHash32
	}
	var out [32]byte
	copy(out[:], d.h.Sum(nil))
	return out
}

// codeDigest computes H_code (32 zero bytes for the empty code hash).
func codeDigest(code []byte, emptyCode bool) [32]byte {
	if emptyCode {
		return zeroHash32
	}
	h := sha256.New()
	h.Write([]byte("HMY-PF-CODE-V1"))
	h.Write(be64(uint64(len(code))))
	h.Write(code)
	var out [32]byte
	copy(out[:], h.Sum(nil))
	return out
}

// accountDigest computes A_i.
func accountDigest(leafKey []byte, nonce uint64, bal []byte, storageRoot common.Hash, codeHash []byte, hStorage, hCode [32]byte) [32]byte {
	h := sha256.New()
	h.Write([]byte("HMY-PF-ACCT-V1"))
	h.Write(leafKey)
	h.Write(be64(nonce))
	h.Write(be64(uint64(len(bal))))
	h.Write(bal)
	h.Write(storageRoot.Bytes())
	h.Write(codeHash)
	h.Write(hStorage[:])
	h.Write(hCode[:])
	var out [32]byte
	copy(out[:], h.Sum(nil))
	return out
}

// stateDigest folds A_i in account-trie order.
type stateDigest struct {
	h hash.Hash
}

func newStateDigest(stateRoot common.Hash) *stateDigest {
	d := &stateDigest{h: sha256.New()}
	d.h.Write([]byte("HMY-PF-STATE-V1"))
	d.h.Write(stateRoot.Bytes())
	return d
}

func (d *stateDigest) addAccount(a [32]byte) { d.h.Write(a[:]) }

func (d *stateDigest) sum() [32]byte {
	var out [32]byte
	copy(out[:], d.h.Sum(nil))
	return out
}
