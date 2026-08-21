# Validator cleanup: return to normal operation

The rollback is complete. You can stop using the rollback script and return
to your normal way of starting Harmony.

The rollback script did not change the Harmony binary at your normal path.
It ran a verified Harmony binary from a temporary folder instead. The script
did download and install the clean rollback database. Depending on the option
you used, it either moved the previous database to a timestamped
`*.pre-recovery-*` folder or deleted it. For shard 1, it also moved the
previous `harmony_db_0` companion aside and Harmony rebuilt it.

Use Harmony v2026.1.3 or a newer
[official release](https://github.com/harmony-one/harmony/releases). Change
one validator service at a time.

Do not delete the active `harmony_db_0` or `harmony_db_1`. Keep your config,
BLS keys, P2P key, and any BLS `.pass` files.

## Systemd validator

1. Replace the example below with the exact service name:

   ```bash
   SERVICE=YOUR_SERVICE.service
   sudo systemctl cat "$SERVICE"
   ```

2. Stop that service. Install the new Harmony binary at the normal path shown
   by the original service configuration.

   ```bash
   sudo systemctl stop "$SERVICE"
   ```

3. Remove only the rollback drop-ins:

   ```bash
   sudo rm -f "/etc/systemd/system/$SERVICE.d/50-harmony-recovery-exec.conf"
   sudo rm -f "/etc/systemd/system/$SERVICE.d/50-harmony-recovery-s1-94978278-exec.conf"
   sudo rm -f "/etc/systemd/system/$SERVICE.d/99-harmony-recovery-hold.conf"
   sudo rm -f "/etc/systemd/system/$SERVICE.d/99-harmony-recovery-s1-94978278-hold.conf"
   sudo systemctl daemon-reload
   ```

4. Check the final command:

   ```bash
   sudo systemctl show "$SERVICE" -p ExecStart
   ```

   It should point to your normal Harmony binary, not a path under
   `/var/lib/harmony-recovery-*`. Confirm that binary is v2026.1.3 or newer.

5. Start and check the service:

   ```bash
   sudo systemctl start "$SERVICE"
   sudo systemctl is-active "$SERVICE"
   ```

## Manual validator

1. Stop the exact Harmony process started by the rollback script.
2. Install Harmony v2026.1.3 or newer at your normal binary path.
3. Start Harmony with your normal command, service manager, or supervisor.

You may use your normal sync settings again.

## Check the validator

Confirm that:

- The running binary is v2026.1.3 or newer.
- The shard and public BLS keys are correct.
- The block height continues to increase.
- No second process is using the same BLS keys.

If a check fails, stop that validator and ask in the validator group.

## Remove rollback files

After the validator starts normally and passes the checks, you may delete:

- The downloaded rollback script.
- `/var/lib/harmony-recovery-92730034`.
- `/var/lib/harmony-recovery-92730034-s1-94978278`.
- `/run/harmony-recovery-92730034`.
- `/run/harmony-recovery-92730034-s1-94978278`.
- Shard 0: `harmony_db_0.pre-recovery-*`.
- Shard 1: `harmony_db_1.pre-recovery-*` and
  `harmony_db_0.pre-s1-recovery-*`.
- `pre-recovery-*` transaction and sync-cache folders.

On a host with several validator services, keep shared rollback files until
every service has returned to normal operation.
