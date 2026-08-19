package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/rlp"

	bls_core "github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/block"
	"github.com/harmony-one/harmony/consensus/signature"
	"github.com/harmony-one/harmony/core/types"
	bls_cosi "github.com/harmony-one/harmony/crypto/bls"
	shardingconfig "github.com/harmony-one/harmony/internal/configs/sharding"
	"github.com/harmony-one/harmony/internal/params"
	"github.com/harmony-one/harmony/internal/recovery/inplace/chainread"
	"github.com/harmony-one/harmony/internal/recovery/inplace/fixture"
	"github.com/harmony-one/harmony/internal/recovery/inplace/report"
)

// readFixtureHeader decodes a stored header from a fixture directory.
func readFixtureHeader(t *testing.T, dir string, num uint64, m *fixture.Manifest) *block.Header {
	t.Helper()
	raw, err := fixture.GetKey(dir, chainread.HeaderKey(num, m.Hashes[num]))
	if err != nil {
		t.Fatalf("read header %d: %v", num, err)
	}
	h := new(block.Header)
	if err := rlp.Decode(bytes.NewReader(raw), h); err != nil {
		t.Fatalf("decode header %d: %v", num, err)
	}
	return h
}

// --- Row 4: wrong target hash / broken ancestry / ss mismatch ---

func TestFailTargetHeaderRows(t *testing.T) {
	t.Run("canonical-mapping-missing", func(t *testing.T) {
		m, db := cloneFixture(t, fixture.VariantBase)
		mustMutate(t, fixture.DeleteKey(db, chainread.HeaderHashKey(fixture.TargetHeight)))
		wantFailLine(t, runCLI(t, m, db), "no canonical hash at target height")
	})
	t.Run("canonical-mapping-wrong", func(t *testing.T) {
		m, db := cloneFixture(t, fixture.VariantBase)
		mustMutate(t, fixture.PutKey(db, chainread.HeaderHashKey(fixture.TargetHeight), m.ChildHash.Bytes()))
		wantFailLine(t, runCLI(t, m, db), "want anchored")
	})
	t.Run("reverse-mapping-missing", func(t *testing.T) {
		m, db := cloneFixture(t, fixture.VariantBase)
		mustMutate(t, fixture.DeleteKey(db, chainread.HeaderNumberKey(m.TargetHash)))
		wantFailLine(t, runCLI(t, m, db), "no reverse number mapping")
	})
	t.Run("reverse-mapping-wrong", func(t *testing.T) {
		// The reverse hash->number mapping exists but names the wrong
		// height.
		m, db := cloneFixture(t, fixture.VariantBase)
		enc := make([]byte, 8)
		binary.BigEndian.PutUint64(enc, fixture.TargetHeight-1)
		mustMutate(t, fixture.PutKey(db, chainread.HeaderNumberKey(m.TargetHash), enc))
		wantFailLine(t, runCLI(t, m, db), "reverse mapping")
	})
	t.Run("target-header-missing", func(t *testing.T) {
		m, db := cloneFixture(t, fixture.VariantBase)
		mustMutate(t, fixture.DeleteKey(db, chainread.HeaderKey(fixture.TargetHeight, m.TargetHash)))
		wantFailLine(t, runCLI(t, m, db), "not present")
	})
	t.Run("target-header-wrong-content", func(t *testing.T) {
		// Valid header RLP under the right key, but recomputing its hash
		// betrays it (the anchored hash pins the entire header content).
		m, db := cloneFixture(t, fixture.VariantBase)
		otherRaw, err := fixture.GetKey(db, chainread.HeaderKey(fixture.TargetHeight-1, m.Hashes[fixture.TargetHeight-1]))
		if err != nil {
			t.Fatal(err)
		}
		mustMutate(t, fixture.PutKey(db, chainread.HeaderKey(fixture.TargetHeight, m.TargetHash), otherRaw))
		wantFailLine(t, runCLI(t, m, db), "recomputes to")
	})
	t.Run("anchor-hash-not-in-db", func(t *testing.T) {
		// Override the anchor with a hash the DB does not have.
		m, db := cloneFixture(t, fixture.VariantBase)
		res := runCLI(t, m, db, "--target-hash", "0x1111111111111111111111111111111111111111111111111111111111111111")
		wantFailLine(t, res, "canonical hash")
	})
}

