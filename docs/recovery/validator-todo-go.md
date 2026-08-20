# Validator TODO: Stage 2 — GO

This follows [Stage 1 — PREPARE](validator-todo.md). Use this document after
the public Telegram/X announcement says PREPARE has reached quorum and Stage
2 GO has begun. Every validator that completed Stage 1 should follow this
document, whether it finished with `READY`, `READY unknown`, or the
manual procedure.

## Before starting

- Confirm that this validator completed Stage 1 by script or manually.
- Make sure its old/original Harmony process is still stopped before running
  `start`.
- If the host has multiple validator services, run each `start` command
  separately, one at a time. The services may run together after they start.
- Do not run `prepare` or `--discard-old-db` during GO.
- If the script finds a Harmony release newer than v2026.1.3, check the
  official release and normally answer `y` to use the latest version. Ask in
  the validator group chat if anything is unclear.

Download the current script again before `start`, even if PREPARE used an
older copy. Version 3 of the script reuses the completed database and recovery
state from earlier versions, and:

- Upgrades the staged v2026.1.2 binary to v2026.1.3, or to a newer release if
  you accept it.
- Disables stream sync and enables DNS sync for the recovered launch.
- For shard 1, quarantines the old `harmony_db_0` companion so Harmony
  rebuilds it.
- Lets manual validators enter encrypted BLS key passphrases securely during
  startup and saves standard mode-600 passphrase files for detached restarts.
- Adds script/release version checks and safer start/systemd handling.

Keep the same invocation directory and user used for PREPARE, or the same
exact systemd unit.

The script should print this exact banner:

```text
Rollback script version 3 (2026-08-20T04:02:21Z)
```

If the version differs or no version is printed, double-check that the
download actually worked and ask for help if needed.

## Shard 0

Download the current shard-0 script:

```bash
curl -fsSL 'https://raw.githubusercontent.com/harmony-one/harmony/refs/heads/rollback-92730034/scripts/rollback-92730034.sh' -o rollback-92730034.sh
bash ./rollback-92730034.sh --version
```

Manual-directory validator:

Run as the same non-root user and from the same Harmony directory used for
PREPARE:

```bash
bash ./rollback-92730034.sh start
```

Systemd validator:

Use the exact same service name used for PREPARE:

```bash
sudo bash ./rollback-92730034.sh start --systemd-unit YOUR_SHARD0_SERVICE.service
```

Do not run the shard-0 script against a shard-1 service.

## Shard 1

Download the current shard-1 script:

```bash
curl -fsSL 'https://raw.githubusercontent.com/harmony-one/harmony/refs/heads/rollback-92730034/scripts/rollback-92730034-s1-94978278.sh' -o rollback-92730034-s1-94978278.sh
bash ./rollback-92730034-s1-94978278.sh --version
```

Manual-directory validator:

Run as the same non-root user and from the same Harmony directory used for
PREPARE:

```bash
bash ./rollback-92730034-s1-94978278.sh start
```

Systemd validator:

Use the exact same service name used for PREPARE:

```bash
sudo bash ./rollback-92730034-s1-94978278.sh start --systemd-unit YOUR_SHARD1_SERVICE.service
```

At first recovered start, the shard-1 script quarantines the old
`harmony_db_0` epoch companion and requires Harmony to recreate it. Do not
delete, move, or restore either `harmony_db_0` path yourself.

Do not run the shard-1 script against a shard-0 service.

## Send the result

Shard-0 success:

```text
RUNNING <bls-ids> recovery-92730034
```

Shard-1 success:

```text
RUNNING <bls-ids> recovery-92730034-s1-94978278
```

Failure:

```text
STOPPED <reason> <log-id>
```

Send the complete final line to the Harmony team. If the result is
`STOPPED`, do not bypass the script with `systemctl start`, a process
supervisor, or a direct Harmony command. Leave the validator stopped and send
the final line and requested run log.

## Shard-0 systemd validator prepared manually without the script

This section applies if a privileged user completed the manual shard-0
procedure in the PREPARE document.

Before starting, quickly confirm that:

- The service is still stopped.
- `harmony_db_0` still has `184510` files and `371422947984` bytes.
- The service uses the verified v2026.1.3 binary.
- The final config or command uses:

  ```text
  --sync=false --sync.client=false --dns.client=true
  ```

After the public Stage 2 GO announcement, start the service normally:

```bash
sudo systemctl start YOUR_SHARD0_SERVICE.service
sudo systemctl is-active YOUR_SHARD0_SERVICE.service
```

Give Harmony up to three minutes to start. Confirm that RPC becomes healthy,
the block number reaches at least `92730034`, and block `92730034` has this
hash:

```text
0x30c35d2f2291e4b27debe7862956cf7a0cc7abefc044273d6823567335086d8d
```

If the service exits, RPC does not become healthy, or the hash differs, stop
the service and ask in the validator group.

## Shard-1 systemd validator prepared manually without the script

This section applies if a privileged user completed the manual shard-1
procedure in the PREPARE document.

Before starting, quickly confirm that:

- The service is still stopped.
- `harmony_db_1` still has `32482` files and `70073877580` bytes.
- The service uses the verified v2026.1.3 binary.
- The final config or command uses:

  ```text
  --sync=false --sync.client=false --dns.client=true
  ```

Before starting Harmony, move the old shard-0 companion database aside so
Harmony rebuilds it:

```bash
cd '<DataDir>'
sudo mv harmony_db_0 "harmony_db_0.pre-s1-recovery-$(date -u +%Y%m%d-%H%M%S)"
```

If `harmony_db_0` is already missing, do not recreate or restore it. Start the
service normally:

```bash
sudo systemctl start YOUR_SHARD1_SERVICE.service
sudo systemctl is-active YOUR_SHARD1_SERVICE.service
```

Give Harmony up to three minutes to start. Confirm that:

- A new `<DataDir>/harmony_db_0/CURRENT` exists.
- RPC becomes healthy and the block number reaches at least `94978278`.
- Block `94978278` has this hash:

  ```text
  0xa25d77e72c7f71f2b18847c7f6a9bbed8af42244915bd9175cc247d157b11b9f
  ```

- Block `94978279` is absent or does not have the rejected old-chain hash:

  ```text
  0xc936581d391b74a620bf6636519834b14a9a2d4e9a5154867c8407f219d8a878
  ```

If the service exits, `harmony_db_0` is not recreated, RPC does not become
healthy, or either hash check fails, stop the service and ask in the validator
group.

## After RUNNING

- Old bad-chain database folders are not used anymore and can be deleted to
  recover disk space:
  - Shard 0: `harmony_db_0.pre-recovery-*`
  - Shard 1: `harmony_db_1.pre-recovery-*` and
    `harmony_db_0.pre-s1-recovery-*`
- Do not delete the active database folders without a timestamp suffix.
- Keep old or duplicate validator processes stopped.
- Continue monitoring finalization and the validator's normal health signals.
- If the host has more prepared services, finish checking this one before
  starting the next.
- After a `RUNNING` result, do not rerun `start` merely to check status; use
  normal service and RPC monitoring.
