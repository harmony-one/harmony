# Validator TODO: prepare now

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
- If disk space is low or the old database was already deleted, stop and contact the Harmony team.

Required free space:

- Shard 0: about `410 GB` for each selected service.
- Shard 1: about `100 GB` for each selected service.

The selected validator stays online while the database downloads. The script then stops it, installs the replacement database, and leaves it stopped.

## Shard 0

Download the current script:

```bash
curl -fsSL 'https://raw.githubusercontent.com/harmony-one/harmony/refs/heads/rollback-92730034/scripts/rollback-92730034.sh' -o rollback-92730034.sh
```

Keep this downloaded file. Download it again only if the Harmony team asks you to use an updated version.

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
```

Keep this downloaded file. Download it again only if the Harmony team asks you to use an updated version.

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

After sending the result, do nothing else. Keep the validator stopped and wait for GO.