func TestFailBodyRows(t *testing.T) {
	t.Run("body-missing", func(t *testing.T) {
		m, db := cloneFixture(t, fixture.VariantBase)
		mustMutate(t, fixture.DeleteKey(db, chainread.BlockBodyKey(fixture.TargetHeight, m.TargetHash)))
		wantFailLine(t, runCLI(t, m, db), "body not present")
	})
	t.Run("tx-root-mismatch", func(t *testing.T) {
		m, db := cloneFixture(t, fixture.VariantBase)
		header := readFixtureHeader(t, db, fixture.TargetHeight, m)
		body, err := types.NewBodyForMatchingHeader(header)
		if err != nil {
			t.Fatal(err)
		}
		tx := types.NewTransaction(7, m.ContractAddr, 0, big.NewInt(1), 21000, big.NewInt(1), nil)
		body.SetTransactions([]*types.Transaction{tx})
		raw, err := rlp.EncodeToBytes(body)
		if err != nil {
			t.Fatal(err)
		}
		mustMutate(t, fixture.PutKey(db, chainread.BlockBodyKey(fixture.TargetHeight, m.TargetHash), raw))
		wantFailLine(t, runCLI(t, m, db), "transaction root mismatch")
	})
	t.Run("incoming-receipt-root-mismatch", func(t *testing.T) {
		m, db := cloneFixture(t, fixture.VariantBase)
		header := readFixtureHeader(t, db, fixture.TargetHeight, m)
		body, err := types.NewBodyForMatchingHeader(header)
		if err != nil {
			t.Fatal(err)
		}
		body.SetIncomingReceipts(types.CXReceiptsProofs{{
			Receipts: types.CXReceipts{},
			MerkleProof: &types.CXMerkleProof{
				BlockNum:      big.NewInt(1),
				CXReceiptHash: m.TargetHash,
			},
			Header:       header,
			CommitSig:    []byte{0x01},
			CommitBitmap: []byte{0x01},
		}})
		raw, err := rlp.EncodeToBytes(body)
		if err != nil {
			t.Fatal(err)
		}
		mustMutate(t, fixture.PutKey(db, chainread.BlockBodyKey(fixture.TargetHeight, m.TargetHash), raw))
		wantFailLine(t, runCLI(t, m, db), "incoming-receipt root mismatch")
	})
	t.Run("nonempty-uncles", func(t *testing.T) {
		m, db := cloneFixture(t, fixture.VariantBase)
		header := readFixtureHeader(t, db, fixture.TargetHeight, m)
		body, err := types.NewBodyForMatchingHeader(header)
		if err != nil {
			t.Fatal(err)
		}
		uncle := readFixtureHeader(t, db, fixture.BoundaryHeight, m)
		body.SetUncles([]*block.Header{uncle})
		raw, err := rlp.EncodeToBytes(body)
		if err != nil {
			t.Fatal(err)
		}
		mustMutate(t, fixture.PutKey(db, chainread.BlockBodyKey(fixture.TargetHeight, m.TargetHash), raw))
		wantFailLine(t, runCLI(t, m, db), "uncles")
	})
}

func TestFailAncestryRows(t *testing.T) {
	mid := uint64(fixture.TargetHeight - 3)
	t.Run("parent-header-missing", func(t *testing.T) {
		m, db := cloneFixture(t, fixture.VariantBase)
		mustMutate(t, fixture.DeleteKey(db, chainread.HeaderKey(mid, m.Hashes[mid])))
		wantFailLine(t, runCLI(t, m, db), "broken parent link")
	})
	t.Run("canonical-disagrees-mid-walk", func(t *testing.T) {
		m, db := cloneFixture(t, fixture.VariantBase)
		mustMutate(t, fixture.PutKey(db, chainread.HeaderHashKey(mid), m.Hashes[mid+1].Bytes()))
		wantFailLine(t, runCLI(t, m, db), "canonical mapping at")
	})
	t.Run("boundary-header-missing", func(t *testing.T) {
		m, db := cloneFixture(t, fixture.VariantBase)
		mustMutate(t, fixture.DeleteKey(db, chainread.HeaderKey(fixture.BoundaryHeight, m.Hashes[fixture.BoundaryHeight])))
		wantFailLine(t, runCLI(t, m, db), "broken parent link")
	})
}

func TestFailShardStateRows(t *testing.T) {
	ssKey := chainread.ShardStateKey(big.NewInt(fixture.Epoch))
	t.Run("ss-missing", func(t *testing.T) {
		m, db := cloneFixture(t, fixture.VariantBase)
		mustMutate(t, fixture.DeleteKey(db, ssKey))
		wantFailLine(t, runCLI(t, m, db), "not present")
	})
	t.Run("ss-byte-different", func(t *testing.T) {
		m, db := cloneFixture(t, fixture.VariantBase)
		raw, err := fixture.GetKey(db, ssKey)
		if err != nil {
			t.Fatal(err)
		}
		raw[len(raw)/2] ^= 0xff
		mustMutate(t, fixture.PutKey(db, ssKey, raw))
		wantFailLine(t, runCLI(t, m, db), "differs from boundary header")
	})
}

