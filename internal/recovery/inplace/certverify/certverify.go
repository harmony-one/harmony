// Package certverify verifies the target block's commit certificate from
// its two possible local sources, using the audited
// chain.NewEngine().VerifyHeaderSignature path against the minimal
// fail-closed ChainReader (BlockChainImpl is never constructed).
package certverify

import (
	"bytes"

	"github.com/ethereum/go-ethereum/ethdb"

	"github.com/harmony-one/harmony/internal/chain"
	"github.com/harmony-one/harmony/internal/recovery/inplace/anchor"
	"github.com/harmony-one/harmony/internal/recovery/inplace/chainread"
	"github.com/harmony-one/harmony/internal/recovery/inplace/report"
)

// Result records which certificate sources were present and which satisfied
// the check.
type Result struct {
	Sources report.CertificateSources
}

// Verify runs check 6 (target certificate, two sources):
//
//   - source A: the exact raw key "block-sig-"+BE64(target height)
//   - source B: the child header's LastCommitSignature+LastCommitBitmap
//
// Every present source is verified cryptographically: stake-weighted quorum
// over the committee decoded from the walk-authenticated ss record, payload
// binding blockNum+blockHash+viewID. Decision: at least one source present
// and verifying passes; two present sources that differ byte-wise FAIL (two
// different valid aggregates for one block must not be papered over); zero
// present sources or any present-but-failing source FAIL.
func Verify(kv ethdb.KeyValueReader, a *anchor.Anchor, out *chainread.Outcome) (*Result, error) {
	res := &Result{}

	exact, foundExact, err := chainread.ReadBlockCommitSig(kv, a.TargetHeight)
	if err != nil {
		return res, err
	}
	res.Sources.ExactKeyPresent = foundExact

	// A present-but-malformed child header (undecodable, or failing hash
	// authentication) is a failing source, not an absent one: every present
	// source must verify, even when the exact key alone would pass.
	if out.ChildSourceErr != "" {
		res.Sources.ChildHeaderPresent = true
		return res, report.Failf("certificate",
			"child-header source at %d is present but malformed: %s", a.TargetHeight+1, out.ChildSourceErr)
	}

	var childPayload []byte
	if out.ChildHeader != nil {
		sig := out.ChildHeader.LastCommitSignature()
		childPayload = append(sig[:], out.ChildHeader.LastCommitBitmap()...)
		res.Sources.ChildHeaderPresent = true
	}

	if !foundExact && childPayload == nil {
		return res, report.Failf("certificate",
			"no certificate source present: block-sig-%d missing and no child header at %d",
			a.TargetHeight, a.TargetHeight+1)
	}

	reader := chainread.NewMinimalChainReader(a.ChainConfig, a.ShardID, out.TargetHeader, a.Epoch, out.ShardState)
	engine := chain.NewEngine()

	verify := func(name string, payload []byte) error {
		sig, bitmap, err := chain.ParseCommitSigAndBitmap(payload)
		if err != nil {
			return report.Failf("certificate", "source %s does not parse: %v", name, err)
		}
		if err := engine.VerifyHeaderSignature(reader, out.TargetHeader, sig, bitmap); err != nil {
			return report.Failf("certificate", "source %s fails verification: %v", name, err)
		}
		return nil
	}

	var satisfied []string
	if foundExact {
		if err := verify("exact-key", exact); err != nil {
			return res, err
		}
		satisfied = append(satisfied, "exact-key")
	}
	if childPayload != nil {
		if err := verify("child-header", childPayload); err != nil {
			return res, err
		}
		satisfied = append(satisfied, "child-header")
	}
	if foundExact && childPayload != nil && !bytes.Equal(exact, childPayload) {
		return res, report.Failf("certificate",
			"exact-key and child-header certificates are both valid but differ byte-wise (%d vs %d bytes)",
			len(exact), len(childPayload))
	}
	if len(satisfied) == 1 {
		res.Sources.SatisfiedBy = satisfied[0]
	} else {
		res.Sources.SatisfiedBy = "exact-key+child-header"
	}
	// Note for Workstream B (informational): if source A was missing, apply
	// time can materialize the exact key from the verified child header.
	return res, nil
}
