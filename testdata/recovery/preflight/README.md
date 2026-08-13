# Preflight test fixtures

Deterministic fixtures for `harmony-recovery preflight`
(`cmd/harmony-recovery`, `internal/recovery/inplace/*`). Each variant
directory holds a small shard-0 LevelDB (`harmony_db_0`) with a BLS-signed
localnet header chain (boundary 36 → target 44 → child 45, epoch 3), the
boundary `ss` record, exact `block-sig` record, and a state trie with EOA /
contract / validator / legacy-code / crafted flag-edge accounts.
`fixtures.json` lists each variant's target hash and state root; `golden/`
holds the pinned receipts and the toy-state digest vector.

Regenerate with:

```bash
scripts/recovery/gen-preflight-fixtures.sh
```

Generation is **byte-reproducible**: after building, the generator rewrites
each database into a canonical form (every key-value re-inserted in sorted
order, so LevelDB's internal sequence numbers are a pure function of the
content, then fully compacted; the timestamped `LOG` file is dropped).
`TestBuildByteReproducible` pins two generations byte-identical, and
`TestCommittedFixtureAgreement` fails if the committed trees drift from a
fresh generation.

Tests build identical fixtures hermetically in temp dirs via
`internal/recovery/inplace/fixture`; the one exception is
`TestCommittedFixtureAgreement`, which runs the CLI against the committed
`base/harmony_db_0` copy and fails if it is missing or drifts from the
golden receipt — pinning the committed fixtures, the in-test generator and
the goldens together. The materialized copies also serve manual inspection
and ad-hoc runs:

```bash
./bin/harmony-recovery preflight \
    --db testdata/recovery/preflight/base/harmony_db_0 \
    --network localnet \
    --target-height 44 \
    --target-hash $(jq -r .base.target_hash testdata/recovery/preflight/fixtures.json)
```

## SECURITY NOTE — fixture-only BLS secrets

The committee secret keys in these fixtures are **fixed small scalars**
(secret *i* = 32-byte little-endian of *i+1*; the pinned
`SecretKey.SetLittleEndian` performs no modular reduction). They exist so
the fixtures are byte-reproducible. They are trivially recoverable by
anyone and must NEVER be used outside tests.
