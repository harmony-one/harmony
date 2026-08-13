#!/usr/bin/env bash
# Full localnet E2E for the produce-verify-seal pipeline (plan WS8/WS9):
# gen fixtures -> inspect (both + agreement) -> export (single donor) ->
# replay -> compact (internal) -> verify -> package -> manual install per
# INSTALL.md -> offline boot smoke with the STOCK harmony binary.
#
# Manual/nightly (make test-recovery). Uses only localnet fixtures with public
# dev keys; creates no cloud resources.
#
#   scripts/recovery/e2e-localnet.sh [workdir]

set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
# shellcheck disable=SC1091
source "${ROOT}/scripts/recovery/common.sh"
WORK="${1:-$(mktemp -d)}"
export NETWORK=localnet SHARD=0 TARGET_HEIGHT=22
BASELINE=18 DONOR=26

echo "== gen fixtures + anchor =="
mkdir -p "${WORK}/reports"
recovery_gorun ./internal/recoverydb/fixture/gen \
  --out "${WORK}/fixtures" --keys "${ROOT}/.hmy" \
  --baseline "${BASELINE}" --target "${TARGET_HEIGHT}" --donor "${DONOR}" \
  --anchor "${WORK}/reports/anchor.json"

export RECOVERY_REPORT_DIR="${WORK}/reports"
export ANCHOR="${WORK}/reports/anchor.json"
DONOR_DB="${WORK}/fixtures/donor/harmony_db_0"
WORKING_DB="${WORK}/fixtures/baseline-a/harmony_db_0"
COPY_B="${WORK}/fixtures/baseline-b/harmony_db_0"
BUNDLE_DIR="${WORK}/bundle"
COMPACT_DB="${WORK}/compact/harmony_db_0"
RELEASE_ROOT="${WORK}/release"

echo "== 10 inspect (both + agreement) =="
COPY_A="${WORKING_DB}" COPY_B="${COPY_B}" ANCHOR="${ANCHOR}" \
  "${ROOT}/scripts/recovery/10-inspect.sh"

echo "== 30 export (single donor) =="
DONOR="${DONOR_DB}" BASELINE_REPORT="${RECOVERY_REPORT_DIR}/inspect-a.json" \
  ANCHOR="${ANCHOR}" BUNDLE_DIR="${BUNDLE_DIR}" \
  FROM_HEIGHT="$((BASELINE + 1))" CERT_CHILD_HEIGHT="$((TARGET_HEIGHT + 1))" \
  DONOR_ID="fixture-donor" \
  "${ROOT}/scripts/recovery/30-export.sh"

echo "== 40 replay =="
WORKING_DB="${WORKING_DB}" ANCHOR="${ANCHOR}" BUNDLE_DIR="${BUNDLE_DIR}" \
  MIN_FREE_BYTES=0 "${ROOT}/scripts/recovery/40-replay.sh"

echo "== 50 compact (internal mode) =="
WORKING_DB="${WORKING_DB}" COMPACT_DB="${COMPACT_DB}" ANCHOR="${ANCHOR}" \
  "${ROOT}/scripts/recovery/50-compact.sh"

echo "== 55 verify =="
COMPACT_DB="${COMPACT_DB}" ANCHOR="${ANCHOR}" \
  "${ROOT}/scripts/recovery/55-verify.sh"

echo "== 60 package (seal) =="
COMPACT_DB="${COMPACT_DB}" ANCHOR="${ANCHOR}" RELEASE_ROOT="${RELEASE_ROOT}" \
  "${ROOT}/scripts/recovery/60-package.sh"

echo "== install per INSTALL.md (verify + rename-aside + copy + post-copy re-verify) =="
FINAL_DIR="$(python3 -c 'import json,sys;print(json.load(open(sys.argv[1]))["release_dir"])' "${RECOVERY_REPORT_DIR}/package.json")"
( cd "${FINAL_DIR}" && sha256sum -c SHA256SUMS >/dev/null )
DATADIR="${WORK}/install"
mkdir -p "${DATADIR}"
[ ! -e "${DATADIR}/harmony_db_0" ]           # destination must not exist
cp -a "${FINAL_DIR}/payload/harmony_db_0" "${DATADIR}/harmony_db_0"
# Post-copy byte re-verification of the installed payload subset.
grep '  payload/harmony_db_0/' "${FINAL_DIR}/SHA256SUMS" \
  | sed "s#  payload/harmony_db_0/#  ${DATADIR}/harmony_db_0/#" \
  | sha256sum -c - >/dev/null
echo "installed + byte-verified at ${DATADIR}/harmony_db_0"

echo "== re-verify the installed DB read-only =="
mkdir -p "${WORK}/reports-installed"
recovery_run inspect-db --network localnet --shard 0 \
  --db "${DATADIR}/harmony_db_0" --read-only \
  --output "${WORK}/reports-installed/inspect.json"

