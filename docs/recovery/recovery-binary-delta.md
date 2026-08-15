# Harmony v2026.1.2 binary checklist for recovery

This is a release checklist for the Harmony team. It is not a work log and it
is not the validator runbook.

The optional recovery script, `scripts/rollback-92730034.sh`, downloads and
runs the normal Harmony v2026.1.2 binary. Validator instructions are in
`docs/recovery/validator-rollout-notes.md`.

## Code included in v2026.1.2

Build v2026.1.2 only after these two PRs merge:

- [PR #5106](https://github.com/harmony-one/harmony/pull/5106) rejects the
  abandoned shard-0 child and the known malicious block hashes.
- [PR #5107](https://github.com/harmony-one/harmony/pull/5107) sets the
  recovery ViewID floor to `1,000,000,000`.

These are the only two patch PRs in v2026.1.2. There is no separate recovery
binary.

## What the Harmony team publishes

Publish the normal release files for both supported machines:

1. `harmony-amd64`
2. `harmony-arm64`
3. The SHA-256 value for each file

Before publishing, use `readelf -h` to confirm that each file is a 64-bit ELF
for the correct machine (`x86-64` or `AArch64`).

Put the final download URLs and SHA-256 values in the `NODE_BIN_URL_*` and
`NODE_BIN_SHA256_*` fields in `scripts/rollback-92730034.sh`.

## What the recovery script checks

The recovery script:

1. Selects the correct binary for the validator machine.
2. Checks the binary's SHA-256 and ELF machine type.
3. Starts Harmony with the validator's existing config and data directory.
4. Uses local RPC at `127.0.0.1:9500` to check the validator keys and confirm
   that block `92,730,034` has hash
   `0x30c35d2f2291e4b27debe7862956cf7a0cc7abefc044273d6823567335086d8d`.
5. Stops the node and records `head-mismatch` if that hash is wrong.

The script does not represent the Harmony team. It is only an optional tool
that automates the validator recovery steps.
