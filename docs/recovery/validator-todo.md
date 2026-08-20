# Validator TODO: Stage 1 — PREPARE now

## Do this now

Run `prepare` for the selected validator. Send the final `READY` or `STOPPED` line to the Harmony team. Then stop.

**Do not run `start`. Harmony will send GO later.**

Before you begin:

- Confirm whether the selected validator is on shard 0 or shard 1.
- If you use systemd, confirm the exact service name. Do not guess.
- Use a stable SSH session, `tmux`, or `screen`.
- For a manual validator, disable cron, supervisor, or anything else that could restart Harmony.
- Run one recovery command at a time.
- Do not prepare shard 0 and shard 1 in parallel.
- Do not delete a database by hand just to make room. If disk space is low,
  use the `--discard-old-db` option below so the script can stop Harmony and
  safely delete the correct old database for you.
- If the script finds a Harmony release newer than v2026.1.3, check the
  official release and normally answer `y` to use the latest version. Ask in
  the validator group chat if anything is unclear.

Required free space:

- Shard 0: about `410 GB` for each selected service.
- Shard 1: about `100 GB` for each selected service.

The selected validator stays online while the database downloads. The script then stops it, installs the replacement database, and leaves it stopped.

Before invoking `prepare`, use the non-mutating `--version` command shown for
the selected shard and confirm this exact banner:

```text
Rollback script version 3 (2026-08-20T04:02:21Z)
```

If the version differs or no version is printed, double-check that the
download worked and ask for help if needed.

## Shard 0

Download the current script:

```bash
curl -fsSL 'https://raw.githubusercontent.com/harmony-one/harmony/refs/heads/rollback-92730034/scripts/rollback-92730034.sh' -o rollback-92730034.sh
bash ./rollback-92730034.sh --version
```

Keep this downloaded file for `prepare`. If the script says a newer version
is available, download the newer version with the command it prints.

Manual validator:

Run as the same non-root user that currently runs Harmony, from the directory containing the Harmony binary or config:

```bash
bash ./rollback-92730034.sh prepare
```

Systemd validator:

Replace `YOUR_SHARD0_SERVICE.service` with the exact service name:

```bash
sudo bash ./rollback-92730034.sh prepare --systemd-unit YOUR_SHARD0_SERVICE.service
```

Do not run this script against a shard-1 service.

## Shard 1

Download the current script:

```bash
curl -fsSL 'https://raw.githubusercontent.com/harmony-one/harmony/refs/heads/rollback-92730034/scripts/rollback-92730034-s1-94978278.sh' -o rollback-92730034-s1-94978278.sh
bash ./rollback-92730034-s1-94978278.sh --version
```

Keep this downloaded file for `prepare`. If the script says a newer version
is available, download the newer version with the command it prints.

Manual validator:

Run as the same non-root user that currently runs Harmony, from the directory containing the Harmony binary or config:

```bash
bash ./rollback-92730034-s1-94978278.sh prepare
```

Systemd validator:

Replace `YOUR_SHARD1_SERVICE.service` with the exact service name:

```bash
sudo bash ./rollback-92730034-s1-94978278.sh prepare --systemd-unit YOUR_SHARD1_SERVICE.service
```

Do not run this script against a shard-0 service.

## Low-space option: `--discard-old-db`

Use this option when there is not enough room to keep both the old and
replacement databases. You can use it on the first run or after ordinary
`prepare` reports `STOPPED low-disk`.

The script records the validator's public BLS keys while Harmony is still
running, verifies the download source and binary, stops Harmony, prints the
exact old database path, and requires you to type `y` before deleting it. It
then downloads the replacement and can report the normal `READY` line with
the BLS keys. Review the deletion path carefully. Do not add `--quiet` on the
first run.

Shard-0 manual validator:

```bash
bash ./rollback-92730034.sh prepare --discard-old-db
```

Shard-0 systemd validator:

```bash
sudo bash ./rollback-92730034.sh prepare --systemd-unit YOUR_SHARD0_SERVICE.service --discard-old-db
```

Shard-1 manual validator:

