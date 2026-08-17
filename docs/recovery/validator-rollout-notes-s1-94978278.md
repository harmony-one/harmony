# Validator rollout notes — shard-1 clean-DB recovery (94,978,278)

Operator notes for the optional two-command shard-1 recovery script:
`scripts/rollback-92730034-s1-94978278.sh`.

Shard-0 operations remain documented separately in
`docs/recovery/validator-rollout-notes.md`. Do not combine the commands or
state directories from the two procedures.

The script is optional. The equivalent manual operation is to stop the
selected shard-1 validator, replace only `harmony_db_1` with the frozen clean
database, quarantine the old `harmony_db_0` epoch-chain companion so Harmony
rebuilds it, run the official v2026.1.2 binary, and start only after
coordinated GO. The script automates those steps with checks and a
stopped-until-GO hold.

## 1. Frozen recovery profile

- Network: Harmony mainnet.
- Shard: `1`.
- Database replaced: `harmony_db_1`.
- Companion epoch database: old `harmony_db_0` is quarantined immediately
  before start and Harmony recreates it from genesis.
- Retained block: `94,978,278`.
- Retained hash:
  `0xa25d77e72c7f71f2b18847c7f6a9bbed8af42244915bd9175cc247d157b11b9f`.
- Rejected original child: block `94,978,279`,
  `0xc936581d391b74a620bf6636519834b14a9a2d4e9a5154867c8407f219d8a878`.
- Recovery ViewID floor: `1,000,000,000`.
- Clean database source: `http://fulldb.s1.t.hmny.io/webdav`.
- Frozen source inventory: `32,482` files and `70,073,877,580` logical
  bytes (about 65.3 GiB).
- Installer output ID: `recovery-92730034-s1-94978278`.

The source must remain unchanged between freeze and GO. The script checks
the exact count and byte total before and after every transfer and refuses a
changed source.

The normal disk gate requires one replacement database plus a 20 GiB
margin: about 86 GiB of additional free space per service. Use 100 GB as the
simple operator requirement. If several services use the same filesystem,
allow that space for each service that has not already staged its copy.

## 2. Release and publication checklist

The script stages the ordinary official v2026.1.2 `harmony-amd64` or
`harmony-arm64` binary. There is no separate shard-1 binary. v2026.1.2
contains the rejected-child rule and applies the recovery ViewID floor to
mainnet shard 1.

Before sending validator instructions:

1. Re-run `rclone size --json` against the frozen source and require exactly
   `32482` files and `70073877580` bytes.
2. Re-run the manual and systemd smoke suites.
3. Perform a real canary using a disposable copy of the clean database.
4. Review and push the current testing version to the
   `rollback-92730034` branch in `harmony-one/harmony`.
5. Confirm the branch URL below serves the expected script before sending
   the validator message.

During testing, the validator-facing URL intentionally follows the mutable
Harmony recovery branch so fixes can be published without changing the
instructions:

```text
https://raw.githubusercontent.com/harmony-one/harmony/refs/heads/rollback-92730034/scripts/rollback-92730034-s1-94978278.sh
```

## 3. Validator message template

---

We need volunteers to test the shard-1 recovery script.

This script is published in Harmony's official `harmony-one/harmony`
repository. During testing, we may patch it from time to time through pull
requests to improve compatibility based on your feedback and feedback from
other validators.

This version supports:

- Manual validators.
- Standard `harmony.service` validators.
- Validators with custom systemd service names.
- Hosts running multiple Harmony services.

What it does:

1. Finds the selected validator's process, config, database directory,
   owner, RPC port, and public BLS keys.
2. Requires local RPC to identify a mainnet shard-1 validator before
   changing its database.
3. Checks available disk space.
4. Downloads the 70 GB clean shard-1 database while the selected validator
   remains online. Interrupted downloads resume file by file.
5. Downloads the official Harmony v2026.1.2 binary and verifies its
   SHA-256 and CPU architecture.
6. Stops the selected validator after the download finishes.
7. Checks that no other process uses the selected validator's config,
   DataDir, or open shard-1 database files. Separate systemd services may
   use the same Harmony binary.
8. Renames the existing `harmony_db_1` to
   `harmony_db_1.pre-recovery-<timestamp>`. It is not deleted by default.