echo "== offline boot smoke with the STOCK harmony binary =="
# Boot the actual stock harmony binary keyless (explorer profile) against
# the installed payload with networking isolated to loopback and every sync
# client disabled, wait past startup, stop it CLEANLY (SIGTERM), reject any
# repair/rewind log line, and re-run the deep verifier on the booted DB
# (round 13 finding 10; round 14 finding 4). The stock config validator
# refuses to start with EVERY sync client off ("either --sync.client or
# --sync.legacy.client shall be enabled"), so the stream-sync downloader
# stays nominally enabled — it only uses the loopback-bound libp2p host.
# NOTE 1: --run.offline is NOT usable: the stock shutdown path hangs in
# host.Close() when the host was never started (p2p/host.go:626-631 closes
# stream protocols that never Start()ed), so SIGTERM never completes.
# NOTE 2: the gRPC sync server must be disabled via the DEPRECATED alias
# --sync.legacy.server=false: the stock flag mapping for --dns.server reads
# the alias's value instead of its own (cmd/config/flags.go:565-566), so
# --dns.server=false alone is silently ignored and *:6000 stays open.
# NOTE 3: Cache.Preimages=true mirrors the mainnet/testnet default
# (localnet alone defaults it OFF, cmd/config/config.go:196) so the stock
# boot cycle writes the COMPLETE preimage marker pair — gen-start on open,
# gen-end on clean Stop — which the post-boot verify-db requires per the
# operator's marker contract (round 16 findings 1-2). It must be set via
# the TOML config: the stock binary defines --cache.preimages but never
# registers the cacheConfigFlags group on the command (cmd/config/flags.go
# has no append of cacheConfigFlags), so the flag is rejected as unknown.
STOCK_BIN="${HARMONY_STOCK_BIN:-}"
if [[ -z "${STOCK_BIN}" ]]; then
  echo "building stock harmony binary (go build ./cmd/harmony)"
  recovery_build_env
  # Stamp the same version ldflags as scripts/go_executable_build.sh: the
  # stock binary os.Exit(1)s at startup if main.commitAt is not a parsable
  # date (cmd/config/version.go), so an unstamped build can never boot.
  E2E_COMMIT="$(cd "${ROOT}" && git rev-parse --short=8 HEAD 2>/dev/null || echo unknown)"
  E2E_COMMITAT="$(cd "${ROOT}" && git log -1 --format=%cd --date=format:'%Y-%m-%dT%H:%M:%S%z' 2>/dev/null || date +%Y-%m-%dT%H:%M:%S%z)"
  E2E_BUILTAT="$(date +%Y-%m-%dT%H:%M:%S%z)"
  ( cd "${ROOT}" && go build \
      -ldflags="-X main.version=vE2E -X main.commit=${E2E_COMMIT} -X main.commitAt=${E2E_COMMITAT} -X main.builtAt=${E2E_BUILTAT} -X main.builtBy=recovery-e2e" \
      -o "${WORK}/harmony" ./cmd/harmony )
  STOCK_BIN="${WORK}/harmony"
fi
TARGET_HASH="$(python3 -c 'import json,sys;print(json.load(open(sys.argv[1]))["target_hash"])' "${ANCHOR}")"
BOOTLOG="${WORK}/stock-boot.log"
BOOTHOME="${WORK}/boot-home"
mkdir -p "${BOOTHOME}"
# lsof is a REQUIRED dependency of this smoke (round 16 finding 3): the
# socket-isolation proof must never silently skip.
command -v lsof >/dev/null 2>&1 || {
  echo "stock boot smoke FAILED — lsof is required for the socket-isolation proof" >&2
  exit 1
}
# The stock explorer profile hardcodes its dashboard to a WILDCARD bind on
# p2p.port-4000 (api/service/explorer/service.go:111, net.JoinHostPort("",
# port)) with no bind-address configuration. Rather than allowlisting that
# externally reachable listener (round 16 finding 3), squat the port first:
# bind it (v4 wildcard + v6only wildcard, bind-only so nothing accepts) so
# the stock ListenAndServe fails with EADDRINUSE — the explorer service
# just logs "[Explorer] Server error." and continues (service.go:145-147),
# and the boot ends up with ZERO wildcard sockets, no exceptions.
EXPLORER_PORT=$(( ${BOOT_P2P_PORT:-39876} - 4000 ))
python3 - "${EXPLORER_PORT}" <<'PYEOF' &
import signal, socket, sys, time
port = int(sys.argv[1])
s4 = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
s4.bind(("0.0.0.0", port))
s6 = socket.socket(socket.AF_INET6, socket.SOCK_STREAM)
s6.setsockopt(socket.IPPROTO_IPV6, socket.IPV6_V6ONLY, 1)
s6.bind(("::", port))
signal.signal(signal.SIGTERM, lambda *_: sys.exit(0))
while True:
    time.sleep(3600)
