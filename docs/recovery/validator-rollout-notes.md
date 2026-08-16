# Validator rollout notes — shard-0 clean-DB recovery (92,730,034)

Operator notes for the optional two-command recovery script
(`scripts/rollback-92730034.sh`).
Companion note on the node binary: `docs/recovery/recovery-binary-delta.md`.

The script is **optional**. A validator can reach the same result by hand:
stop the node, install the official v2026.1.2 binary, replace
`harmony_db_0` with the frozen clean DB, and start only at GO. The script
automates those steps with checks and a stopped-until-GO hold.

The script lives on the dedicated branch `rollback-92730034`. A branch head
can move, so validator-facing links must **never** point at the branch.
Always use a raw GitHub URL pinned to the reviewed commit SHA
(`https://raw.githubusercontent.com/harmony-one/harmony/<commit-sha>/scripts/rollback-92730034.sh`)
and publish the script file's SHA-256 in a separate channel (chat + email) so
validators can verify what they downloaded. Do not tie the script URL to the
v2026.1.2 tag; the script and the release are published separately.

## 1. The release: v2026.1.2

v2026.1.2 is built and tagged **only after** these two PRs merge. They are
the only code changes in the release:

- [PR #5106](https://github.com/harmony-one/harmony/pull/5106) — rejects the
  abandoned shard-0 child block `92,730,035`
  (`0x5de06979…7c5d`) and the known malicious blocks by exact hash, on every
  path a block can enter.
- [PR #5107](https://github.com/harmony-one/harmony/pull/5107) — activates a
  one-off ViewID floor of `1,000,000,000` for the recovery restart, so the
  restarted chain cannot be out-voted by stale views.

There is **no separate recovery build and no extra patch**. The script
stages the ordinary official `harmony-amd64` / `harmony-arm64` artifacts of
v2026.1.2 and launches them with the validator's own config. The node's
normal config validation applies; nothing is relaxed or forced.

## 2. Freeze checklist

The DB source constants in the `FREEZE CONSTANTS` block of
`scripts/rollback-92730034.sh` are already filled. Fill the remaining
release-dependent constants, commit, review, and record that commit SHA (it
becomes the pinned script URL):

1. `NODE_BIN_URL_AMD64` / `NODE_BIN_SHA256_AMD64` and
   `NODE_BIN_URL_ARM64` / `NODE_BIN_SHA256_ARM64`: the official v2026.1.2
   release artifacts and their SHA-256 values. Verify each file with
   `readelf -h` (64-bit ELF, machine `x86-64` resp. `AArch64`). The script
   selects by `uname -m` and re-checks the ELF machine of what it downloads.
2. `DB_RCLONE_SOURCE` (filled): the clean DB is Harmony's public read-only
   shard-0 SnapDB service, documented at docs.harmony.one under "Shard 0
   validator Snap DB sync" and accepted by the team as the clean shard-0 DB
   for block 92,730,034. The constant is a self-contained rclone WebDAV
   connection string
   (`:webdav,url='http://snapdb.s0.t.hmny.io/webdav',vendor=other,user=snap,pass=…:`),
   so validators need no rclone config file. The team must not refresh the
   SnapDB content between freeze and GO; if it changes anyway, the count and
   byte checks below detect it and the run stops with `source-mismatch`.
3. `DB_FILE_COUNT` / `DB_BYTES` (filled): the measured values of that source
   from `rclone size --json` — **184510 files, 371422947984 bytes**. The
   script refuses to install anything that does not match these numbers
   exactly. The WebDAV service exposes no file hashes, so there is no DB
   SHA-256 to fill; section 6 describes what is verified instead.
4. `SCRIPT_URL`: the pinned raw URL of the script itself (informational, is
   written into run logs).

## 3. The validator message (send with the pinned URLs)

Prerequisite line for the message: install `rclone` first
(`sudo apt-get install rclone` or the distribution's equivalent). `curl` and
standard tools are also required. If one or more commands are missing, the
script lists every missing command and prints a copyable install command for
apt, dnf/yum, or pacman. Install them and run the same recovery command again;
no database change has happened at that point. The node's config must keep
its local RPC (`127.0.0.1:9500`) enabled — the script uses it to identify keys
and to verify health.

**systemd validators (harmony.service) — root required:**

```
curl -fsSL <pinned-script-url> -o rollback-92730034.sh && sudo bash ./rollback-92730034.sh prepare
# later, only after the team sends GO:
sudo bash ./rollback-92730034.sh start
```

**Manual (non-systemd) validators that run harmony as a non-root user, with
binary/config/database owned by that user — run WITHOUT sudo, as that user:**

```
curl -fsSL <pinned-script-url> -o rollback-92730034.sh && bash ./rollback-92730034.sh prepare
# later, only after the team sends GO:
bash ./rollback-92730034.sh start
```

Rootless runs keep every file (database, replacement DB, state, staged
binary) owned by the node user; state lives in
`./.hmy-recovery-92730034/work/` in the invocation directory. If you see
`STOPPED needs-root ...`, your machine has a harmony.service or files owned
by another user: rerun with sudo.

Low-space recovery:

```bash
sudo bash ./rollback-92730034.sh prepare --discard-old-db
```

When the old DB still exists, the script first discovers the validator and
public BLS IDs, confirms the source and replacement binary are available,
and stops Harmony. It then prints the full DB path and requires the operator
to type `y` before deletion. Any other answer cancels and leaves the old DB
intact. After confirmation, it deletes the old DB and downloads and installs
the clean DB while Harmony remains stopped.

`--quiet` skips this confirmation and is only for centrally supervised
automation where the exact deletion path was already reviewed:

```bash
sudo bash ./rollback-92730034.sh prepare --discard-old-db --quiet
```

If the old DB was already deleted, use the same command with
`harmony.service` stopped. The script discovers the paths and flags from
systemd, reports `READY unknown recovery-92730034`, and continues without a
BLS tally identity. This result is suitable for installation testing but must
not count toward restart voting power.

Include verbatim:
- Despite the name, the script installs a clean database ending at block
  92,730,034 and **reverts nothing**; it never restarts your old chain.
- Supported machines: Linux x86_64 and Linux arm64 (e.g. Raspberry Pi 5).
- The database copy is large. It runs while your node is still up, and
  reruns of `prepare` resume the copy file by file. The script shows the
  current step in the terminal; during the copy, rclone reports bytes,
  transfer speed, percentage, and ETA every 10 seconds.
- Progress is written to stderr. The final `READY`, `RUNNING`, or `STOPPED`
  result remains the only line written to stdout.
- The script records and preserves the original Harmony arguments in order.
  This covers common consensus, BLS, P2P, RPC, sync, logging, and Prometheus
  flags. Only the executable path changes to the staged v2026.1.2 binary.
- Run both commands **from the same directory, as the same user**, on a
  persistent filesystem (not /tmp). Manual validators must run them from the
  directory containing the harmony binary or harmony config file.
- **Manual validators: disable any cron/boot/supervisor autostart before
  `prepare`, and leave Harmony stopped until GO.**
- Reply with your exact `READY <bls-ids> recovery-92730034` line.
- If you see `STOPPED <reason> <log-id>`, send both to the team; do not retry
  destructive steps yourself. Reruns of the same command are safe.

## 4. READY tally (manual spreadsheet)

- One row per machine, pasting its full comma-joined READY key set.
- Dedupe by BLS key, then select machines whose complete READY key sets are
  pairwise disjoint (starting a machine activates *all* its keys, so "one
  machine per key" is unsafe with overlapping multi-key configurations).
- Tally the selected union against epoch-3002 shard-0 voting power:
  strictly >2/3 required, aim 75–80%.
- Confirm duplicate signers stopped. Send GO **only to the selected
  machines**; every other READY machine is told to stay stopped.

## 5. STOPPED-reason triage

| Reason | Meaning / action |
| --- | --- |
| `unsupported-platform` | The machine is not Linux x86_64 or Linux aarch64. Handle it manually. |
| `missing-dependencies` | One or more required commands are missing. The lines immediately above `STOPPED` list every missing command and print the package-manager command to install them. Install the packages, then run the same recovery command again. |
| `deletion-cancelled` | The operator did not type `y` at the full-path deletion prompt. The validator remains stopped and the old DB remains intact. Review the path and rerun when ready. |
| `needs-root` | Ran without sudo but a harmony.service is loaded, or the harmony process/files belong to a different user. Rerun with sudo (or as the owning user). |
| `unsupported-layout` | Not packaged-systemd and not a clean manual-directory shape (supervisor, ambiguous processes, non-mainnet/archival/multi-shard config, RPC unreachable, conflicting drop-in, or an unusual argument containing whitespace/control/systemd-special characters). Normal CLI flags and values are preserved. Handle unusual cases one-on-one. |
| `low-disk` | Free space is below one full DB copy plus margin. Free space, or approve `prepare --discard-old-db` (only after a central old-DB archive is confirmed). |
| `source-mismatch` | The remote DB source does not report the pinned file count and byte total (checked before and after the transfer). The SnapDB content changed after freeze or the wrong source is pinned. Node untouched; escalate — do not retry blindly. |
| `download-failed` | rclone could not reach or read the DB source, or a binary download failed, or the downloaded binary is not the right-architecture ELF. Node untouched; rerun after checking connectivity. |
| `checksum-mismatch` | The downloaded harmony binary does not match its pinned SHA-256. Node untouched; check the artifact URL. |
| `db-verify-failed` | The staged or installed DB directory failed the structure check (wrong count/bytes, extra or missing file, symlink or special entry, bad `CURRENT`/`MANIFEST`). The node is not started. Escalate with the log. |
| `stop-failed` | Node or unit would not stop, or could not be proven stopped. Investigate before rerunning. |
| `duplicate-process` | Another process matches the node binary (path or SHA-256), config, DataDir, or open DB files. Find and stop it; never let two signers run. |
| `head-mismatch` | After start, the node reported a wrong hash for block 92,730,034. The node was stopped again and the mismatch is latched in the state file: every later `start` rerun re-proves the node is stopped (stopping it again if anything restarted it), keeps refusing with `head-mismatch`, and never restarts the node. Escalate immediately; only the team, after investigating, may clear the `HEAD_MISMATCH` line from the state file. |
| `not-ready` / `receipt-mismatch` | `start` before READY, or the systemd unit no longer runs the staged command. Re-run `prepare` / inspect drop-ins. |
| `start-failed` / `unhealthy` | Node did not come up, or did not reach the health pins (height, target hash, BLS key set) in time. It was stopped again (systemd stays held). Collect the run log and node.log. |
| `cannot-determine-state` | State file and filesystem disagree, or another invocation was running. The node may be **running, stopped, or in an unknown state** depending on when the failure occurred (for example, it is still running if this happened during the `prepare` transfer or a lock conflict). Check the actual node state first (`systemctl status harmony.service`, `pgrep`), do not delete, move, or modify anything under the DataDir until the situation is understood, and escalate with the run log. |

Run logs: `/var/lib/harmony-recovery-92730034/private/run-<log-id>.log` (root
runs) or `<invocation-dir>/.hmy-recovery-92730034/work/private/run-<log-id>.log`
(rootless runs).

## 6. What the recovery script verifies

The WebDAV source used for DB snapshots reports **no file hashes**, so the
transfer itself has no content checksum. Do not describe the DB copy as
hash-verified. The actual trust controls are:

1. The source is Harmony's team-controlled public SnapDB service, accepted
   by the team as the block-92,730,034 DB and left unchanged between freeze
   and GO.
2. The remote file count and total bytes must equal the pinned values,
   checked before and after the transfer.
3. The received tree must be a plain LevelDB directory: only regular files
   with goleveldb names (`CURRENT`, `LOCK`, `LOG`, `LOG.old`, numeric
   `.ldb`/`.sst`/`.log`), exactly one `MANIFEST-*`, and `CURRENT` naming it.
   No symlinks, no subdirectories, no device or special files.
4. The harmony binary is pinned by SHA-256 and ELF architecture.
5. After start, the node itself must report block 92,730,034 with hash
   `0x30c35d2f…6d8d` over local RPC. If it reports a different hash, the
   script stops the node again, latches the mismatch in its state file, and
   reports `head-mismatch`; later `start` reruns refuse to relaunch until
   the team clears the latch.

A corrupted file of the same size that still produces the same LevelDB shape
would pass checks 2–3 and be caught only by check 5 (or by the node failing
to open the DB). This is a recorded, accepted limitation of the raw-directory
method.

## 7. Canary, post-GO checks, and accepted risks

Canary (before rollout, on both layouts — packaged systemd and
manual-directory; record wall-clock timings):

1. Run `prepare` to `READY`. Verify keys, config, and the original binary are
   untouched.
2. Reboot the machine. Verify the node stayed stopped (systemd: unit inactive
   under the hold; manual: no process). The smoke suite's
   `pre-reboot`/`post-reboot` groups script this check.
3. Rehearse `start` on a disposable copy, never on the real chain before GO.

Post-GO checks:

- Replacement block `92,730,035` has the retained parent, differs from the
  abandoned child `0x5de06979…7c5d`, and the following blocks finalize.
- Spot-check `RUNNING` machines: reported BLS key sets equal the READY tally.

Recorded accepted risks (operator decisions of 2026-08-14):

- **No DB content hash** (see section 6): integrity rests on the
  team-accepted SnapDB source, the count/bytes pins, the structure check,
  and the post-start RPC pin.
- **Manual autostart**: keeping a manual node stopped across reboots is the
  validator's instructed responsibility (the script re-scans on every rerun
  and start); there is no technical hold outside systemd.
- **Rootless duplicate scan is best-effort**: without root, the script cannot
  read other users' `/proc/<pid>/exe` or fd links, so it can match those
  processes only by command line. Acceptable for single-user manual boxes.
- `--discard-old-db` permanently deletes the renamed old DB; gate it
  centrally (only after a central old-DB archive is confirmed) and send it
  one-on-one.
