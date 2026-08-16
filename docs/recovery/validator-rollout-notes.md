# Validator rollout notes — shard-0 clean-DB recovery (92,730,034)

Operator notes for the optional two-command recovery script
(`scripts/rollback-92730034.sh`).
Companion note on the node binary: `docs/recovery/recovery-binary-delta.md`.
Shard-1 operations use a separate script and runbook:
`docs/recovery/validator-rollout-notes-s1-94978278.md`. Do not combine the
commands or state directories from the two procedures.

The script is optional. The equivalent manual operation is to stop the
selected shard-0 validator, replace `harmony_db_0` with the frozen clean
database, run the official v2026.1.2 binary, and start only after coordinated
GO. The script automates those steps with checks and a stopped-until-GO hold.

## 1. Frozen recovery profile

- Network: Harmony mainnet.
- Shard: `0`.
- Database replaced: `harmony_db_0`.
- Retained block: `92,730,034`.
- Retained hash:
  `0x30c35d2f2291e4b27debe7862956cf7a0cc7abefc044273d6823567335086d8d`.
- Rejected original child: block `92,730,035`,
  `0x5de06979a333f20afb8b245a8cf44472dc5bfc7383a57ddee48e1809bcee7c5d`.
- Recovery ViewID floor: `1,000,000,000`.
- Clean database source: `http://snapdb.s0.t.hmny.io/webdav`.
- Frozen source inventory: `184,510` files and `371,422,947,984` logical
  bytes (about 345.9 GiB).
- Installer output ID: `recovery-92730034`.

The source must remain unchanged between freeze and GO. The script checks
the exact count and byte total before and after every transfer and refuses a
changed source.

The normal disk gate requires one replacement database plus a 10% margin:
about 381 GiB of additional free space per service. Use 410 GB as the simple
operator requirement. If several services use the same filesystem, allow
that space for each service that has not already staged its copy.

## 2. Release and publication checklist

The script stages the ordinary official v2026.1.2 `harmony-amd64` or
`harmony-arm64` binary. There is no separate recovery binary. v2026.1.2
contains the rejected-child rule and the recovery ViewID floor for shard 0.

Before sending validator instructions:

