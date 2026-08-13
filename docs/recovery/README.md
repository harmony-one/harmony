# Shard-0 clean-database recovery operator guide

This guide is for the internal `harmony-recovery-db` producer pipeline. It
builds a clean validator `harmony_db_0` at the pinned recovery target
**92,730,034** from:

- the full Aug 8 shard-0 snapshot; and
- a forensic shard-0 node whose database has advanced past the target (the
  currently available forensic node is at 92,805,158).

The producer works entirely offline. It does not connect to peers or start
consensus. It exports blocks and certificates from the forensic database,
replays them into an Aug 8 working restore, compacts the result, deeply
verifies it, and seals a checksummed release directory.

Run every phase separately and inspect its gate before continuing. A nonzero
exit is a stop condition; do not improvise around it.

## Safety rules

1. Never run a producer command against a live Harmony database. Stop the
   node or use a crash-consistent, read-only snapshot.
2. Restore the Aug 8 snapshot twice:
   - `AUG8_WORK` is mutated by replay.
   - `AUG8_REFERENCE` remains immutable and supplies the agreement check.
   A read-only snapshot plus a writable CoW clone is acceptable. Pointing
   both variables at one directory defeats the agreement gate.
3. Do not boot a fresh Harmony node after restoring `AUG8_WORK`. Restore the
   files and leave the service stopped; `harmony-recovery-db` performs the
   replay itself.
4. The forensic head may be above the pinned target. The exporter reads only
   `baseline+1 .. 92,730,034`, plus child 92,730,035 for the target
   certificate.
5. Do **not** use `docs/recovery/recovery-anchor.mainnet.json` as
   `harmony-recovery-db`'s anchor. That file belongs to the separate
   `harmony-recovery` in-place tool and has a different schema. This guide
   creates `producer-anchor.json`.
6. Leave `METADATA_REFERENCE_MANIFEST` unset. The in-place metadata tool's
   `hmr-reference-v1` output is not the optional
   `hmy-recovery-metadata-reference-v1` input accepted by this producer.
7. A failed or interrupted replay/compaction destination is not resumable.
   Restore/recreate the destination and start that mutating phase again.
8. Do not restart validators from the sealed database until the coordinated
   recovery restart is announced.

## Host and storage assumptions

Use a Linux producer host with approximately:

- 8 TB total storage and at least 3.2 TB free after restoring the working DB;
- 16 or more CPU cores;
- 64 GB or more RAM; and
- `bash`, `jq`, and `sha256sum`.

The replayed working database can remain several TB. The compact validator
artifact has a hard 200 GB release gate, and packaging needs space for another
copy of that compact artifact.

If the forensic database cannot be mounted locally, perform only the export
phase on the stopped forensic host and transfer the checksummed bundle to the
producer. The remainder of the workflow runs on the producer.

## 1. Define producer paths

Edit the path values in this block. All paths passed to the producer must be
absolute.

```bash
set -euo pipefail

export REPO="/opt/harmony"
export RECOVERY_ROOT="/recovery"

# Two separate restores/clones of the Aug 8 snapshot.
export AUG8_WORK="$RECOVERY_ROOT/aug8-working/harmony_db_0"
export AUG8_REFERENCE="$RECOVERY_ROOT/aug8-reference/harmony_db_0"

# A stopped/cold copy of the forensic node's harmony_db_0.
export DONOR="$RECOVERY_ROOT/forensic-92805158/harmony_db_0"

export RECOVERY_REPORT_DIR="$RECOVERY_ROOT/reports"
export BUNDLE_DIR="$RECOVERY_ROOT/bundle-92730034"
export COMPACT_DB="$RECOVERY_ROOT/compact/harmony_db_0"
export RELEASE_ROOT="$RECOVERY_ROOT/releases"

export NETWORK="mainnet"
export SHARD="0"
export TARGET_HEIGHT="92730034"

export COPY_A="$AUG8_WORK"
export COPY_B="$AUG8_REFERENCE"
export WORKING_DB="$AUG8_WORK"

mkdir -p \
  "$RECOVERY_REPORT_DIR" \
  "$(dirname "$BUNDLE_DIR")" \
  "$(dirname "$COMPACT_DB")" \
  "$RELEASE_ROOT"

for db in "$AUG8_WORK" "$AUG8_REFERENCE" "$DONOR"; do
  test -f "$db/CURRENT" || {
    echo "Not a LevelDB harmony_db_0: $db" >&2
    exit 1
  }
done

test ! -e "$BUNDLE_DIR"
test ! -e "$COMPACT_DB"

unset METADATA_REFERENCE_MANIFEST || true
unset RECOVERY_HARMONY_BINARY_SHA256 || true
unset PROVISIONAL_START_VIEW_ID || true
```