9. Keeps `harmony_db_0` in place while preparing, then quarantines it at the
   first recovered `start` so Harmony rebuilds a fresh beacon epoch chain.
10. Moves the clean `harmony_db_1` into place and prepares v2026.1.2.
11. Leaves the selected validator stopped after `prepare` and prints a
    `READY` line.

The script does not modify validator keys, the Harmony config, the original
Harmony binary, or either database's contents in place. It uses directory
renames and does not run a chain revert.

Requirements:

- About 100 GB of additional free space for each shard-1 service being
  prepared. If several services use the same disk, allow space for each one.
- A stable SSH session, tmux, or screen.
- Manual validators must disable cron, supervisor, or anything else that
  could restart Harmony automatically.
- Systemd service names, DataDirs, RPC ports, and BLS keys must be distinct
  when several validators run on one host.
- Run multiple recovery commands sequentially, never in parallel.
- Do not run the shard-0 and shard-1 recovery scripts in parallel.
- Do not run this command against a shard-0 service.
- Do not run any `start` command until the Harmony team sends GO.

Download the current testing script from Harmony's recovery branch:

```bash
curl -fsSL 'https://raw.githubusercontent.com/harmony-one/harmony/refs/heads/rollback-92730034/scripts/rollback-92730034-s1-94978278.sh' -o rollback-92730034-s1-94978278.sh
```

Keep this downloaded file for both `prepare` and `start`. Re-download it
only when the Harmony team explicitly asks you to pick up a testing fix.

Choose the command matching the validator layout.

Manual validator:

Run as the same non-root user that currently runs Harmony. Run it from the
directory containing the Harmony binary or config.

```bash
cd ~/harmony
```

```bash
bash ./rollback-92730034-s1-94978278.sh prepare
```

Standard systemd validator:

The script may be downloaded to any persistent directory.

```bash
sudo bash ./rollback-92730034-s1-94978278.sh prepare --systemd-unit harmony.service
```

Custom systemd service:

Replace `YOUR_SHARD1_SERVICE.service` with the exact service name. Ask the
Harmony team if the name is uncertain; do not guess.

```bash
sudo bash ./rollback-92730034-s1-94978278.sh prepare --systemd-unit YOUR_SHARD1_SERVICE.service
```

Multiple shard-1 services on one host:

Run a separate command for each selected service, one at a time:

```bash
sudo bash ./rollback-92730034-s1-94978278.sh prepare --systemd-unit FIRST_SHARD1_SERVICE.service
```

```bash
sudo bash ./rollback-92730034-s1-94978278.sh prepare --systemd-unit SECOND_SHARD1_SERVICE.service
```

Services that are not being recovered need no command and may remain
running. In particular, do not target co-resident shard-0 services with this
script.

Low-space recovery:

Do not use `--discard-old-db` unless the Harmony team instructs you to. It
permanently deletes only the old `harmony_db_1` and stops Harmony before
downloading the replacement. It does not delete `harmony_db_0`; that
companion is separately quarantined at `start`. The script shows the
resolved shard-1 path and requires `y` before deletion.

Manual low-space command, run as the normal validator user from its Harmony
directory:

```bash
bash ./rollback-92730034-s1-94978278.sh prepare --discard-old-db
```

Systemd low-space command:

```bash
sudo bash ./rollback-92730034-s1-94978278.sh prepare --systemd-unit YOUR_SHARD1_SERVICE.service --discard-old-db
```

If `harmony_db_1` was already deleted, the script cannot use live RPC to
prove the stopped service's shard. It prints the selected unit, executable,
config, DataDir, RPC endpoint, config/command-line shard hints, observed RPC
metadata, and database paths. The operator must independently verify the
selection and type `SHARD1` exactly. This confirmation is mandatory even
with `--quiet`.

What to expect:

- The script displays each step in the terminal.
- During download, it reports transferred bytes, speed, percentage, and ETA
  every 10 seconds.
- If packages are missing, it prints an install command. Install them and
  rerun the same recovery command.
- The selected validator remains online until the clean database has
  downloaded, except in approved low-space mode.