PYEOF
SQUAT_PID=$!
trap 'kill "${SQUAT_PID}" 2>/dev/null || true' EXIT
sleep 1
kill -0 "${SQUAT_PID}" 2>/dev/null || {
  echo "stock boot smoke FAILED — could not reserve explorer port ${EXPLORER_PORT}" >&2
  exit 1
}
# Dump the stock localnet defaults and flip only Cache.Preimages (NOTE 3).
BOOTCONF="${WORK}/boot-config.toml"
/usr/bin/env DYLD_FALLBACK_LIBRARY_PATH="${LD_LIBRARY_PATH:-}" \
  "${STOCK_BIN}" config dump --network localnet "${BOOTCONF}" >/dev/null
python3 - "${BOOTCONF}" <<'PYEOF'
import sys
path = sys.argv[1]
text = open(path).read()
old, new = "Preimages = false", "Preimages = true"
# Exactly one match expected: the [Cache] entry (RPCOpt uses PreimagesEnabled).
if text.count(old) != 1:
    sys.exit(f"expected exactly one {old!r} in {path}, found {text.count(old)}")
open(path, "w").write(text.replace(old, new))
PYEOF
(
  cd "${BOOTHOME}" && exec /usr/bin/env DYLD_FALLBACK_LIBRARY_PATH="${LD_LIBRARY_PATH:-}" \
    "${STOCK_BIN}" --config "${BOOTCONF}" --network localnet --run explorer --run.shard 0 \
    --datadir "${DATADIR}" --log.console \
    --http=false --ws=false \
    --p2p.ip 127.0.0.1 --p2p.port "${BOOT_P2P_PORT:-39876}" \
    --dns=false --sync.legacy.server=false --sync=true --sync.client=true --prometheus=false
) >"${BOOTLOG}" 2>&1 &
BOOTPID=$!
BOOT_OK=""
for _ in $(seq 1 90); do
  if grep -q "Loaded most recent local full block" "${BOOTLOG}" 2>/dev/null; then
    BOOT_OK=1
    break
  fi
  if ! kill -0 "${BOOTPID}" 2>/dev/null; then
    break
  fi
  sleep 1
done
if [[ -z "${BOOT_OK}" ]]; then
  kill "${BOOTPID}" 2>/dev/null || true
  wait "${BOOTPID}" 2>/dev/null || true
  echo "stock boot smoke FAILED — harmony never loaded the local head; last log lines:" >&2
  tail -n 50 "${BOOTLOG}" >&2
  exit 1
fi
# Let startup complete (services up, any repair/rewind would run now).
sleep "${BOOT_SETTLE_SECONDS:-8}"
# The process must still be ALIVE after settling — a crash after the
# head-load line must not pass as a clean stop (round 15 finding 3).
if ! kill -0 "${BOOTPID}" 2>/dev/null; then
  BOOT_STATUS=0; wait "${BOOTPID}" || BOOT_STATUS=$?
  echo "stock boot smoke FAILED — harmony died during settle (exit status ${BOOT_STATUS}); last log lines:" >&2
  tail -n 50 "${BOOTLOG}" >&2
  exit 1
fi
# Prove network isolation before stopping (round 15 finding 2, hardened in
# round 16 finding 3): every socket of the boot process must be loopback in
# every state — no wildcard listeners, no external endpoints, NO
# exceptions (the explorer dashboard's wildcard bind was defeated above by
# the port squat). Outbound dialing has no targets either: localnet has no
# default bootnodes (GetDefaultBootNodes returns nil) and the DNS sync
# client is off. The explorer bind failure must also be visible in the log,
# proving the squat actually intercepted it rather than racing it.
#
# The inspection itself must fail closed too (round 17 finding 1): lsof
# errors are never suppressed into an empty pass. lsof must exit 0 — its
# exit 1 conflates "no matching files" with real errors, and the booted
# node ALWAYS holds loopback libp2p listeners, so an empty match is itself
# an inspection failure — and the snapshot must contain at least one
# loopback socket, proving lsof genuinely saw the process.
SOCKS_FILE="${WORK}/boot-sockets.txt"
LSOF_ERR_FILE="${WORK}/boot-sockets.err"
LSOF_STATUS=0
lsof -a -p "${BOOTPID}" -i -n -P >"${SOCKS_FILE}" 2>"${LSOF_ERR_FILE}" || LSOF_STATUS=$?
if [[ "${LSOF_STATUS}" -ne 0 ]]; then
  kill -KILL "${BOOTPID}" 2>/dev/null || true
  wait "${BOOTPID}" 2>/dev/null || true
  echo "stock boot smoke FAILED — socket inspection failed (lsof exit ${LSOF_STATUS}); stderr:" >&2
  cat "${LSOF_ERR_FILE}" >&2
  exit 1
