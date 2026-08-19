# `harmony-recovery preflight` — shard-0 recovery eligibility check

One page for validators. The tool answers a single question: **does your
shard-0 database still hold everything needed to recover to block
92,730,034?** It checks the target block's header, commit certificate,
ancestry back to the epoch-3002 boundary, and walks the complete target
state (every account, every storage slot, every contract and validator code
blob, every trie node cryptographically authenticated).

## You do NOT need to stop your node

Run it against the live database. The tool opens the LevelDB strictly
read-only, takes no lock, and **never writes to the database** — the node
does not notice it. A PASS on a live node is a *point-in-time sample*: it
tells the coordinators your DB is a viable recovery source right now. The
authoritative verification is repeated at apply time on a stopped node; the
preflight is the fleet-wide eligibility survey, not the final gate.

## How to run

```bash
./harmony-recovery preflight --db /path/to/harmony_db_0 --name "my-validator"
```

- `--db` must point at the node's `harmony_db_0` directory itself (the
  basename is checked — a renamed copy or a wrong-shard `harmony_db_1` is
  refused). Only the default single-LevelDB layout is supported; sharded
  (`harmony_sharddb_*`), pebble and TiKV layouts are refused.
- `--name` is optional but appreciated: coordinators use it to attribute
  your result.
- The target height and hash are compiled into the binary — there is nothing
  to configure and nothing to download.
- Defaults: `--network mainnet --shard 0`,
  receipt written to `preflight-result.json` in the current directory
  (`--report` to change it; a path inside the database directory is
  refused).
- Tuning (rarely needed): `--storage-workers`, `--handles`, `--db-cache-mb`,
  `--trie-cache-mb`. If the tool complains about `RLIMIT_NOFILE`, raise
  `ulimit -n` or lower `--handles`.
- Test-only (hidden, refused on mainnet): `--target-height`,
  `--target-hash`, non-mainnet `--network` values for fixtures.

## What to report

The last line on stdout is exactly one of:

```
PASS
FAIL: <one-line reason>
```

Post that line in the Telegram channel and **attach the JSON receipt**
(`preflight-result.json`). The receipt contains the check-by-check results,
state counts, a deterministic state digest (identical digests across
validators are a free cross-check for the coordinators), and a bounded list
of informational anomalies. It contains no keys and no personal data beyond
the hostname and the `--name` you chose.

## Exit codes and the retry remedy

| code | meaning |
|------|---------|
| 0 | PASS — all checks passed (point-in-time sample if the node was running) |
| 1 | FAIL — a verification check failed; the one-line reason says which |
| 2 | unusable — bad flags, missing/unsupported database layout |
| 3 | persistent read error after retries (the receipt carries `result: FAIL` with `exit_code: 3`) |

On exit 3 the remedy is: **re-run the tool; if it keeps failing, stop the
node briefly and re-run.** A live database can relocate files while the tool
reads (the tool retries these races automatically); persistent failures
usually mean actual disk-level corruption — the receipt names the corrupt
file.

## Runtime expectation

On the pilot host (fill in after the single-DB pilot: wall-clock, peak RSS,
`--handles` used, whether the node was running), a full run takes
**PILOT-PENDING**. Machines with slower disks scale roughly with the size of
`harmony_db_0`. Progress is printed to stderr while it runs.