// --- Certificate matrix ---

func targetPayload(m *fixture.Manifest, viewID uint64) []byte {
	return signature.ConstructCommitPayload(
		params.LocalnetChainConfig,
		shardingconfig.LocalnetSchedule.CalcEpochNumber(fixture.TargetHeight),
		m.TargetHash,
		fixture.TargetHeight, viewID)
}

func TestFailCertificateRows(t *testing.T) {
	sigKey := chainread.BlockCommitSigKey(fixture.TargetHeight)
	dropChild := func(t *testing.T, db string) {
		mustMutate(t, fixture.DeleteKey(db, chainread.HeaderHashKey(fixture.ChildHeight)))
		// Also detach the head pointers so the informational sample stays
		// quiet about the missing canonical entry.
	}
	t.Run("no-source-present", func(t *testing.T) {
		m, db := cloneFixture(t, fixture.VariantBase)
		mustMutate(t, fixture.DeleteKey(db, sigKey))
		dropChild(t, db)
		wantFailLine(t, runCLI(t, m, db), "no certificate source present")
	})
	t.Run("exact-key-bad-signature", func(t *testing.T) {
		m, db := cloneFixture(t, fixture.VariantBase)
		payload := append([]byte(nil), m.CertPayload...)
		payload[10] ^= 0x01 // corrupt the aggregate signature
		mustMutate(t, fixture.PutKey(db, sigKey, payload))
		wantFailLine(t, runCLI(t, m, db), "exact-key")
	})
	t.Run("bitmap-below-quorum", func(t *testing.T) {
		m, db := cloneFixture(t, fixture.VariantBase)
		under, err := m.SignRaw(targetPayload(m, fixture.TargetHeight), []int{0})
		if err != nil {
			t.Fatal(err)
		}
		mustMutate(t, fixture.PutKey(db, sigKey, under))
		dropChild(t, db)
		wantFailLine(t, runCLI(t, m, db), "not enough signature collected")
	})
	t.Run("payload-viewid-mismatch", func(t *testing.T) {
		m, db := cloneFixture(t, fixture.VariantBase)
		wrongView, err := m.SignRaw(targetPayload(m, fixture.TargetHeight+1), nil)
		if err != nil {
			t.Fatal(err)
		}
		mustMutate(t, fixture.PutKey(db, sigKey, wrongView))
		dropChild(t, db)
		wantFailLine(t, runCLI(t, m, db), "Unable to verify aggregated signature")
	})
	t.Run("outside-committee-signer", func(t *testing.T) {
		// Aggregate signed by a key that is not in the committee while the
		// bitmap claims the full committee: quorum-by-bitmap looks fine,
		// only the cryptographic verification catches it.
		m, db := cloneFixture(t, fixture.VariantBase)
		outside := &bls_core.SecretKey{}
		var scalar [32]byte
		scalar[0], scalar[1] = 0x99, 0x99 // deterministic, differs from all slot secrets
		if err := outside.SetLittleEndian(scalar[:]); err != nil {
			t.Fatal(err)
		}
		sig := outside.SignHash(targetPayload(m, fixture.TargetHeight))
		mask := bls_cosi.NewMask(m.PubKeys)
		for i := range m.PubKeys {
			if err := mask.SetBit(i, true); err != nil {
				t.Fatal(err)
			}
		}
		payload := append(sig.Serialize(), mask.Bitmap...)
		mustMutate(t, fixture.PutKey(db, sigKey, payload))
		dropChild(t, db)
		wantFailLine(t, runCLI(t, m, db), "Unable to verify aggregated signature")
	})
	t.Run("payload-wrong-hash", func(t *testing.T) {
		// Correct committee, correct height/viewID, but the payload binds a
		// different block hash.
		m, db := cloneFixture(t, fixture.VariantBase)
		wrongHash := signature.ConstructCommitPayload(
			params.LocalnetChainConfig,
			shardingconfig.LocalnetSchedule.CalcEpochNumber(fixture.TargetHeight),
			m.ChildHash, fixture.TargetHeight, fixture.TargetHeight)
		payload, err := m.SignRaw(wrongHash, nil)
		if err != nil {
			t.Fatal(err)
		}
		mustMutate(t, fixture.PutKey(db, sigKey, payload))
		dropChild(t, db)
		wantFailLine(t, runCLI(t, m, db), "Unable to verify aggregated signature")
	})
	t.Run("child-header-undecodable", func(t *testing.T) {
		// The canonical child slot holds garbage bytes: a present source
		// must verify, so this FAILs even though the exact key is valid.
		m, db := cloneFixture(t, fixture.VariantBase)
		mustMutate(t, fixture.PutKey(db, chainread.HeaderKey(fixture.ChildHeight, m.ChildHash), []byte{0xde, 0xad, 0xbe, 0xef}))
		res := runCLI(t, m, db)
		wantFailLine(t, res, "present but malformed")
		if !res.receipt.CertificateSources.ChildHeaderPresent {
			t.Fatalf("malformed child source not reported present: %+v", res.receipt.CertificateSources)
		}
	})
	t.Run("child-header-hash-invalid", func(t *testing.T) {
		// Valid header RLP of a DIFFERENT block stored under the child key:
		// only hash authentication betrays it.
		m, db := cloneFixture(t, fixture.VariantBase)
		otherRaw, err := fixture.GetKey(db, chainread.HeaderKey(fixture.BoundaryHeight, m.Hashes[fixture.BoundaryHeight]))
		if err != nil {
			t.Fatal(err)
		}
		mustMutate(t, fixture.PutKey(db, chainread.HeaderKey(fixture.ChildHeight, m.ChildHash), otherRaw))
		wantFailLine(t, runCLI(t, m, db), "present but malformed")
	})
	t.Run("sources-differ-bytewise", func(t *testing.T) {
		m, db := cloneFixture(t, fixture.VariantBase)
		// A second valid aggregate (8 of 9 signers still clears quorum) that
		// differs from the child header's full-committee bytes.
		signers := []int{1, 2, 3, 4, 5, 6, 7, 8}
		alt, err := m.SignRaw(targetPayload(m, fixture.TargetHeight), signers)
		if err != nil {
			t.Fatal(err)
		}
		mustMutate(t, fixture.PutKey(db, sigKey, alt))
		wantFailLine(t, runCLI(t, m, db), "differ byte-wise")
	})
	t.Run("child-only-satisfies", func(t *testing.T) {
		// Not a FAIL: exact key missing, child present and valid = PASS with
		// satisfied_by=child-header (apply time can materialize the key).
		m, db := cloneFixture(t, fixture.VariantBase)
		mustMutate(t, fixture.DeleteKey(db, sigKey))
		res := runCLI(t, m, db)
		wantExit(t, res, report.ExitPass)
		if res.receipt.CertificateSources.SatisfiedBy != "child-header" ||
			res.receipt.CertificateSources.ExactKeyPresent {
			t.Fatalf("certificate sources %+v", res.receipt.CertificateSources)
		}
	})
	t.Run("exact-key-only-satisfies", func(t *testing.T) {
		m, db := cloneFixture(t, fixture.VariantBase)
		dropChild(t, db)
		res := runCLI(t, m, db)
		wantExit(t, res, report.ExitPass)
		if res.receipt.CertificateSources.SatisfiedBy != "exact-key" ||
			res.receipt.CertificateSources.ChildHeaderPresent {
			t.Fatalf("certificate sources %+v", res.receipt.CertificateSources)
		}
		if res.receipt.HeadSample.ChildAtTargetPlus != "absent" {
			t.Fatalf("child sample %q", res.receipt.HeadSample.ChildAtTargetPlus)
		}
	})
}