```bash
bash ./rollback-92730034-s1-94978278.sh prepare --discard-old-db
```

Shard-1 systemd validator:

```bash
sudo bash ./rollback-92730034-s1-94978278.sh prepare --systemd-unit YOUR_SHARD1_SERVICE.service --discard-old-db
```

The shard-1 command deletes only old `harmony_db_1`; it does not delete the
`harmony_db_0` companion. For either shard, if the target database was
already manually deleted before the script recorded the BLS keys, the result
may be `READY unknown`.

## Send the result

When `prepare` finishes, send the complete final line to the Harmony team.

Shard-0 success:

```text
READY <bls-ids> recovery-92730034
```

Shard-1 success:

```text
READY <bls-ids> recovery-92730034-s1-94978278
```

Failure:

```text
STOPPED <reason> <log-id>
```

If the script reports missing packages, install the packages using the command it prints, then run the same `prepare` command again.

After sending the result, do nothing else. Keep the validator stopped and
wait for GO. When GO is sent, follow
[`validator-todo-go.md`](validator-todo-go.md), not this PREPARE document.

## Systemd operator who will not run the script with sudo

The script needs root for a systemd validator because it must stop the
service, replace the database, and restore ownership. It does not read or
copy private keys.

For a shard-0 validator, a privileged user can prepare it manually:

1. Stop Harmony and prevent any related service or supervisor from restarting
   it. Keep it stopped, including after a reboot.
2. Confirm the exact DataDir, then remove only:

   ```text
   <DataDir>/harmony_db_0
   ```

   Do not touch the config, `.hmy`, BLS keys, P2P key, or logs.
3. Follow Harmony's
   [shard-0 SnapDB guide](https://docs.harmony.one/home/network/validators/node-setup/syncing-db#id-3.-shard-0-validator-snap-db-sync)
   and download the clean snapshot into that exact path. When complete, it
   must contain:

   ```text
   184510 files
   371422947984 bytes
   ```
4. Disable stream sync and enable DNS sync in the config:

   ```toml
   [Sync]
   Enabled = false
   Client = false

   [DNSSync]
   Client = true
   ```

   Command-line and systemd arguments override the config. Remove conflicting
   sync arguments or make sure the final command includes:

   ```text
   --sync=false --sync.client=false --dns.client=true
   ```
5. Install the correct official
   [Harmony v2026.1.3 binary](https://github.com/harmony-one/harmony/releases/tag/v2026.1.3)
   and verify its SHA-256:

   ```text
   amd64: 8a937d29bb678effa7c7a15aa6f6bd75522e452cb0ee037c3a0feb08461ab52b
   arm64: c624d556773347d4ae2b92140714a06a5323e847c468794da1e4b99ed9facf1e
   ```

   Make sure the new database and binary have the ownership and permissions
   needed by the systemd service user.
6. Do not restart Harmony. Tell the validator group that manual preparation
   is complete and wait for the public Stage 2 GO announcement.

For a shard-1 validator, a privileged user can prepare it manually:

1. Stop Harmony and prevent any related service or supervisor from restarting
   it. Keep it stopped, including after a reboot.
2. Confirm the exact DataDir, then remove only:

   ```text
   <DataDir>/harmony_db_1
   ```

   Keep `harmony_db_0` in place for now. Do not touch the config, `.hmy`, BLS
   keys, P2P key, or logs.
3. Follow Harmony's
   [shard-1 FullDB guide](https://docs.harmony.one/home/network/validators/node-setup/syncing-db#id-4.2-shard-1-validator)
   and download the clean database into that exact path. When complete, it
   must contain:

   ```text
   32482 files
   70073877580 bytes
   ```
4. Disable stream sync and enable DNS sync using the same config and command
   settings shown in the shard-0 manual steps above.
5. Install the same verified official v2026.1.3 binary shown above. Make sure
   the new database and binary have the ownership and permissions needed by
   the systemd service user.
6. Do not restart Harmony. Tell the validator group that manual preparation
   is complete and wait for the public Stage 2 GO announcement.
