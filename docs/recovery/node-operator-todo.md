# Node operator rollback guide

This guide is for non-validating mainnet nodes used by RPC services, apps,
bridges, exchanges, and indexers.

Do not use the validator rollback scripts. They require a validator
configuration, a non-archival database layout, and loaded validator BLS keys.
They will reject a normal RPC, explorer, archival, elastic, or keyless node.

## Before you begin

1. Pause traffic or application jobs that use this node.
2. Find the exact Harmony service, shard, and DataDir.
3. Stop Harmony and prevent automatic restart.
4. Install Harmony v2026.1.3 or a newer
   [official release](https://github.com/harmony-one/harmony/releases).
5. Keep the config, P2P key, logs, and other application data.

## Replace the chain database

Delete the discarded-chain database after confirming the exact DataDir and
shard. It contains only abandoned-chain state and should not be reused.

Shard 0:

```bash
cd '<DataDir>' || exit 1
pwd
```

After confirming the path printed by `pwd`:

```bash
sudo rm -rf -- "$PWD/harmony_db_0"
```

Shard 1:

```bash
cd '<DataDir>' || exit 1
pwd
```

After confirming the path printed by `pwd`:

```bash
sudo rm -rf -- "$PWD/harmony_db_1" "$PWD/harmony_db_0"
```

Restore a database from the rollback chain, or start with an empty database
and sync from the rollback network. The
[Harmony database sync guide](https://docs.harmony.one/home/network/validators/node-setup/syncing-db)
lists the SnapDB and FullDB sources.

The shard-0 SnapDB is suitable if the service needs current state and blocks
produced after the rollback. It does not contain earlier block history. Use a
FullDB if the service needs historical block, transaction, receipt, or log
queries. Confirm that any other snapshot contains the rollback checkpoint
hashes listed below.

Archival, explorer, shard-data, Elastic, and TiKV nodes may have more database
or index storage. Rebuild those stores from the rollback chain using your
normal process. Do not reuse indexes containing discarded blocks.

## Start and verify

Start Harmony with your normal service or command. Normal sync settings may
be used.

Confirm that:

- The running binary is v2026.1.3 or newer.
- The node reports the correct shard.
- The block height increases and catches up with the public network.
- These block hashes match:

  ```text
  Shard 0 block 92,730,034:
  0x30c35d2f2291e4b27debe7862956cf7a0cc7abefc044273d6823567335086d8d

  Shard 0 block 92,730,035:
  0x90171a290e303937321db9b084f62438315390bf1d20b22b8800dd5b0a406447

  Shard 1 block 94,978,278:
  0xa25d77e72c7f71f2b18847c7f6a9bbed8af42244915bd9175cc247d157b11b9f

  Shard 1 block 94,978,279:
  0xbf8c0b4d5852e78c2fd815eaa40709c370c4985ef91372f53377992b10afb022
  ```

Stop the node and ask for help if a hash is different.

## Application and indexer data

Blocks after the rollback checkpoints have different hashes and transactions.
Roll back external indexes to:

- Shard 0: block `92,730,034`.
- Shard 1: block `94,978,278`.

Resume indexing at the next block. Compare block hashes, not only block
numbers. Recheck deposits, withdrawals, bridge messages, and application
events after these checkpoints before resuming service.

Any discarded database folders left by an earlier rollback attempt may also
be deleted.