1. Re-run `rclone size --json` against the frozen source and require exactly
   `184510` files and `371422947984` bytes.
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
https://raw.githubusercontent.com/harmony-one/harmony/refs/heads/rollback-92730034/scripts/rollback-92730034.sh
```

## 3. Validator message template

---

We need volunteers to test the shard-0 recovery script.

This script is published in Harmony's official `harmony-one/harmony`
repository. During testing, we may patch it through pull requests to improve
compatibility based on validator feedback.

This version supports:

- Manual validators.
- Standard `harmony.service` validators.
- Validators with custom systemd service names.
- Hosts running multiple Harmony services.

What it does:

1. Finds the selected validator's process, config, database directory,
   owner, RPC port, and public BLS keys.
2. Checks available disk space.
3. Downloads the 371 GB clean shard-0 database while the selected validator
   remains online. Interrupted downloads resume file by file.
4. Downloads the official Harmony v2026.1.2 binary and verifies its
   SHA-256 and CPU architecture.
5. Stops the selected validator after the download finishes.
6. Checks that no other process uses the selected validator's config,
   DataDir, or open database files. Separate systemd services may use the
   same Harmony binary.
7. Renames the existing `harmony_db_0` to
   `harmony_db_0.pre-recovery-<timestamp>`. It is not deleted by default.
8. Moves the clean database into place and prepares v2026.1.2.
9. Leaves the selected validator stopped and prints a `READY` line.

The script does not modify validator keys, the Harmony config, or the
original Harmony binary. It does not run a chain revert.

Requirements:

- About 410 GB of additional free space for each shard-0 service being
  prepared. If several services use the same disk, allow space for each one.
- A stable SSH session, tmux, or screen.
- Manual validators must disable cron, supervisor, or anything else that
  could restart Harmony automatically.
- Systemd service names, DataDirs, RPC ports, and BLS keys must be distinct
  when several validators run on one host.
- Run multiple recovery commands sequentially, never in parallel.
- Do not run the shard-0 and shard-1 recovery scripts in parallel.
- Do not run this command against a shard-1 service.
- Do not run any `start` command until the Harmony team sends GO.

Download the current testing script from Harmony's recovery branch:

```bash
curl -fsSL 'https://raw.githubusercontent.com/harmony-one/harmony/refs/heads/rollback-92730034/scripts/rollback-92730034.sh' -o rollback-92730034.sh
```

Keep this downloaded file for both `prepare` and `start`. Re-download it only
when the Harmony team explicitly asks you to pick up a testing fix.

Choose the command matching the validator layout.

Manual validator:

Run as the same non-root user that currently runs Harmony. Run it from the
directory containing the Harmony binary or config.

```bash
cd ~/harmony
```

```bash
bash ./rollback-92730034.sh prepare
```

Standard systemd validator:

The script may be downloaded to any persistent directory.

```bash
sudo bash ./rollback-92730034.sh prepare --systemd-unit harmony.service
```

Custom systemd service:

Replace `YOUR_SHARD0_SERVICE.service` with the exact service name. Ask the
Harmony team if the name is uncertain; do not guess.

```bash
sudo bash ./rollback-92730034.sh prepare --systemd-unit YOUR_SHARD0_SERVICE.service
```

Multiple shard-0 services on one host:

Run a separate command for each selected service, one at a time:

```bash
sudo bash ./rollback-92730034.sh prepare --systemd-unit FIRST_SHARD0_SERVICE.service
```

```bash
sudo bash ./rollback-92730034.sh prepare --systemd-unit SECOND_SHARD0_SERVICE.service
```

Services that are not being recovered need no command and may remain
running. In particular, do not target co-resident shard-1 services with this
script.

If an earlier script already downloaded the database but stopped with
`duplicate-process`, replace it with the current script and rerun `prepare`
with the exact service name. It should reuse the staged database.

Low-space recovery:

Do not use `--discard-old-db` unless the Harmony team instructs you to. It
permanently deletes the old `harmony_db_0` and stops Harmony before
downloading the replacement. The script shows the resolved path and requires
`y` before deletion.

Manual low-space command, run as the normal validator user from its Harmony
directory:

```bash
bash ./rollback-92730034.sh prepare --discard-old-db
```

Systemd low-space command:

```bash
sudo bash ./rollback-92730034.sh prepare --systemd-unit YOUR_SHARD0_SERVICE.service --discard-old-db
```

If `harmony_db_0` was already deleted, recovery is supported only for a
selected, fully stopped systemd service using `--discard-old-db`. The result
is `READY unknown recovery-92730034` and must not count toward GO.

What to expect:

- The script displays each step in the terminal.
- During download, it reports transferred bytes, speed, percentage, and ETA
  every 10 seconds.
- If packages are missing, it prints an install command. Install them and
  rerun the same recovery command.
- The selected validator remains online until the clean database has
  downloaded, except in approved low-space mode.
- The selected validator is stopped before `harmony_db_0` is replaced.
- The selected validator remains stopped after `prepare`.
- Each successful service prints its own `READY` line.

Send the complete final line to the Harmony team:

```text
READY <bls-ids> recovery-92730034
```

If it fails, send the complete `STOPPED` line:

```text
STOPPED <reason> <log-id>
```

Do not run these until the Harmony team explicitly sends GO.

Manual validator:

```bash
bash ./rollback-92730034.sh start
```

Systemd validator:

```bash
sudo bash ./rollback-92730034.sh start --systemd-unit YOUR_SHARD0_SERVICE.service
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
- Tally the selected union against epoch-3002 shard-0 effective voting
  power.
- Require strictly more than two-thirds. Aim for 75–80% to tolerate failed
  starts.
- `READY unknown recovery-92730034` must not count toward the tally.
- Confirm duplicate or backup signers are stopped.
- Send GO only to the selected services. Every other READY service remains
  stopped.

## 5. Old database and quarantine cleanup

After normal `prepare`, the paths are:

```text
<DataDir>/harmony_db_0                          current clean shard-0 DB
<DataDir>/harmony_db_0.pre-recovery-<timestamp> old shard-0 DB
<DataDir>/pre-recovery-<timestamp>/             tx journal/sync-cache quarantine
```

Do not confuse the old database with the separate quarantine directory.

Keep the old database only if the team still needs a forensic copy. After
READY, it may be deleted with team approval. To delete it while keeping the
recovery state accurate, rerun `prepare` with the same service and
`--discard-old-db`:

```bash
sudo bash ./rollback-92730034.sh prepare --systemd-unit YOUR_SHARD0_SERVICE.service --discard-old-db
```

For a manual validator, run the equivalent command without `sudo` or a
service selector, from the original Harmony directory.

For an already READY service, this does not download the clean database
again. It deletes only the recorded
`harmony_db_0.pre-recovery-<timestamp>` and returns to READY.