// --- State deletion/corruption rows (row 1) ---

func TestFailStateRows(t *testing.T) {
	nodesOf := func(t *testing.T, m *fixture.Manifest, db string) *fixture.TrieNodes {
		nodes, err := fixture.EnumerateTrieNodes(db, m.StateRoot, m.ContractAddr)
		if err != nil {
			t.Fatalf("enumerate: %v", err)
		}
		return nodes
	}
	t.Run("state-root-node-deleted", func(t *testing.T) {
		m, db := cloneFixture(t, fixture.VariantBase)
		mustMutate(t, fixture.DeleteKey(db, m.StateRoot.Bytes()))
		res := runCLI(t, m, db)
		wantFailLine(t, res, "missing trie node")
		if !strings.Contains(res.stdout, m.StateRoot.Hex()[2:10]) {
			t.Fatalf("FAIL line does not name the root: %s", res.stdout)
		}
	})
	t.Run("account-internal-node-deleted", func(t *testing.T) {
		m, db := cloneFixture(t, fixture.VariantBase)
		nodes := nodesOf(t, m, db)
		if len(nodes.AccountInternal) == 0 {
			t.Fatal("fixture has no internal account nodes")
		}
		mustMutate(t, fixture.DeleteKey(db, nodes.AccountInternal[0].Bytes()))
		wantFailLine(t, runCLI(t, m, db), "missing trie node")
	})
	t.Run("storage-root-node-deleted", func(t *testing.T) {
		m, db := cloneFixture(t, fixture.VariantBase)
		nodes := nodesOf(t, m, db)
		mustMutate(t, fixture.DeleteKey(db, nodes.StorageRoot.Bytes()))
		wantFailLine(t, runCLI(t, m, db), "missing trie node")
	})
	t.Run("storage-internal-node-deleted", func(t *testing.T) {
		// Defect-2-class geometry: the root opens fine; the failure appears
		// on a child read during iteration and must never be silent-empty.
		m, db := cloneFixture(t, fixture.VariantBase)
		nodes := nodesOf(t, m, db)
		if len(nodes.StorageInternal) == 0 {
			t.Fatal("fixture storage trie has no internal nodes")
		}
		mustMutate(t, fixture.DeleteKey(db, nodes.StorageInternal[0].Bytes()))
		wantFailLine(t, runCLI(t, m, db), "missing trie node")
	})
	t.Run("node-content-substitution", func(t *testing.T) {
		// Valid RLP of a DIFFERENT node stored under the key: only content
		// authentication (keccak(blob)==key) catches it.
		m, db := cloneFixture(t, fixture.VariantBase)
		nodes := nodesOf(t, m, db)
		if len(nodes.AccountInternal) < 2 {
			t.Fatal("need two internal nodes")
		}
		other, err := fixture.GetKey(db, nodes.AccountInternal[1].Bytes())
		if err != nil {
			t.Fatal(err)
		}
		mustMutate(t, fixture.PutKey(db, nodes.AccountInternal[0].Bytes(), other))
		wantFailLine(t, runCLI(t, m, db), "content authentication")
	})
	t.Run("contract-code-deleted", func(t *testing.T) {
		m, db := cloneFixture(t, fixture.VariantBase)
		mustMutate(t, fixture.DeleteKey(db, append([]byte("c"), m.ContractCodeHash.Bytes()...)))
		wantFailLine(t, runCLI(t, m, db), "missing from all namespaces")
	})
	t.Run("validator-code-deleted", func(t *testing.T) {
		m, db := cloneFixture(t, fixture.VariantBase)
		mustMutate(t, fixture.DeleteKey(db, append([]byte("vc"), m.ValidatorCodeHashes[0].Bytes()...)))
		wantFailLine(t, runCLI(t, m, db), "missing from all namespaces")
	})
	t.Run("legacy-code-deleted", func(t *testing.T) {
		m, db := cloneFixture(t, fixture.VariantBase)
		mustMutate(t, fixture.DeleteKey(db, m.LegacyCodeHash.Bytes()))
		wantFailLine(t, runCLI(t, m, db), "missing from all namespaces")
	})
	t.Run("code-bytes-flipped", func(t *testing.T) {
		m, db := cloneFixture(t, fixture.VariantBase)
		key := append([]byte("c"), m.ContractCodeHash.Bytes()...)
		code, err := fixture.GetKey(db, key)
		if err != nil {
			t.Fatal(err)
		}
		code[len(code)/2] ^= 0x80
		mustMutate(t, fixture.PutKey(db, key, code))
		wantFailLine(t, runCLI(t, m, db), "hashes to")
	})
	t.Run("code-differs-across-locations", func(t *testing.T) {
		m, db := cloneFixture(t, fixture.VariantBase)
		// Same hash key in the vc namespace with different bytes.
		bogus := []byte("not the same bytes")
		mustMutate(t, fixture.PutKey(db, append([]byte("vc"), m.ContractCodeHash.Bytes()...), bogus))
		wantFailLine(t, runCLI(t, m, db), "DIFFERENT bytes")
	})
	t.Run("code-duplicate-identical-anomaly", func(t *testing.T) {
		// Identical bytes at c and legacy: anomaly, still PASS.
		m, db := cloneFixture(t, fixture.VariantBase)
		code, err := fixture.GetKey(db, append([]byte("c"), m.ContractCodeHash.Bytes()...))
		if err != nil {
			t.Fatal(err)
		}
		mustMutate(t, fixture.PutKey(db, m.ContractCodeHash.Bytes(), code))
		res := runCLI(t, m, db)
		wantExit(t, res, report.ExitPass)
		if res.receipt.State.Anomalies.ByKind["code-multiple-locations"] == 0 {
			t.Fatalf("expected code-multiple-locations anomaly: %+v", res.receipt.State.Anomalies)
		}
	})
	t.Run("bad-account-leaf", func(t *testing.T) {
		m, db := cloneFixture(t, fixture.VariantBadAccountLeaf)
		wantFailLine(t, runCLI(t, m, db), "does not decode")
	})
	t.Run("bad-storage-leaf", func(t *testing.T) {
		m, db := cloneFixture(t, fixture.VariantBadStorageLeaf)
		wantFailLine(t, runCLI(t, m, db), "not an RLP byte string")
	})
	t.Run("flagged-empty-code", func(t *testing.T) {
		m, db := cloneFixture(t, fixture.VariantFlaggedEmptyCode)
		wantFailLine(t, runCLI(t, m, db), "empty code hash")
	})
	t.Run("validator-codes-swapped", func(t *testing.T) {
		// Swapping the two validators' wrapper blobs breaks the
		// keccak(code)==CodeHash authentication (content moved under the
		// wrong hash key).
		m, db := cloneFixture(t, fixture.VariantBase)
		vc0 := append([]byte("vc"), m.ValidatorCodeHashes[0].Bytes()...)
		vc1 := append([]byte("vc"), m.ValidatorCodeHashes[1].Bytes()...)
		c0, err := fixture.GetKey(db, vc0)
		if err != nil {
			t.Fatal(err)
		}
		c1, err := fixture.GetKey(db, vc1)
		if err != nil {
			t.Fatal(err)
		}
		mustMutate(t, fixture.PutKey(db, vc0, c1))
		mustMutate(t, fixture.PutKey(db, vc1, c0))
		wantFailLine(t, runCLI(t, m, db), "hashes to")
	})
	t.Run("flagged-legacy-wrapper-code", func(t *testing.T) {
		// PASS row: a flag-set account whose wrapper blob lives ONLY at the
		// legacy bare-hash key still classifies as a validator, and the
		// wrapper decode + address-binding checks still run (physical
		// location does not determine class).
		m, db := cloneFixture(t, fixture.VariantBase)
		vcKey := append([]byte("vc"), m.ValidatorCodeHashes[0].Bytes()...)
		blob, err := fixture.GetKey(db, vcKey)
		if err != nil {
			t.Fatal(err)
		}
		mustMutate(t, fixture.PutKey(db, m.ValidatorCodeHashes[0].Bytes(), blob))
		mustMutate(t, fixture.DeleteKey(db, vcKey))
		res := runCLI(t, m, db)
		wantExit(t, res, report.ExitPass)
	})
	t.Run("flagged-wrapper-address-unbound", func(t *testing.T) {
		// Build-time variant: hash-consistent wrapper bound to a foreign
		// address on a flagged account - only the address binding catches
		// it.
		m, db := cloneFixture(t, fixture.VariantWrapperUnbound)
		wantFailLine(t, runCLI(t, m, db), "does not bind")
	})
}

