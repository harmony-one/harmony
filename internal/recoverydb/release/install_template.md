# Installing the recovered shard-0 database (manual note)

Release `{{.ReleaseID}}` — network `{{.Network}}`, shard {{.ShardID}}, profile `validator`.

This directory contains a clean, deeply verified `harmony_db_0` at the pinned
recovery target:

- target height: **{{.TargetHeight}}**
- target hash: **{{.TargetHash}}**
- target parent hash: `{{.TargetParentHash}}`
- target epoch: {{.TargetEpoch}}

Follow the four steps in order. If any check fails, STOP and report to the
recovery team — do not improvise.

## (a) Verify the download

From this release directory:

```sh
sha256sum -c SHA256SUMS
cat READY   # must print exactly: {{.ReleaseID}}
```

(`rclone check --checksum <remote> .` is an equivalent remote-side check.)
Any failure means a partial or corrupted download — stop and re-download.
`SHA256SUMS` covers every file in this tree except itself and `READY`.

## (b) Install

1. Stop the node service and confirm the process is actually gone — never
   touch `harmony_db_0` while anything may hold it:

```sh
sudo systemctl stop harmony
pgrep -x harmony && echo "STILL RUNNING - do not proceed" || echo "stopped"
```

2. Rename the entire old database aside — **never merge or copy over it**.
   The destination path must not exist before the copy (this also avoids the
   `cp` nested-directory trap of ending up with `harmony_db_0/harmony_db_0`):

```sh
mv "<DataDir>/harmony_db_0" "<DataDir>/harmony_db_0.pre-recovery.$(date +%s)"
test ! -e "<DataDir>/harmony_db_0"   # must hold before the next step
cp -a payload/harmony_db_0 "<DataDir>/harmony_db_0"
```

## (c) Confirm the installed bytes

Re-run the payload subset of `SHA256SUMS` against the installed path. This
byte-verifies that the installed DB **is** the artifact `verify-db` proved to
be at the pinned target on the producer — the supported head confirmation on
any hardware:

```sh
grep '^.*  payload/harmony_db_0/' SHA256SUMS \
  | sed 's#  payload/harmony_db_0/#  <DataDir>/harmony_db_0/#' \
  | sha256sum -c -
```

`release.json` prints what that proven head is (target height/hash above).
Honest limits: reading `release.json` alone checks metadata; it is this
post-copy byte-verification that ties the installed DB to the proof. An
actual live DB head read requires a compatible linux-amd64
`harmony-recovery-db` binary and is optional:

> **Digest contract (operator-decided deviation from plan §11.4).** The
> logical KV digest sealed into this release excludes exactly three keys:
> the recovery marker plus the stock node's two preimage bookkeeping keys
> (`preimage-gen-start`/`preimage-gen-end`), which a stock node rewrites on
> every preimage-enabled open and clean stop. The original plan wording
> defined only the recovery marker as excluded; the recovery operator
> explicitly widened the exclusion set so a booted-then-cleanly-stopped DB
> still verifies digest-identical. The preimage pair is not unchecked:
> `verify-db` requires it to be present as a complete pair or not at all,
> with exact values pinned to the recovery target.

```sh
# conditional on having the binary; safe: opens strictly read-only
harmony-recovery-db inspect-db --network {{.Network}} --shard {{.ShardID}} \
  --db "<DataDir>/harmony_db_0" --read-only \
  --output "<DataDir>/recovery-inspect.json"
```

## (d) Wait for the coordinated restart — what NOT to do

Leave the node **stopped**. The coordinated restart is manual and
team-verified; the restart binary/configuration (stock or
`v2026.1.2-recovery`) is announced by the team at restart time.

**Do not restart into stock public sync on your own.** Before the network is
relaunched on the recovered chain, a self-started stock node can pull the
abandoned branch or otherwise sync past the recovered target, invalidating
this clean install — the same reason stock sync was rejected as a recovery
mechanism. A stock restart is safe only once the team announces the network
is live on the recovered chain and instructs it.
