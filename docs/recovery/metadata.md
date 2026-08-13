# `harmony-recovery metadata` — validator-metadata derivation, reference export, branch audit

Normative reference for the `metadata` command family (plan Workstream B).
These are **internal, run-once** tools operated on the single source machine
`mhe-snaps0-01` (synced to the abandoned head `92,751,016`). Validators
never run them: they run B4 `prepare --apply` / the patched binary, which
re-derive the same normalization locally and refuse on reference mismatch.

The commands extend the preflight-owned `harmony-recovery` binary. They
require a **stopped** node (the strict opener takes a shared `flock`; a live
node holding the exclusive lock fails the open).

- [Commands](#commands)
- [HMR1 container format](#hmr1-container-format)
- [Reference manifest schema](#reference-manifest-schema)
- [Report schemas](#report-schemas)
- [Digest definitions](#digest-definitions)
- [Exit codes](#exit-codes)
- [Consumer interface contract (B4 / C / D / E)](#consumer-interface-contract)
- [Run-once runbook (mhe-snaps0-01)](#run-once-runbook)
- [Release artifacts](#release-artifacts)
- [Performance pilot gate](#performance-pilot-gate)
- [Operator decision log](#operator-decision-log)
- [Handoff-vs-code discrepancy register](#handoff-vs-code-discrepancy-register)
- [Fixture regeneration](#fixture-regeneration)

## Commands

All three read `recovery-anchor.json` (`--anchor`) for the frozen incident
constants; there is **no `--network` flag** (network, shard and the
constants come from the anchor, cross-checked against the schedule and the
DB at run time). Every output path (`--report`, `--out-dir`, `--scratch`)
is refused if it resolves inside the source DB directory.

Common LevelDB tuning flags on every subcommand: `--handles` (open-file
cache capacity, 0 = default 512) and `--db-cache-mb` (block cache size in
MiB, 0 = default 256).

### `metadata scan --db PATH --anchor FILE --report FILE`

Read-only diagnosis / dry-run — run first, and as the cheap re-check tool.
Strict read-only open, anchor cross-verification, target resolution,
one normalization pass, and a full JSON report: per-section
retained/removed/invalid/missing/duplicate counts, the deletion plan (counts
+ digest; apply re-derives, the report authenticates), the full digest set,
absence assertions, snapshot-reconstruction coverage, the kept-untouched
`validator-stats` inventory, sync-era/legacy key presence, the shard-1
freeze inputs, DB identity fingerprints before/after, and a zero-write
proof. The normalized validator-list length is printed prominently for a
**manual, informational** comparison against the separately-run preflight
(there is no receipt input and no gating — §8 Q2).

### `metadata export-reference --db PATH --anchor FILE --out-dir DIR`

The run-once reference producer. Any `Fatal`/`MissingRequired` finding
refuses export (`ReviewItem`s are allowed and recorded in the diagnostics
digest). A built-in **double-run determinism self-check** derives everything
twice over independent handles and byte-compares both `.hmr` serializations
and both reference manifests before writing anything; on mismatch it exits
`23` with only `determinism-diff/pass-a.*` / `pass-b.*` dumps and the export
report (no release artifacts).

Publication is **failure-atomic, one visible unit**: artifacts and the
finalized success `export-report.json` are staged privately together, the
staged set is verified (exactly the expected files, none empty), and only
then published as `<out-dir>/release/` through ONE atomic directory rename.
The `release/` directory therefore either holds the complete verified set —
`metadata-<target>.hmr`, `metadata-<target>.reference.json`,
`run-checksums.sha256`, `export-report.json` — or does not exist at all; an
incomplete set can never appear under a consumer-facing release name, on
any failure path (export refuses an out-dir that already has `release/`),
and there is no window in which `release/` exists without its success
report or an exit-0 report is visible without `release/`. On a failed or
refused attempt a separate FAILURE report is written to `<out-dir>` itself;
a success-shaped report only ever exists inside `release/`. The self-check
cannot be skipped in a release build.

### `metadata audit-branch --db PATH --anchor FILE --out-dir DIR --scratch DIR`

Masked-overlay re-execution of the abandoned branch
`target+1 .. --end-height` (default anchor `audit_end_height`). Reads consult
scratch first, then the source minus a mask; writes go to scratch only — the
source stays strictly read-only. Before any block executes, scratch is
seeded with the full mechanical application of the normalization output
(deletion tombstones + materialized rewrites), the post-target chain
tombstones, and the heads rewound to the target — i.e. a dry run of B4's
end state. Mandatory **two-pass** structure: pass 1 discovers the
branch-written crosslink/spent records (classifying affected findings
`pollution-suspect`); pass 2 masks them and is authoritative. Emits
`abandoned-branch-audit.json`.

Flags: `--end-height` (default anchor value), `--scratch`, `--keep-scratch`,
`--single-pass` (debug only; output marked non-authoritative),
`--trusted-shard1-pointer <shardID>:<blockNum>` (pre-incident escape hatch,
accepted only if it satisfies the §4.4 invariants),
`--trusted-shard1-pointer-provenance`, `--scratch-reserve-gb` (default 200;
must be non-negative — there is no bypass), `--reference` (path to the
`export-reference` manifest: the audit cross-checks the manifest's target
tuple and digests against its own view of the source and binds the manifest
digest into the report's hash chain).

## HMR1 container format

Byte-exact, deterministic, byte-reproducible from a `NormalizedSet`:

```text
magic 4B ASCII "HMR1"
format-version u32BE = 1
anchor-digest  32B = SHA-256 of the exact recovery-anchor.json bytes
record-count   u64BE
records, in strictly increasing raw-key order (bytewise), each:
    key-length   u32BE
    raw-key      (exact LevelDB key)
    value-length u64BE
    raw-value
```

The decoder rejects, each with a distinct error class: bad magic, bad
version, truncation, trailing bytes, non-monotone order, duplicate keys, and
count/header disagreement. Sections are assigned by key shape (the container
stays a flat ordered set): `validator-list` (1 record, RLP of the normalized
ordered list), `dvl` (every corrected non-empty record), `validator-snapshot-<epoch>`
(retained target-epoch snapshots, original validated bytes), `shard-state-<epoch>`
(1 record, exact bytes), `reward-accumulator` (1 record — `blk-rwd-<target>`,
§8 Q5). Stats are never included; target wrapper/state blobs are never
included.

## Reference manifest schema

`metadata-<target>.reference.json` — strictly canonical JSON (sorted keys,
no insignificant whitespace, no HTML escaping), **only chain-invariant,
timestamp-free** material. Its SHA-256 is THE reference digest the release
notes publish and B4/D bind. Schema `hmr-reference-v1`:

```json
{
  "schema": "hmr-reference-v1",
  "network": "mainnet",
  "shard": 0,
  "anchor": { "target_height", "target_hash", "target_root", "epoch",
              "epoch_first_block", "epoch_last_block",
              "snapshot_base_height", "abandoned_child_hash" },
  "anchor_config_sha256": "<sha256 of recovery-anchor.json>",
  "ruleset_version": "hmr-norm-v1",
  "package_sha256": "<sha256 of the .hmr file bytes>",
  "record_count": <u64>,
  "sections": [ { "name", "record_count", "sha256" }, ... ],
  "wrapper_set_sha256": "<hex>",
  "diagnostics_sha256": "<hex>",
  "absence_assertions": [ { "namespace", "predicate", "expected_remaining": 0 }, ... ]
}
```

`planned_deletions` counts are **excluded** (source-specific run evidence);
the manifest carries only the predicate and the required end-state.
Timestamps are forbidden. Varying only per-run deletion counts must not
change the reference digest; flipping any bound input (payload byte, ruleset
version, a section digest) must change it.

## Report schemas

- **scan report** (`metadata-scan-report-v1`) and **export report**
  (`metadata-export-report-v1`) carry run evidence: DB fingerprints, counts,
  coverage, the per-phase deletion plan with `planned_deletions` counts,
  timings, `created_at`. These are never digested into the reference. The
  scan report stays internal; the export SUCCESS report is published
  atomically inside `release/` with the artifacts (failure reports go to
  the out-dir root), but it is run evidence, not part of the sealed
  four-file release set.
- **audit report** (`abandoned-branch-audit-v1`,
  `abandoned-branch-audit.json`) is a release artifact: per-block execution
  + validity results, first validity failure cross-checked against the
  anchor known-bad list, the staking-op inventory (native by directive +
  precompile by kind), the epoch-transition byte-equality record, the
  bidirectional reconciliation with a fixed-shape write census and bounded
  anomalies, the per-shard subsets B4 consumes, and the per-shard pointer
  solver results.

## Digest definitions

All SHA-256.

- **record frame**: `u32be(len(key)) ‖ key ‖ u64be(len(value)) ‖ value`.
- **section digest**: `SHA-256("hmr1/section/" ‖ name ‖ 0x00 ‖ concat(record frames in key order))`.
- **package digest**: SHA-256 of the `.hmr` file bytes.
- **reference digest**: SHA-256 of the exact `metadata-<target>.reference.json`
  bytes (embeds the package digest + ruleset version). This is what release
  notes publish and B4's pre-mutation check, the `COMPLETE` marker, and the
  D1 startup gate bind.
- **wrapper-set digest**: `SHA-256("hmr1/wrappers" ‖ 0x00 ‖ concat(addr(20) ‖ u64be(len(code)) ‖ code))`
  in normalized list order; `code` = raw stored wrapper bytes at the target
  root (decode-validated, hashed unre-encoded).
- **diagnostics digest**: SHA-256 of the canonical JSON of the
  chain-deterministic findings, sorted by `(code, key)`.

## Exit codes

`0` OK · `13` unsafe DB open / concurrent writer / missing `LOCK` · `14` I/O
or corruption · `15` invalid config / path overlap (validation inside
`RunE`; cobra flag-parse / unknown-command errors exit `2` at the landed
root) · `16` interrupted · `20` `MISSING_REQUIRED_METADATA` (clean-DB
fallback signal) · `21` `INVALID_RETAINED_METADATA` (fatal corruption, incl.
`NoncanonicalKey`) · `22` `TARGET_STATE_UNAVAILABLE` · `23`
`DETERMINISM_MISMATCH` · `24` `AUDIT_ANOMALY` (incl. `POINTER_AMBIGUOUS`) ·
`130` SIGINT.

**Precedence** for mixed outcomes (highest wins):
`130 > 16 > 15 > 13 > 14 > 22 > 21 > 20 > 24 > 23 > 0`. Corruption (`21`)
deliberately outranks the fallback signal (`20`): a corrupt DB must be
investigated, not routed to the clean-DB path. Reports always carry every
finding regardless of the winning code.

## Consumer interface contract

The shared normalization library `internal/recovery/metadata/norm` is THE
single implementation for scan, export, apply (B4) and verify. Its behavior
is fixed by the anchor plus the `ruleset_version` constant (`hmr-norm-v1`) —
no policy knobs (stats kept untouched, target reward accumulator included).

- **`norm.Normalize(a Anchor, s Sources) (*Result, error)`** reads raw keys
  with strict error-checked iterators (never the fail-open `rawdb` readers),
  the target `*state.DB` opened at the anchor root (`snaps=nil`), a
  best-effort historical opener, and a header reader for the boundary block.
  It performs zero writes.
- **`DeletionPlan`** phases map to B4's journal exactly:
  `DVL_SANITIZING`, `SNAPSHOT_SANITIZING`, `EPOCH_SANITIZING`,
  `LOOKUP_AND_CANONICAL_CLEANUP` — **no stats phase** (§8 Q4). The last phase
  carries only `audit-input-required` placeholders (shard-1 subsets, the
  derived pointer) plus B4-owned canonical cleanup, never computed here.
- **B4 reference-comparison rule** (handoff §2.4): re-derive locally under
  the same `ruleset_version`; match the reference digest
  (= SHA-256 of `metadata-<target>.reference.json`); recompute the package
  digest from the payload; **refuse before any mutation on mismatch**. B4
  never installs `.hmr` records — it re-derives; the `.hmr` exact records
  exist because B4 needs byte-exact values, and the audit reconciliation
  validates the source-specific deletion plan.
- **Audit shard-1 subset / pointer contract for B4**: `abandoned-branch-audit.json`
  carries, per non-beacon shard, the branch-written crosslink and
  `cxReceiptSpent` key sets and the derived last-crosslink pointer (invariant
  solver, §4.4), or `POINTER_AMBIGUOUS` with no value (operator investigates,
  supplies `--trusted-shard1-pointer`). B4 consumes the scan + audit reports
  together.
- **`COMPLETE` marker (B4 writes, D1 verifies)**: anchor-config digest,
  reference digest, tool digest, normalized-output digest.
- **Preflight (C)**: assumed passed; **no receipt coupling** and nothing may
  depend on the preflight receipt (§8 Q2). Scan prints the normalized
  list length for a manual, informational comparison only.
- **Orchestration (E)**: quarantine `transactions.rlp` and the staged-sync
  progress cache (a separate `kv.RwDB`, not `harmony_db_0`) before apply
  (handoff §9 / §2.3).

## Run-once runbook

On `mhe-snaps0-01`, with the BLS build environment sourced:

1. **Stop the node** and confirm no process holds `harmony_db_0` (the strict
   opener's shared `flock` fails otherwise).
2. `metadata scan --db harmony_db_0 --anchor recovery-anchor.json --report scan.json`.
   Review: verdict OK, zero-write proof true, the fallback signals absent,
   the normalized list length matches the preflight expectation (manual).
3. `metadata export-reference --db harmony_db_0 --anchor recovery-anchor.json --out-dir out/`.
   The double-run self-check runs automatically. Confirm exit 0 and that
   `out/release/` exists holding `metadata-92730034.hmr` +
   `metadata-92730034.reference.json` + `run-checksums.sha256` + the
   success `export-report.json` (the directory appears only on a fully
   clean run, complete or not at all); note the reference digest.
4. `metadata audit-branch --db harmony_db_0 --anchor recovery-anchor.json --out-dir out/ --scratch scratch/`.
   Two passes run. Confirm exit 0 (no `AUDIT_ANOMALY`), all roots matched,
   the epoch-3003 next-epoch records byte-equal the source's, and the
   reconciliation closed. On `POINTER_AMBIGUOUS`, investigate and re-run with
   `--trusted-shard1-pointer`.
5. **Final sealing step** (§4.7 tier b): stage exactly the four release
   files in a directory and regenerate `SHA256SUMS` over them:

   ```bash
   mkdir -p sealed && cp recovery-anchor.json out/release/metadata-92730034.hmr \
     out/release/metadata-92730034.reference.json out/abandoned-branch-audit.json sealed/
   ( cd sealed && sha256sum recovery-anchor.json metadata-92730034.hmr \
       metadata-92730034.reference.json abandoned-branch-audit.json > SHA256SUMS \
     && sha256sum -c SHA256SUMS )
   ```

   Anything else in `sealed/` fails the seal. Sealing exists as a separate
   step because export runs before the audit artifact exists.
6. Attach the four files + `SHA256SUMS` to the `v2026.1.2-recovery` release
   and publish the reference digest in the release notes.

## Release artifacts

Exactly four files, sealed by one `SHA256SUMS`:

1. `recovery-anchor.json` (its SHA-256 is embedded in the `.hmr` header and
   the reference manifest, so the exact file ships).
2. `metadata-92730034.hmr`.
3. `metadata-92730034.reference.json` (its SHA-256 is the reference digest).
4. `abandoned-branch-audit.json`.

Per-run scan/export reports and the scratch directory stay internal.

## Performance pilot gate

Owner: **operator devops** (§8 Q10). Before the real run, on
`mhe-snaps0-01`-class hardware, run a full `metadata scan` plus a bounded
`audit-branch` slice **at least through block `92,733,440`** (both passes) —
covering the epoch-3003 snapshot batch (`92,733,438`) and the 3002→3003
transition/election, the most expensive work in the range. Record and
confirm/revise in writing:

| Metric | Budget (estimate until the pilot) | Measured |
|--------|-----------------------------------|----------|
| Full scan wall-clock | ≤ 2 h | |
| Scan peak RSS | < 16 GB | |
| Audit wall-clock per pass | ≤ 24 h | |
| Audit peak RSS | < 64 GB | |
| Scratch high-water mark | (≥ 200 GB reserve enforced) | |
| Audit throughput | (steady-state blocks/s) | |
| Host spec | — | |

The real run and the release artifacts wait for the pilot report.

## Operator decision log

Binding; supersedes the handoff where they conflict (plan §8, answered
2026-08-13). Reviewers: treat the removals as decided trade-offs, not gaps.

1. **Two donors → single machine.** Run once on `mhe-snaps0-01`
   (non-reverted). `metadata compare` / two-donor convergence removed;
   assurance = double-run self-check + junk-insensitivity. Residual: no
   independent-hardware corroboration; corruption that passes structural
   validation would not be caught by a second machine. Accepted.
2. **Preflight receipt gating → dropped**, optional hook removed. Assume
   preflight passed. Scan prints the list length for a manual comparison.
   Residual: list completeness rests on preflight. Accepted.
3. **Anchor ceremony → dropped; hash supplied.** Plain-JSON config
   cross-verified at run time; integrity = SHA-256 checksums + report hash
   chaining. Residual: authenticity rests on the release process. Accepted.
4. **validator-stats → keep, untouched.** Not in the `.hmr`, the deletion
   plan, or the absence assertions; scan reports a count + informational
   digest; audit inventories (never reconciles) branch stats writes.
5. **Reward accumulator → include.** `blk-rwd-<target>` is a mandatory
   `.hmr` section.
6. **LastCommits → delete, approved.** Dead legacy key; B4 writes the exact
   `block-sig-<target>` so nothing ever falls back.
7. **Producer/preflight ownership resolved.** Two former coordination asks
   became exactly two edits to preflight-owned files: the `main.go`
   registration + root help-text edit (pinned by the WS1 golden test) and
   the code-verified `api/service/prometheus` guard exemption in
   `deps_guard_test.go` (WS6).
8. **Trusted shard-1 pointer → keep solver + escape hatch.** Operator
   investigates only on `POINTER_AMBIGUOUS`.
9. **Audience → internal, run once; outputs attached to the release.**
   Compare removed; one best-effort snapshot behavior with coverage
   reporting; deliverables sealed by one `SHA256SUMS`.
10. **Performance pilot → keep; owner operator devops.**
11. **Two-binary naming → intentional.** `harmony-recovery` (this plan) vs
    the producer's `harmony-recovery-db`.

## Handoff-vs-code discrepancy register

1. The `ss` schema comment claims `num+hash`; the actual key is
   `ss`+`epoch.Bytes()`. Cosmetic.
2. VRF/VDF and epoch-block-number deletion rules target dead-writer
   namespaces; retained defensively, scan reports observed counts (expected
   zero for epochs > target).
3. The handoff's audit prose implies stock validation can execute the
   branch; it cannot — hence the masked overlay + `InsertChain` + separate
   record-mode checks (which include `Engine.VerifyHeader`, because
   `InsertChain(..., false)` skips general header verification).
4. "Last/continuous pointers" maps to the `cl`+shard(4) crosslink pointer;
   the leader-rotation `continuous` key is unused legacy (reported by scan,
   deletion at B4's discretion).
5. Stats: handoff offers delete-or-reset; operator decided **keep** (§8 Q4).
6. Two-donor convergence and signing ceremonies dropped (§8 Q1/Q3);
   replaced by the double-run self-check and junk-insensitivity fixtures.

## Fixture regeneration

The acceptance suite generates fixtures in-process (deterministic block
times + fixture-only BLS secrets from the public `.hmy` dev keys). To write a
kit for inspection or the pilot:

```bash
scripts/recovery/gen-metadata-fixtures.sh [out-dir]
```

The chain is byte-reproducible; two generations export a byte-identical
`.hmr` and reference manifest (`acceptance.TestGeneratorDeterministic`).