// --- Anomaly truncation (row 10) ---

func TestAnomalyTruncation(t *testing.T) {
	m, db := cloneFixture(t, fixture.VariantManyAnomalies)
	res := runCLI(t, m, db)
	wantExit(t, res, report.ExitPass)
	an := res.receipt.State.Anomalies
	total := fixture.ManyAnomaliesCount + 1 + 3 // planted + base flag-zero + (odd, wrapper-shaped, dual-class)
	if an.Total != total {
		t.Fatalf("anomaly total = %d, want %d (%+v)", an.Total, total, an)
	}
	if len(an.Example) != 20 {
		t.Fatalf("examples = %d, want exactly 20", len(an.Example))
	}
	if an.Omitted != total-20 {
		t.Fatalf("omitted = %d, want %d", an.Omitted, total-20)
	}
	if an.ByKind["flag-decoded-zero"] != fixture.ManyAnomaliesCount+1 {
		t.Fatalf("by_kind = %+v", an.ByKind)
	}
	if st, err := os.Stat(res.report); err != nil || st.Size() > 64*1024 {
		t.Fatalf("receipt size %v bytes exceeds 64KiB bound (err %v)", st.Size(), err)
	}
}

// --- Exit 2 rows: layout, flags, report path, fd budget ---

func TestUnusableRows(t *testing.T) {
	t.Run("mainnet-override-refused", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := run([]string{"preflight", "--db", t.TempDir(), "--network", "mainnet",
			"--target-height", "44", "--target-hash", "0xabc"}, &stdout, &stderr)
		if code != report.ExitUnusable {
			t.Fatalf("exit = %d, want 2 (stderr %s)", code, stderr.String())
		}
		if !strings.Contains(stderr.String(), "refused on --network mainnet") {
			t.Fatalf("stderr %q", stderr.String())
		}
	})
	t.Run("missing-db", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := run([]string{"preflight", "--db", filepath.Join(t.TempDir(), "nope"),
			"--network", "localnet", "--target-height", "44",
			"--target-hash", "0x1111111111111111111111111111111111111111111111111111111111111111"},
			&stdout, &stderr)
		if code != report.ExitUnusable {
			t.Fatalf("exit = %d, want 2", code)
		}
	})
	t.Run("sharded-layout", func(t *testing.T) {
		m, _ := cloneFixture(t, fixture.VariantBase)
		dir := filepath.Join(t.TempDir(), "harmony_sharddb_0")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		res := runCLI(t, m, dir)
		wantExit(t, res, report.ExitUnusable)
		if !strings.Contains(res.stderr, "sharded") {
			t.Fatalf("stderr %q", res.stderr)
		}
	})
	t.Run("wrong-basename", func(t *testing.T) {
		// A perfectly valid DB under a renamed directory is refused: --db
		// must point at harmony_db_<shard> itself.
		m, db := cloneFixture(t, fixture.VariantBase)
		renamed := filepath.Join(filepath.Dir(db), "db-backup")
		if err := os.Rename(db, renamed); err != nil {
			t.Fatal(err)
		}
		res := runCLI(t, m, renamed)
		wantExit(t, res, report.ExitUnusable)
		if !strings.Contains(res.stderr, "harmony_db_0") {
			t.Fatalf("stderr %q", res.stderr)
		}
	})
	t.Run("pebble-layout", func(t *testing.T) {
		m, db := cloneFixture(t, fixture.VariantBase)
		if err := os.WriteFile(filepath.Join(db, "OPTIONS-000005"), []byte("pebble"), 0o644); err != nil {
			t.Fatal(err)
		}
		res := runCLI(t, m, db)
		wantExit(t, res, report.ExitUnusable)
		if !strings.Contains(res.stderr, "pebble") {
			t.Fatalf("stderr %q", res.stderr)
		}
	})
	t.Run("report-inside-db-refused", func(t *testing.T) {
		m, db := cloneFixture(t, fixture.VariantBase)
		var stdout, stderr bytes.Buffer
		code := run([]string{"preflight", "--db", db, "--network", "localnet",
			"--target-height", fmt.Sprint(fixture.TargetHeight),
			"--target-hash", m.TargetHash.Hex(),
			"--report", filepath.Join(db, "r.json")}, &stdout, &stderr)
		if code != report.ExitUnusable {
			t.Fatalf("exit = %d, want 2 (stderr %s)", code, stderr.String())
		}
		if !strings.Contains(stderr.String(), "inside the DB directory") {
			t.Fatalf("stderr %q", stderr.String())
		}
	})
	t.Run("fd-budget-too-low", func(t *testing.T) {
		m, db := cloneFixture(t, fixture.VariantBase)
		res := runCLI(t, m, db, "--handles", "2000000000")
		wantExit(t, res, report.ExitUnusable)
		if !strings.Contains(res.stderr, "RLIMIT_NOFILE") {
			t.Fatalf("stderr %q", res.stderr)
		}
	})
}