fi
if ! tail -n +2 "${SOCKS_FILE}" | grep -Eq '127\.0\.0\.1|\[::1\]'; then
  kill -KILL "${BOOTPID}" 2>/dev/null || true
  wait "${BOOTPID}" 2>/dev/null || true
  echo "stock boot smoke FAILED — socket snapshot shows no loopback socket (inspection did not see the process):" >&2
  cat "${SOCKS_FILE}" >&2
  exit 1
fi
# grep -v exiting 1 here means every socket was loopback — the pass case;
# the inspection itself was already validated above.
NONLOOP="$(tail -n +2 "${SOCKS_FILE}" | grep -Ev '127\.0\.0\.1|\[::1\]' || true)"
if [[ -n "${NONLOOP}" ]]; then
  kill -KILL "${BOOTPID}" 2>/dev/null || true
  wait "${BOOTPID}" 2>/dev/null || true
  echo "stock boot smoke FAILED — non-loopback sockets present during boot:" >&2
  echo "${NONLOOP}" >&2
  exit 1
fi
if ! grep -q "address already in use" "${BOOTLOG}"; then
  kill -KILL "${BOOTPID}" 2>/dev/null || true
  wait "${BOOTPID}" 2>/dev/null || true
  echo "stock boot smoke FAILED — explorer port squat did not intercept the wildcard bind (no EADDRINUSE in boot log)" >&2
  exit 1
fi
# Stop CLEANLY so the leveldb lock is released and shutdown paths run.
kill -TERM "${BOOTPID}"
STOPPED=""
for _ in $(seq 1 60); do
  if ! kill -0 "${BOOTPID}" 2>/dev/null; then
    STOPPED=1
    break
  fi
  sleep 1
done
if [[ -z "${STOPPED}" ]]; then
  kill -KILL "${BOOTPID}" 2>/dev/null || true
  wait "${BOOTPID}" 2>/dev/null || true
  echo "stock boot smoke FAILED — harmony did not stop cleanly on SIGTERM within 60s" >&2
  exit 1
fi
# A genuinely clean exit (round 15 finding 3): the graceful path logs
# "Gracefully shutting down..." + "Successfully shut down!" and exits 0
# (cmd/harmony/main.go listenOSSigAndShutDown -> node.ShutDown -> os.Exit(0));
# anything else — panic, os.Exit(1), signal death — fails the smoke.
BOOT_STATUS=0
wait "${BOOTPID}" || BOOT_STATUS=$?
if [[ "${BOOT_STATUS}" -ne 0 ]]; then
  echo "stock boot smoke FAILED — harmony exited with status ${BOOT_STATUS} (not a graceful shutdown); last log lines:" >&2
  tail -n 50 "${BOOTLOG}" >&2
  exit 1
fi
if ! grep -q "Gracefully shutting down" "${BOOTLOG}" || ! grep -q "Successfully shut down" "${BOOTLOG}"; then
  echo "stock boot smoke FAILED — graceful-shutdown log lines missing; last log lines:" >&2
  tail -n 50 "${BOOTLOG}" >&2
  exit 1
fi
if ! grep "Loaded most recent local full block" "${BOOTLOG}" | grep -qi "${TARGET_HASH}"; then
  echo "stock boot smoke FAILED — booted head is not the pinned target ${TARGET_HASH}:" >&2
  grep "Loaded most recent local" "${BOOTLOG}" >&2 || true
  exit 1
fi
# No repair/rewind/corruption activity is tolerated anywhere in the boot log.
if grep -Ei 'rewind|rewound|repair|truncat|corrupt|bad block|unclean shutdown' "${BOOTLOG}"; then
  echo "stock boot smoke FAILED — repair/rewind activity in the boot log (lines above)" >&2
  exit 1
fi
echo "stock harmony booted the installed DB at the pinned target ${TARGET_HASH} and stopped cleanly"

echo "== deep re-verify of the BOOTED DB (post-boot verify-db) =="
# The boot must not have changed the artifact's logical content: the full
# state + offchain verifier must still pass against the sealed compact.json
# digests (round 14 finding 4).
RECOVERY_REPORT_DIR="${WORK}/reports-installed" recovery_run verify-db \
  --network localnet --shard 0 \
  --db "${DATADIR}/harmony_db_0" --read-only \
  --anchor-manifest "${ANCHOR}" \
  --full-state-check --full-offchain-check \
  --source-reference "${WORK}/reports/compact.json" \
  --output "${WORK}/reports-installed/verification-post-boot.json"
echo "post-boot deep verification passed"

echo "E2E OK — work dir: ${WORK}"