- The selected validator is stopped before `harmony_db_1` is replaced.
- `harmony_db_0` remains in place through `prepare`; the updated `start`
  command quarantines it and requires Harmony to recreate it.
- The selected validator remains stopped after `prepare`.
- Each successful service prints its own `READY` line.

Send the complete final line to the Harmony team:

```text
READY <bls-ids> recovery-92730034-s1-94978278
```

If it fails, send the complete `STOPPED` line:

```text
STOPPED <reason> <log-id>
```

Do not run these until the Harmony team explicitly sends GO.

If this validator reached READY with an earlier shard-1 script, download the
current branch version before running `start`. It reuses the completed
database and existing recovery state, then safely quarantines and rebuilds
`harmony_db_0`.

Manual validator:

```bash
bash ./rollback-92730034-s1-94978278.sh start
```

Systemd validator:

```bash
sudo bash ./rollback-92730034-s1-94978278.sh start --systemd-unit YOUR_SHARD1_SERVICE.service
```

For hosts with multiple prepared services, run a separate `start` command
for each approved service, one at a time.

---

## 4. READY tally and GO

- Keep one row per recovered service with its complete comma-joined BLS key
  set.
- Reject duplicate BLS keys.
- Select services whose complete key sets are pairwise disjoint; starting a
  service activates all of its keys.
- Tally the selected union against epoch-3002 shard-1 effective voting
  power.
- Require strictly more than two-thirds. Aim for 75–80% to tolerate failed
  starts.
- `READY unknown recovery-92730034-s1-94978278` must not count toward the
  tally.
- Confirm duplicate or backup signers are stopped.
- Send GO only to the selected machines. Every other READY machine remains
  stopped.

## 5. Old database and quarantine cleanup

After normal `prepare`, the paths are:

```text
<DataDir>/harmony_db_1                         current clean shard-1 DB
<DataDir>/harmony_db_1.pre-recovery-<timestamp> old shard-1 DB
<DataDir>/harmony_db_0                         old epoch DB before start; rebuilt DB after start
<DataDir>/harmony_db_0.pre-s1-recovery-<timestamp> quarantined old epoch DB
<DataDir>/pre-recovery-<timestamp>/            tx journal/sync-cache quarantine
```

Do not confuse the old database with the separate quarantine directory.

Keep the old database through canary and GO unless the team has already
approved permanent deletion. To delete the recorded old database while
keeping recovery state accurate, rerun `prepare` with the same service and
`--discard-old-db`:

```bash
sudo bash ./rollback-92730034-s1-94978278.sh prepare --systemd-unit YOUR_SHARD1_SERVICE.service --discard-old-db
```

For a manual validator, run the equivalent command without `sudo` or a
service selector, from the original Harmony directory.

For an already READY service, this does not download the clean database
again. It deletes only the recorded
`harmony_db_1.pre-recovery-<timestamp>` and returns to READY.

If `harmony_db_1` was deleted before the first run, recovery is supported
only for a selected, fully stopped systemd service using
`--discard-old-db`. Before changing the DataDir, the script displays all
discovered service/config/database facts and requires the operator to type
`SHARD1` exactly. Without live RPC identity, the eventual result is
`READY unknown recovery-92730034-s1-94978278` and must not count toward GO.

Never delete the active `harmony_db_1`. Do not manually delete
`harmony_db_0`; use the updated `start` command so the old epoch DB is
journaled and quarantined safely. If it was already deleted after READY, the
updated script records that condition and still allows Harmony to rebuild
it. Delete either quarantine only after the team confirms it is no longer
needed.

## 6. STOPPED-reason triage

- `unsupported-platform`: not Linux x86_64 or Linux arm64.
- `missing-dependencies`: install the packages printed immediately above
  the final line and rerun the same command.
- `deletion-cancelled`: the operator did not confirm the exact database
  deletion path. The old database remains intact.
- `shard-confirmation-cancelled`: `harmony_db_1` was already absent and the
  operator did not type `SHARD1` after reviewing the discovered service,
  config, DataDir, shard hints, and database paths. No database path was
  created or replaced.
- `needs-root`: a systemd layout was run without root, or the files/process
  belong to another user.