// --- Exit 3 rows: persistent read errors ---

func TestReadErrorRows(t *testing.T) {
	t.Run("referenced-sst-deleted", func(t *testing.T) {
		// Deleting a referenced table file is a (normally transient) race
		// class; with the file permanently gone, bounded retries exhaust
		// into exit 3 with the remedy line.
		m, db := cloneFixture(t, fixture.VariantBase)
		removeOneSST(t, db)
		res := runCLI(t, m, db)
		wantExit(t, res, report.ExitReadError)
		if !strings.Contains(res.stderr, "remedy: re-run") {
			t.Fatalf("stderr lacks remedy line:\n%s", res.stderr)
		}
		// Schema v2 result enum is exactly PASS|FAIL; exit_code 3 marks the
		// read-error class.
		if res.receipt == nil || res.receipt.Result != "FAIL" || res.receipt.ExitCode != 3 {
			t.Fatalf("receipt %+v", res.receipt)
		}
		if res.receipt.Retries.ReopenCount == 0 {
			t.Fatalf("expected reopen retries before giving up, got %+v", res.receipt.Retries)
		}
		// The one-line stdout contract holds for read errors too (exit
		// code 3 distinguishes the class).
		lines := strings.Split(strings.TrimRight(res.stdout, "\n"), "\n")
		if len(lines) != 1 || !strings.HasPrefix(lines[0], "FAIL: ") {
			t.Fatalf("stdout must be exactly one FAIL line on read error, got %q", res.stdout)
		}
		if !strings.Contains(lines[0], "read error") {
			t.Fatalf("FAIL line %q does not name the read error", lines[0])
		}
	})
	t.Run("sst-block-corruption-zero-retries", func(t *testing.T) {
		// Immutable-SST corruption: direct exit 3 with zero reopen attempts,
		// console naming the corrupt table.
		m, db := cloneFixture(t, fixture.VariantBase)
		name := corruptLargestSST(t, db)
		res := runCLI(t, m, db)
		wantExit(t, res, report.ExitReadError)
		if res.receipt.Retries.ReopenCount != 0 {
			t.Fatalf("reopen count = %d, want 0 (non-retryable class)", res.receipt.Retries.ReopenCount)
		}
		if !strings.Contains(res.stderr, "corrupt") {
			t.Fatalf("stderr does not mention corruption:\n%s", res.stderr)
		}
		_ = name
	})
	t.Run("corrupt-manifest-stopped-db", func(t *testing.T) {
		// The geth-wrapper hazard regression: open fails, nothing recovers
		// or rewrites the manifest, directory contents byte-identical.
		m, db := cloneFixture(t, fixture.VariantBase)
		manifest := findFile(t, db, "MANIFEST-")
		raw, err := os.ReadFile(manifest)
		if err != nil {
			t.Fatal(err)
		}
		raw[len(raw)/2] ^= 0xff
		if err := os.WriteFile(manifest, raw, 0o644); err != nil {
			t.Fatal(err)
		}
		before := snapshotDir(t, db)
		res := runCLI(t, m, db)
		wantExit(t, res, report.ExitReadError)
		after := snapshotDir(t, db)
		diffSnapshots(t, before, after)
	})
}