## 2. Build the producer

Build on Linux so the resulting binary can also be copied to the forensic
host if export must run there.

```bash
cd "$REPO"

source scripts/setup_bls_build_flags.sh ""
scripts/go_executable_build.sh harmony-recovery-db

export HARMONY_RECOVERY_DB_BIN="$REPO/bin/harmony-recovery-db"
test -x "$HARMONY_RECOVERY_DB_BIN"
"$HARMONY_RECOVERY_DB_BIN" --version
```

## 3. Discover the actual Aug 8 baseline

The presumed Aug 8 head is 92,591,097, but the workflow always uses the head
read from the restored database.

```bash
"$HARMONY_RECOVERY_DB_BIN" inspect-db \
  --network mainnet \
  --shard 0 \
  --db "$AUG8_WORK" \
  --read-only \
  --output "$RECOVERY_REPORT_DIR/discovery.json"

jq -e '.heads_agree and .canonical_head_match' \
  "$RECOVERY_REPORT_DIR/discovery.json"

export BASELINE_HEIGHT="$(
  jq -r '.heads[] | select(.key == "LastBlock") | .height' \
    "$RECOVERY_REPORT_DIR/discovery.json"
)"
export BASELINE_HASH="$(
  jq -r '.heads[] | select(.key == "LastBlock") | .hash' \
    "$RECOVERY_REPORT_DIR/discovery.json"
)"
export GENESIS_HASH="$(
  jq -r '.genesis_hash' "$RECOVERY_REPORT_DIR/discovery.json"
)"

(( BASELINE_HEIGHT < TARGET_HEIGHT ))

printf 'Baseline: %s %s\n' "$BASELINE_HEIGHT" "$BASELINE_HASH"
printf 'Replay range: %s..%s\n' "$((BASELINE_HEIGHT + 1))" "$TARGET_HEIGHT"
```

## 4. Create the producer anchor

The zero/empty target-derived fields below are explicitly supported as
not-yet-filled fields. The tool still pins and checks the target, parent and
abandoned-child hashes compiled into the incident build.

```bash
export ANCHOR="$RECOVERY_REPORT_DIR/producer-anchor.json"

cat >"$ANCHOR" <<JSON
{
  "schema_version": "hmy-recovery-anchor-v1",
  "network": "mainnet",
  "shard_id": 0,
  "genesis_hash": "$GENESIS_HASH",
  "target_height": 92730034,
  "target_hash": "0x30c35d2f2291e4b27debe7862956cf7a0cc7abefc044273d6823567335086d8d",
  "target_parent_hash": "0x14e2bcbb4aba7e04e13fd6fdb8427632e942403e58dcc9f0c412bb0c7a38951e",
  "target_state_root": "0x0000000000000000000000000000000000000000000000000000000000000000",
  "target_epoch": 3002,
  "target_view_id": 0,
  "baseline_height": $BASELINE_HEIGHT,
  "baseline_hash": "$BASELINE_HASH",
  "abandoned_child_height": 92730035,
  "abandoned_child_hash": "0x5de06979a333f20afb8b245a8cf44472dc5bfc7383a57ddee48e1809bcee7c5d",
  "rejected_shard1_height": 94978279,
  "rejected_shard1_hash": "0xc936581d391b74a620bf6636519834b14a9a2d4e9a5154867c8407f219d8a878",
  "target_certificate_sha256": "",
  "shard_state_digest_sha256": "",
  "known_bad": []
}
JSON

jq -e . "$ANCHOR" >/dev/null

(
  cd "$RECOVERY_REPORT_DIR"
  sha256sum producer-anchor.json > producer-anchor.json.sha256
  sha256sum -c producer-anchor.json.sha256
)
```

