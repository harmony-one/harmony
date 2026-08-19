# harmony-recovery-db — canonical CLI contract

Single source of truth for flags (plan §4). Every command is fail-closed:
absolute paths only, sources opened strictly read-only through the
never-recovering opener, `--network` and `--shard` mandatory, v1 refuses
`--shard != 0`, every read/iterator/decode/write/close error is fatal, and
no networking/RPC/txpool/consensus/BLS-signing is ever initialized.

Exit codes: `0` ok · `2` usage · `3` precondition · `4` verification-failed ·
`5` io/corruption.

## Global (persistent) flags

| Flag | Meaning |
| --- | --- |
| `--network` | `mainnet` \| `testnet` \| `localnet` \| `partner` \| `stressnet` \| `pangaea` — **mandatory** |
| `--shard` | shard ID; v1 supports only `0` — **mandatory** |
| `--log-file` | optional log file (stderr otherwise) |

## inspect-db

```
inspect-db --network --shard --db PATH --read-only \
  [--full-state-check] [--full-offchain-check] [--require-preimages] \
  [--target-height N] [--anchor-manifest PATH] \
  [--compare-with OTHER_REPORT.json] [--agreement-output PATH] \
  --output report.json
```

- `--read-only` mandatory. `--require-preimages` requires `--full-state-check`.
- `--target-height` enables the full-archival replay preflight and the
  baseline gate. `--anchor-manifest` adds the known-bad checks to the gate.
- `--compare-with` emits a baseline-agreement verdict (default
  `<output>.agreement.json`); agreement requires complete DigestSets on both
  copies (both `--full-state-check` and `--full-offchain-check`).

## inventory-db

```
inventory-db --network --shard --db PATH --read-only --output inventory.json
```

Minimal namespace accounting (counts + logical bytes per bucket). All
un-prefixed 32-byte keys land in the single physical `bare-hash32` bucket; an
unresolved bare key is **reported, not fatal** on an archival source.

## export-bundle

```
export-bundle --network --shard --source-db PATH --read-only \
  --baseline-manifest INSPECT_REPORT.json \
  --from-height N --to-height M --certificate-child-height M+1 \
  [--report-only] [--anchor-manifest PATH] [--chunk-bytes N] [--donor STR] \
  --output DIR            # (or a report path with --report-only)
```

- `--from-height` must equal the baseline inspect report's head + 1;
  `--certificate-child-height` must equal `--to-height + 1`.
- `--report-only` runs the mechanical donor preflight and writes a report;
  a gapped donor is refused (block-accurate).

## compare-bundles (optional)

```
compare-bundles --network --shard --left DIR --right DIR --output report.json
```

Chain differences are fatal; donor-local `block-sig-N` differences are
informational.

## replay-bundle

```
replay-bundle --network --shard --destination-db PATH \
  --inspect-report FRESH_INSPECT.json --baseline-agreement VERDICT.json \
  --bundle DIR [--bundle-comparison PATH] \
  --anchor-manifest PATH --target-height N \
  --offline --no-resume-on-unclean-exit [--min-free-bytes N] \
  --output replay.json
```

- `--offline` and `--no-resume-on-unclean-exit` are mandatory acknowledgements.
- Requires a fresh `--inspect-report` of the destination and a passing
  `--baseline-agreement` that names it. `--bundle-comparison` is optional
  (single-donor mode). Any existing journal ⇒ refuse (v1 no-resume).

## compact-db

```
compact-db --network --shard --source-db PATH --source-read-only \
  --destination-db PATH --anchor-manifest PATH --source-reference replay.json \
  [--metadata-reference-manifest PATH] --target-height N \
  [--retain-from-height M] [--batch-bytes N] --fail-if-destination-nonempty \
  [--size-limit-bytes N] [--with-validator-stats] [--with-preimages LIST.json] \
  --output compact.json
```

- `--source-read-only` and `--fail-if-destination-nonempty` mandatory.
- `--metadata-reference-manifest` selects **reference mode**; absent ⇒
  **internal mode** with the `internal:none` sentinel. The mode is recorded
  in `compact.json` and must match at `verify-db` time.
- `--retain-from-height` may only extend retention (lower the start).
- A clean build over `--size-limit-bytes` (default 200 GB) finishes
  `COMPLETE_UNRELEASABLE` (preserved, refused by `package-db`).

## verify-db

```
verify-db --network --shard --db PATH --read-only --anchor-manifest PATH \
  [--metadata-reference-manifest PATH] --full-state-check --full-offchain-check \
  --source-reference compact.json [--temp-dir DIR] --output verification.json
```

- `--read-only`, `--full-state-check`, `--full-offchain-check` mandatory.
- `--metadata-reference-manifest` must be supplied **iff** `compact.json`
  records reference mode — supplied on one side and not the other is an error.

## package-db

```
package-db --network --shard --db COMPACT_DB --anchor-manifest PATH \
  --target-height N --verification-report verification.json \
  [--recovery-harmony-binary-sha256 H] [--provisional-start-view-id V] \
  --release-root DIR --output package.json
```

Single invocation: stage → fully re-verify → atomically promote → seal with
`READY` last. The optional in-place integration fields default to `"absent"`.
The devops publish note is printed on success.