// TestNoWritesToLiveDirectory asserts the tool leaves the database directory
// byte-identical after a full PASS run (no LOCK, no LOG, no anything).
func TestNoWritesToLiveDirectory(t *testing.T) {
	m, db := cloneFixture(t, fixture.VariantBase)
	before := snapshotDir(t, db)
	res := runCLI(t, m, db)
	wantExit(t, res, report.ExitPass)
	after := snapshotDir(t, db)
	diffSnapshots(t, before, after)
}

// --- helpers ---

func mustMutate(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("fixture mutation: %v", err)
	}
}

func removeOneSST(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, ent := range entries {
		if strings.HasSuffix(ent.Name(), ".ldb") || strings.HasSuffix(ent.Name(), ".sst") {
			if err := os.Remove(filepath.Join(dir, ent.Name())); err != nil {
				t.Fatal(err)
			}
			return
		}
	}
	t.Fatal("fixture has no table files (all data still in the journal?)")
}

func corruptLargestSST(t *testing.T, dir string) string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var best string
	var bestSize int64
	for _, ent := range entries {
		if !strings.HasSuffix(ent.Name(), ".ldb") && !strings.HasSuffix(ent.Name(), ".sst") {
			continue
		}
		info, err := ent.Info()
		if err != nil {
			t.Fatal(err)
		}
		if info.Size() > bestSize {
			best, bestSize = ent.Name(), info.Size()
		}
	}
	if best == "" {
		t.Fatal("fixture has no table files")
	}
	path := filepath.Join(dir, best)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	// Flip a byte in (nearly) every 4KiB block so any read path hits a
	// checksum failure deterministically, sparing the footer.
	for off := 4096; off < len(raw)-128; off += 4096 {
		raw[off] ^= 0xa5
	}
	if len(raw) > 256 {
		raw[len(raw)/2] ^= 0xa5
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	return best
}