The old database may also be deleted directly by its exact timestamped path.
Direct deletion does not affect `start`, but the recovery state continues to
record the old database as kept.

The `pre-recovery-<timestamp>/` quarantine contains the post-incident
transaction journal and sync cache, not a database backup. It is not needed
for `start` and may be deleted after READY if the team no longer needs it.

Never delete the active `harmony_db_0`.

If `harmony_db_0` was deleted before the first run, recovery is supported
only for a selected, fully stopped systemd service using
`--discard-old-db`. Without live RPC identity, the result is
`READY unknown recovery-92730034` and must not count toward GO.

## 6. STOPPED-reason triage

- `unsupported-platform`: not Linux x86_64 or Linux arm64.
- `missing-dependencies`: install the packages printed immediately above the
  final line and rerun the same command.
- `deletion-cancelled`: the operator did not confirm the exact database
  deletion path. The old database remains intact.
- `needs-root`: a systemd layout was run without root, or the files/process
  belong to another user.
- `unsupported-layout`: ambiguous process or service, wrong network or role,
  archive/sharded configuration, unexpected shard database, unavailable RPC,
  conflicting systemd drop-in, or unsafe path/arguments.
- `low-disk`: insufficient room for the replacement plus margin. Do not use
  `--discard-old-db` without explicit approval.
- `source-mismatch`: the SnapDB source no longer has the frozen file count
  and byte total. Do not retry blindly.
- `download-failed`: source or binary download failed, or the downloaded
  binary is the wrong ELF architecture.
- `checksum-mismatch`: the Harmony binary does not match its pinned SHA-256.
- `db-verify-failed`: the staged or installed database failed count, size,
  filename, file-type, `CURRENT`, or `MANIFEST` checks.
- `stop-failed`: the selected validator could not be proven inactive.
- `duplicate-process`: another process uses the selected config, DataDir, or
  open `harmony_db_0` files.
- `head-mismatch`: the node reported the wrong retained hash. The mismatch is
  latched and the node remains stopped until the team investigates.
- `not-ready` or `receipt-mismatch`: start was attempted before READY or the
  service no longer matches the recorded recovery command.
- `start-failed` or `unhealthy`: the node failed startup or the post-start
  height, target hash, or BLS checks.
- `cannot-determine-state`: persistent state, filesystem, or lock ownership
  is inconsistent. Check whether the selected service is running before
  changing anything.

Run logs:

```text
/var/lib/harmony-recovery-92730034/units/<service>/private/run-<log-id>.log
/var/lib/harmony-recovery-92730034/private/run-<log-id>.log
<invocation-dir>/.hmy-recovery-92730034/work/private/run-<log-id>.log
```

## 7. Verification and accepted limits

The WebDAV backend reports no content hashes. Do not describe the database
copy as checksum-verified. The script instead requires:

1. The team-approved frozen source.
2. Exact remote count and logical bytes before and after transfer.
3. A flat, regular-file-only goleveldb tree with exactly one manifest and a
   valid `CURRENT`.
4. A SHA-256- and architecture-pinned Harmony binary.
5. Pre-change local RPC access and a valid public BLS key set.
6. Post-start RPC proof of the retained height and hash and the same BLS key
   set.

A same-size content substitution can pass the physical count and size checks.
The node failing to open the database or failing the post-start semantic
checks is the remaining detection path.

Manual-directory autostart prevention remains an operator responsibility.
Rootless `/proc` inspection is best effort across other users. Multiple
systemd services may use identical Harmony binaries, but they must not share
config, DataDir, RPC port, or BLS keys.

## 8. Canary and post-GO checks

Before rollout:

1. Run the smoke suite's manual and systemd groups.
2. Run `prepare` to READY on a real disposable shard-0 validator copy.
3. Confirm keys, config, and the original binary are unchanged.
4. Reboot and confirm the selected validator remains stopped.
5. Rehearse `start` only on a disposable copy before production GO.

After GO:

- Block `92,730,035` must have parent
  `0x30c35d2f2291e4b27debe7862956cf7a0cc7abefc044273d6823567335086d8d`.
- Its hash must differ from rejected hash
  `0x5de06979a333f20afb8b245a8cf44472dc5bfc7383a57ddee48e1809bcee7c5d`.
- Several following shard-0 blocks must finalize on the replacement chain.
- Spot-check RUNNING services against the READY BLS tally.
- Confirm co-resident shard-1 services remain healthy.