- `unsupported-layout`: ambiguous process or service, wrong network/shard
  or role over RPC, missing companion `harmony_db_0`, archive/sharded/elastic
  configuration, conflicting systemd drop-in, or unsafe path/arguments.
- `low-disk`: insufficient room for the replacement plus margin. Do not use
  `--discard-old-db` without explicit approval.
- `source-mismatch`: the FullDB source no longer has the frozen file count
  and byte total. Do not retry blindly.
- `download-failed`: source or binary download failed, or the downloaded
  binary is the wrong ELF architecture.
- `checksum-mismatch`: the Harmony binary does not match its pinned SHA-256.
- `db-verify-failed`: the staged or installed database failed count, size,
  filename, file-type, `CURRENT`, or `MANIFEST` checks.
- `stop-failed`: the selected validator could not be proven inactive.
- `duplicate-process`: another process uses the selected config, DataDir, or
  open `harmony_db_1` files.
- `head-mismatch`: the node reported the wrong retained hash or exposed the
  rejected original child. The mismatch is latched and the node remains
  stopped until the team investigates.
- `not-ready` or `receipt-mismatch`: start was attempted before READY or the
  service no longer matches the recorded recovery command.
- `start-failed` or `unhealthy`: the node failed startup or the post-start
  shard identity, height, target hash, rejected-child, or BLS checks.
- `cannot-determine-state`: persistent state, filesystem, frozen profile, or
  lock ownership is inconsistent. Check whether the selected service is
  running before changing anything.

Run logs:

```text
/var/lib/harmony-recovery-92730034-s1-94978278/units/<service>/private/run-<log-id>.log
/var/lib/harmony-recovery-92730034-s1-94978278/private/run-<log-id>.log
<invocation-dir>/.hmy-recovery-92730034-s1-94978278/work/private/run-<log-id>.log
```

## 7. Verification and accepted limits

The WebDAV backend reports no content hashes. Do not describe the database
copy as checksum-verified. The script instead requires:

1. The team-approved frozen source.
2. Exact remote count and logical bytes before and after transfer.
3. A flat, regular-file-only goleveldb tree with exactly one manifest and a
   valid `CURRENT`.
4. A SHA-256- and architecture-pinned Harmony binary.
5. Pre-change RPC proof that the selected service is a mainnet shard-1
   validator, plus its public BLS key set.
6. A journaled rename of the old `harmony_db_0` before recovered launch.
7. Post-start proof that Harmony recreated `harmony_db_0`, plus RPC proof of
   the target hash, absence of the rejected original child, the same shard
   identity, and BLS-key continuity.

A same-size content substitution can pass the physical count/size checks.
The node failing to open the database or failing the post-start semantic
checks is the remaining detection path. This is the same accepted
raw-directory limitation as the shard-0 emergency procedure.

The recreated epoch DB must learn only from recovered/trusted shard-0 peers.
The installer proves recreation, not complete canonical epoch catch-up, so
do not send GO while unrecovered shard-0 peers can repopulate abandoned epoch
metadata.

Manual-directory autostart prevention remains an operator responsibility.
Rootless `/proc` inspection is best effort across other users. Multiple
systemd services may use identical Harmony binaries, but they must not share
config, DataDir, RPC port, or BLS keys.

## 8. Canary and post-GO checks

Before rollout:

1. Run the smoke suite's manual and systemd groups.
2. Run `prepare` to READY on a real disposable shard-1 validator copy.
3. Confirm keys, config, and the original binary are unchanged.
4. Reboot and confirm the selected validator remains stopped.
5. Rehearse `start` only on a disposable copy before production GO; confirm
   the old `harmony_db_0` was quarantined and a new one was created.

After GO:

- Block `94,978,279` must have parent
  `0xa25d77e72c7f71f2b18847c7f6a9bbed8af42244915bd9175cc247d157b11b9f`.
- Its hash must differ from rejected hash
  `0xc936581d391b74a620bf6636519834b14a9a2d4e9a5154867c8407f219d8a878`.
- Several following shard-1 blocks must finalize on the replacement chain.
- Spot-check RUNNING services against the READY BLS tally.
- Confirm co-resident shard-0 services remain healthy and each shard-1
  validator has a newly rebuilt `harmony_db_0`.