// corruptAllSSTs flips bytes in every data block of every table file.
func corruptAllSSTs(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	corrupted := 0
	for _, ent := range entries {
		if !strings.HasSuffix(ent.Name(), ".ldb") && !strings.HasSuffix(ent.Name(), ".sst") {
			continue
		}
		path := filepath.Join(dir, ent.Name())
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if len(raw) < 256 {
			continue
		}
		raw[16] ^= 0xa5
		for off := 4096; off < len(raw)-128; off += 4096 {
			raw[off] ^= 0xa5
		}
		if err := os.WriteFile(path, raw, 0o644); err != nil {
			t.Fatal(err)
		}
		corrupted++
	}
	if corrupted == 0 {
		t.Fatal("no table files to corrupt")
	}
}

func findFile(t *testing.T, dir, prefix string) string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, ent := range entries {
		if strings.HasPrefix(ent.Name(), prefix) {
			return filepath.Join(dir, ent.Name())
		}
	}
	t.Fatalf("no %s* file in %s", prefix, dir)
	return ""
}

type dirSnapshot map[string][]byte

func snapshotDir(t *testing.T, dir string) dirSnapshot {
	t.Helper()
	snap := dirSnapshot{}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, ent := range entries {
		if ent.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, ent.Name()))
		if err != nil {
			t.Fatal(err)
		}
		snap[ent.Name()] = data
	}
	return snap
}

func diffSnapshots(t *testing.T, before, after dirSnapshot) {
	t.Helper()
	for name := range after {
		if _, ok := before[name]; !ok {
			t.Fatalf("tool created file %s in the database directory", name)
		}
	}
	for name, data := range before {
		got, ok := after[name]
		if !ok {
			t.Fatalf("file %s disappeared from the database directory", name)
		}
		if !bytes.Equal(data, got) {
			t.Fatalf("file %s changed (%d -> %d bytes)", name, len(data), len(got))
		}
	}
}