## 5. Deep-inspect both Aug 8 restores

This wrapper performs three full scans: A, B, and A again while producing the
agreement verdict.

```bash
cd "$REPO"
scripts/recovery/10-inspect.sh

jq -e \
  '.heads_agree and .replay_preflight.full_archival and .baseline_gate.passed' \
  "$RECOVERY_REPORT_DIR/inspect-a.json"

jq -e \
  '.heads_agree and .replay_preflight.full_archival and .baseline_gate.passed' \
  "$RECOVERY_REPORT_DIR/inspect-b.json"

jq -e '.agreed == true' "$RECOVERY_REPORT_DIR/agreement.json"

INSPECTED_HEIGHT="$(
  jq -r '.heads[] | select(.key == "LastBlock") | .height' \
    "$RECOVERY_REPORT_DIR/inspect-a.json"
)"
test "$INSPECTED_HEIGHT" -eq "$BASELINE_HEIGHT"
```

Do not continue unless both full-archival gates and the agreement verdict
pass.

## 6. Preflight and export the forensic donor

The database at `DONOR` must be stopped or a crash-consistent snapshot. The
preflight proves that it retains every required canonical header, body and
certificate and that the chain reaches the pinned target hash.

```bash
export BASELINE_REPORT="$RECOVERY_REPORT_DIR/inspect-a.json"
export FROM_HEIGHT="$((BASELINE_HEIGHT + 1))"
export CERT_CHILD_HEIGHT="92730035"
export DONOR_ID="forensic-head-92805158"

scripts/recovery/30-export.sh --report-only

jq -e '
  .passed and
  .gap_count == 0 and
  .canonical_present and
  .headers_present and
  .bodies_present and
  .certificates_present and
  .cert_child_present and
  .chain_walk_ok and
  .target_hash_ok
' "$RECOVERY_REPORT_DIR/export-preflight.json"

scripts/recovery/30-export.sh

(
  cd "$BUNDLE_DIR"
  sha256sum -c SHA256SUMS
)
```

### Exporting on the forensic host instead

To avoid transferring the multi-TB donor DB, copy these files to identical
absolute paths on the stopped forensic host:

- the static `harmony-recovery-db` binary;
- `inspect-a.json` and `inspect-a.json.sha256`;
- `producer-anchor.json` and `producer-anchor.json.sha256`; and
- the repository's `scripts/recovery/` directory, or invoke the equivalent
  `export-bundle` command directly.

Run only this section there. Transfer the resulting `BUNDLE_DIR` back to the
producer and run `sha256sum -c SHA256SUMS` from the transferred directory
before replay.

## 7. Replay into the working Aug 8 restore

The default below reserves 1 TB of free space. Raise it if the storage plan
requires a larger safety margin.

```bash
export MIN_FREE_BYTES="1000000000000"

scripts/recovery/40-replay.sh

EXPECTED_BLOCKS="$((TARGET_HEIGHT - BASELINE_HEIGHT))"

jq -e \
  --argjson target "$TARGET_HEIGHT" \
  --argjson expected "$EXPECTED_BLOCKS" '
    .gate.passed and
    .blocks_replayed == $expected and
    all(.final_heads[]; .height == $target)
  ' "$RECOVERY_REPORT_DIR/replay.json"
```

If replay fails or is interrupted, preserve the reports for diagnosis,
discard the failed working destination, restore a clean Aug 8 working copy,
and regenerate the inspect/agreement evidence for that restore.

## 8. Compact into a fresh validator database

Use the supported internal metadata mode.

```bash
unset METADATA_REFERENCE_MANIFEST || true

test ! -e "$COMPACT_DB"
scripts/recovery/50-compact.sh

jq -e '
  .mode == "internal" and
  .size_gate.passed and
  .journal_state == "COMPLETE_VERIFIED"
' "$RECOVERY_REPORT_DIR/compact.json"
```

A failed size gate produces `COMPLETE_UNRELEASABLE`; do not package that
artifact.

## 9. Deeply verify the compact database

```bash
scripts/recovery/55-verify.sh

jq -e '
  .passed and
  .journal_state == "COMPLETE_VERIFIED" and
  .certificates_verified == 29364
' "$RECOVERY_REPORT_DIR/verification.json"
```

The expected certificate count is the complete retained window
92,700,671 through 92,730,034.

## 10. Seal the release

```bash
scripts/recovery/60-package.sh

export RELEASE_DIR="$(
  jq -er '.release_dir' "$RECOVERY_REPORT_DIR/package.json"
)"

(
  cd "$RELEASE_DIR"
  sha256sum -c SHA256SUMS

  RELEASE_ID="$(jq -r '.release_id' release.json)"
  test "$(cat READY)" = "$RELEASE_ID"

  jq '{
    release_id,
    target_height,
    target_hash,
    payload_bytes,
    payload_files,
    mode,
    logical_kv_digest
  }' release.json
)

printf 'Sealed release: %s\n' "$RELEASE_DIR"
```

The sealed directory contains an instance-specific `INSTALL.md`. Publish the
whole directory; preferably transfer `READY` last as described by the
packager's output.

## 11. Install on a validator and leave it stopped

Edit `RELEASE_DIR` and `DATA_DIR`.

```bash
set -euo pipefail

export RELEASE_DIR="/path/to/sealed/release"
export DATA_DIR="/path/to/harmony/data"

cd "$RELEASE_DIR"
sha256sum -c SHA256SUMS
test "$(cat READY)" = "$(jq -r '.release_id' release.json)"

sudo systemctl stop harmony

if pgrep -x harmony >/dev/null; then
  echo "Harmony is still running; aborting" >&2
  exit 1
fi

OLD_DB="$DATA_DIR/harmony_db_0.pre-recovery.$(date -u +%Y%m%dT%H%M%SZ)"

sudo mv "$DATA_DIR/harmony_db_0" "$OLD_DB"
test ! -e "$DATA_DIR/harmony_db_0"

sudo cp -a payload/harmony_db_0 "$DATA_DIR/harmony_db_0"

grep '^.*  payload/harmony_db_0/' SHA256SUMS \
  | sed "s#  payload/harmony_db_0/#  $DATA_DIR/harmony_db_0/#" \
  | sudo sha256sum -c -
```

Do not restart the validator independently. Keep it stopped until the
recovery coordinator specifies the binary/configuration and announces the
coordinated restart.

## Reports and failure handling

Every report has a sibling `.sha256`, and each downstream phase records the
hash of its inputs:

```text
producer-anchor.json
  -> inspect-a.json / inspect-b.json / agreement.json
  -> bundle manifest
  -> replay.json
  -> compact.json
  -> verification.json
  -> package.json / release.json
```

Keep `RECOVERY_REPORT_DIR`, the bundle, the replayed working database and the
sealed release until the recovery has completed. They are the audit evidence
and allow failures to be diagnosed without trusting terminal logs.

Exit classes:

- `0`: success
- `2`: CLI usage error
- `3`: failed precondition
- `4`: verification failure
- `5`: I/O, locking or corruption failure

For command-level flag details, see `docs/recovery/cli-contract.md`. For the
design and gate rationale, see `docs/recovery/shard0-92730034.md`.
