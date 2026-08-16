#!/usr/bin/env bash
# Smoke test for scripts/rollback-92730034.sh. Run as root on a DISPOSABLE
# Linux box (VM or container) - it creates users, writes /var/lib and
# /etc/systemd, and kills processes. Never run on a real validator.
#
#   sudo bash scripts/rollback-92730034-smoke.sh [manual|systemd|pre-reboot|post-reboot|all]
#
#   manual      manual-directory layout cases (any Linux root environment)
#   systemd     systemd layout cases (requires systemd as PID 1)
#   pre-reboot  prepare one fixture to READY (systemd layout if PID 1 is
#               systemd, else manual), then exit; reboot the box, then run
#   post-reboot verify still-stopped across the reboot and start -> RUNNING
#   all         manual + systemd (skips systemd cases when unavailable)
#
# Script mechanics are exercised with a test-profile constants block (sed-ed
# into a disposable copy), a fake harmony ELF, a loopback RPC stub, a local
# HTTP fixture server (binaries only), and a raw LevelDB-shaped clean-DB
# fixture directory transferred through rclone's local backend. The real
# binary/DB pairing is exercised only by the central smoke test and the
# canaries, never here.
#
# shellcheck disable=SC2015,SC2012,SC2016
# (SC2015: `cond && ok || bad` is the intended assert pattern - ok never fails.
#  SC2012: ls is used only to snapshot a directory listing. SC2016: literal $
#  inside single-quoted bash -c strings is intended.)
set -euo pipefail

REPO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SRC="$REPO_DIR/scripts/rollback-92730034.sh"
T=/var/tmp/rbsmoke            # fixture root (must survive a reboot; /tmp is tmpfs on many distros)
RB="$T/rb-test.sh"            # disposable test copy of the installer
WWW="$T/www"
WORK=/var/lib/harmony-recovery-92730034
BIN="$WORK/bin/harmony-recovery"
PRIV="$WORK/private"
STATE="$PRIV/state"
SENT_DIR=/run/harmony-recovery-92730034
DIE="$T/fake-harmony-die"
HTTP=http://127.0.0.1:8642
TARGET_HASH="0x30c35d2f2291e4b27debe7862956cf7a0cc7abefc044273d6823567335086d8d"
K1="$(printf 'ab%.0s' $(seq 48))"   # 96 hex chars
K2="$(printf 'cd%.0s' $(seq 48))"
K3="$(printf 'ef%.0s' $(seq 48))"
BLS_SORTED="$K1,$K2"
UNIT_A="rbsmoke-validator-a.service"
UNIT_B="rbsmoke-validator-b.service"
# Force manual-layout detection even where a harmony.service unit exists.
ABSENT_UNIT="rbsmoke-absent.service"

PASS=0; FAIL=0; FAILED=""
ok()   { PASS=$((PASS+1)); echo "PASS  $1"; }
bad()  { FAIL=$((FAIL+1)); FAILED+=" $1"; echo "FAIL  $1: ${2-}"; }
skip() { echo "SKIP  $1: ${2-}"; }

have_systemd() { [[ -d /run/systemd/system ]] && [[ "$(ps -p 1 -o comm= 2>/dev/null)" == "systemd" ]]; }

# ---------- generic helpers ----------

RB_OUT=""; RB_RC=0
run_rb() { # <cwd> <args...>; captures single stdout line + rc
  local dir="$1"; shift
  set +e
  RB_OUT="$(cd "$dir" && SERVICE="${RB_SERVICE-}" bash "$RB" "$@")"
  RB_RC=$?
  set -e
}

run_rb_user() { # <user> <cwd> <args...>; rootless invocation as <user>
  local u="$1" dir="$2"; shift 2
  set +e
  RB_OUT="$(cd "$dir" && runuser -u "$u" -- env SERVICE="${RB_SERVICE-}" bash "$RB" "$@")"
  RB_RC=$?
  set -e
}

host_arch_token() { # AMD64 or ARM64 constant-name suffix for this host
  case "$(uname -m)" in
    x86_64) echo AMD64 ;;
    aarch64) echo ARM64 ;;
    *) echo "unsupported test arch: $(uname -m)" >&2; exit 1 ;;
  esac
}

expect() { # <case> <want-rc> <line-regex>
  local name="$1" rc="$2" re="$3" nlines
  nlines="$(printf '%s\n' "$RB_OUT" | wc -l | tr -d ' ')"
  if [[ "$RB_RC" != "$rc" ]]; then bad "$name" "rc=$RB_RC want $rc (out: $RB_OUT)"; return 1; fi
  if [[ "$nlines" != "1" ]]; then bad "$name" "stdout not exactly one line: '$RB_OUT'"; return 1; fi
  if ! [[ "$RB_OUT" =~ $re ]]; then bad "$name" "line '$RB_OUT' !~ $re"; return 1; fi
  ok "$name"
}

state_get() { sed -n "s/^$1=//p" "$STATE" 2>/dev/null || true; }
unit_state_get() { sed -n "s/^$2=//p" "$WORK/units/$1/private/state" 2>/dev/null || true; }

pids_by_exe() { # <exe path> -> pids on stdout
  local d out=""
  for d in /proc/[0-9]*; do
    [[ "$(readlink "$d/exe" 2>/dev/null)" == "$1" ]] && out+="${d#/proc/} "
  done
  echo "$out"
}

wait_for() { # <secs> <cmd...>: poll until cmd true
  local deadline=$(( SECONDS + $1 )); shift
  while (( SECONDS < deadline )); do "$@" && return 0; sleep 1; done
  return 1
}

kill_fakes() {
  local d exe
  for d in /proc/[0-9]*; do
    exe="$(readlink "$d/exe" 2>/dev/null || true)"
    if [[ "$exe" == "$T/cases/"* || "$exe" == "$BIN" \
       || "$exe" == "$WORK/units/"*"/bin/harmony-recovery" \
       || "$exe" == "/usr/sbin/harmony" || "$exe" == "/usr/sbin/harmony-evil" \
       || "$exe" == "$T/imposter/"* ]]; then
      kill -9 "${d#/proc/}" 2>/dev/null || true
    fi
  done
}

rpc_set() { # healthy defaults unless overridden: rpc_set [bn] [hash] [keys-json]
  local bn="${1-92730100}" h="${2-$TARGET_HASH}" keys="${3-[\"$K2\",\"$K1\"]}"
  printf '{"bn": %s, "blockhash": "%s", "blskey": %s}\n' "$bn" "$h" "$keys" > "$T/rpc.json"
}

rpc_set_port() { # <port> [keys-json]
  printf '{"bn": 92730100, "blockhash": "%s", "blskey": %s}\n' \
    "$TARGET_HASH" "${2-[\"$K3\"]}" > "$T/rpc-$1.json"
}

cleanup_case() {
  kill_fakes
  if have_systemd; then
    systemctl stop harmony.service >/dev/null 2>&1 || true
    systemctl stop "$UNIT_A" "$UNIT_B" >/dev/null 2>&1 || true
    rm -rf /etc/systemd/system/harmony.service.d
    rm -rf "/etc/systemd/system/$UNIT_A.d" "/etc/systemd/system/$UNIT_B.d"
    rm -f "/etc/systemd/system/$UNIT_A" "/etc/systemd/system/$UNIT_B"
    rm -f /usr/sbin/harmony-evil
    systemctl daemon-reload || true
  fi
  rm -rf /home/rbsmoke-a /home/rbsmoke-b
  rm -f /etc/harmony/rbsmoke-a.conf /etc/harmony/rbsmoke-b.conf
  rm -rf "$WORK" "$SENT_DIR"
  rm -f "$DIE"
  rm -f "$T"/rpc-*.json
  rpc_set
  rpc_set_port 9600
}

# ---------- fixtures ----------

build_fixtures() {
  mkdir -p "$T" "$WWW" "$T/cases" "$T/imposter"
  id hmytest >/dev/null 2>&1 || useradd -m hmytest
  id harmony >/dev/null 2>&1 || useradd -m harmony

  if [[ ! -x "$T/fake-orig" ]]; then
    cat > "$T/fake.c" <<'EOF'
#include <stdio.h>
#include <string.h>
#include <unistd.h>
static const char *variant = VARIANT;
int main(int argc, char **argv) {
  /* Keep VARIANT live so -O2 cannot fold orig/recovery into identical bytes:
     duplicate-scan tests rely on the two binaries having different SHA-256. */
  if (argc > 1 && strcmp(argv[1], "variant") == 0) { printf("%s\n", variant); return 0; }
  if (access("/var/tmp/rbsmoke/fake-harmony-die", F_OK) == 0) { fprintf(stderr, "dying by request\n"); return 1; }
  for (;;) pause();
}
EOF
    cc -O2 -DVARIANT='"orig"'  -o "$T/fake-orig"     "$T/fake.c"
    cc -O2 -DVARIANT='"recov"' -o "$T/fake-recovery" "$T/fake.c"
  fi
  NODE_SHA="$(sha256sum "$T/fake-recovery" | awk '{print $1}')"
  ORIG_SHA="$(sha256sum "$T/fake-orig" | awk '{print $1}')"
  [[ "$NODE_SHA" != "$ORIG_SHA" ]] \
    || { echo "fixture binaries are byte-identical; duplicate-scan coverage would be meaningless"; exit 1; }
  cp -f "$T/fake-recovery" "$WWW/node.bin"

  # A valid-looking 64-bit LE ELF header with the WRONG e_machine for this
  # host: pins the other-arch URL, and drives the wrong-arch rejection case.
  python3 - "$T" <<'EOF'
import os, sys
t = sys.argv[1]
machine = 0xB7 if os.uname().machine == 'x86_64' else 0x3E
hdr = bytearray(64)
hdr[0:6] = b'\x7fELF\x02\x01'
hdr[18] = machine
with open(t + '/wrongarch.bin', 'wb') as f:
    f.write(bytes(hdr) + b'RBSMOKE-WRONG-ARCH' * 8)
EOF
  cp -f "$T/wrongarch.bin" "$WWW/node-other.bin"
  OTHER_SHA="$(sha256sum "$T/wrongarch.bin" | awk '{print $1}')"

  # Raw LevelDB-shaped clean-DB fixture directories, served through rclone's
  # local backend (the production source is a frozen read-only WebDAV path).
  if [[ ! -d "$T/dbsrc" ]]; then
    mkdir -p "$T/dbsrc"
    printf 'MANIFEST-000001\n' > "$T/dbsrc/CURRENT"
    cp "$T/dbsrc/CURRENT" "$T/dbsrc/CURRENT.bak"
    head -c 512 /dev/urandom > "$T/dbsrc/MANIFEST-000001"
    head -c 4096 /dev/urandom > "$T/dbsrc/000001.ldb"
    head -c 2048 /dev/urandom > "$T/dbsrc/000002.log"
    : > "$T/dbsrc/LOCK"
    printf 'log\n' > "$T/dbsrc/LOG"
    # Malformed source: CURRENT names a MANIFEST that does not exist. Its
    # metrics are pinned honestly, so only the structure check can reject it.
    mkdir -p "$T/dbsrc-bad"
    printf 'MANIFEST-000009\n' > "$T/dbsrc-bad/CURRENT"
    head -c 4096 /dev/urandom > "$T/dbsrc-bad/000001.ldb"
    # Same count and bytes as the good tree, but one table renamed to a
    # non-goleveldb filename: only the filename-class check can reject it.
    cp -a "$T/dbsrc" "$T/dbsrc-badname"
    mv "$T/dbsrc-badname/000001.ldb" "$T/dbsrc-badname/evil-000001.ldb"
  fi

  cat > "$T/stub.py" <<'EOF'
import json, os, re, threading
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

ROOT = '/var/tmp/rbsmoke'

def cfg(port):
    path = ROOT + ('/rpc.json' if port == 9500 else '/rpc-%d.json' % port)
    with open(path) as f:
        return json.load(f)

class Rpc(BaseHTTPRequestHandler):
    def log_message(self, *a): pass
    def do_POST(self):
        body = json.loads(self.rfile.read(int(self.headers.get('Content-Length', 0)) or 0))
        c = cfg(self.server.server_port)
        m = body.get('method', '')
        if m == 'hmyv2_getNodeMetadata':
            result = {'blskey': c['blskey']}
        elif m == 'hmyv2_blockNumber':
            result = c['bn']
        elif m == 'hmyv2_getBlockByNumber':
            result = {'hash': c['blockhash']}
        else:
            result = None
        out = json.dumps({'jsonrpc': '2.0', 'id': body.get('id', 1), 'result': result}).encode()
        self.send_response(200)
        self.send_header('Content-Type', 'application/json')
        self.send_header('Content-Length', str(len(out)))
        self.end_headers()
        self.wfile.write(out)

class Files(BaseHTTPRequestHandler):
    def log_message(self, *a): pass
    def do_GET(self):
        path = os.path.join(ROOT, 'www', os.path.basename(self.path))
        if not os.path.isfile(path):
            self.send_error(404); return
        size = os.path.getsize(path)
        start = 0
        rng = self.headers.get('Range')
        status = 200
        if rng:
            m = re.match(r'bytes=(\d+)-$', rng)
            if m:
                start = int(m.group(1)); status = 206
        with open(ROOT + '/http.log', 'a') as lg:
            lg.write('GET %s status=%d start=%d\n' % (os.path.basename(path), status, start))
        self.send_response(status)
        self.send_header('Content-Type', 'application/octet-stream')
        self.send_header('Content-Length', str(size - start))
        if status == 206:
            self.send_header('Content-Range', 'bytes %d-%d/%d' % (start, size - 1, size))
        self.end_headers()
        with open(path, 'rb') as f:
            f.seek(start)
            self.wfile.write(f.read())

for port in (9500, 9600):
    threading.Thread(target=lambda p=port: ThreadingHTTPServer(('127.0.0.1', p), Rpc).serve_forever(), daemon=True).start()
ThreadingHTTPServer(('127.0.0.1', 8642), Files).serve_forever()
EOF
  pkill -f 'rbsmoke/stub.py' 2>/dev/null || true
  sleep 0.5
  rpc_set
  rpc_set_port 9600
  : > "$T/http.log"
  setsid python3 "$T/stub.py" >/dev/null 2>&1 < /dev/null &
  wait_for 10 curl -sf -o /dev/null "$HTTP/node.bin" || { echo "fixture HTTP server failed"; exit 1; }
}

db_src_count() { find "$1" -mindepth 1 -type f | wc -l | tr -d ' '; }
db_src_bytes() { find "$1" -mindepth 1 -type f -printf '%s\n' | awk '{s+=$1} END{print s+0}'; }

mk_script() { # [db-source-dir] [margin-min-bytes]; builds $RB with test constants
  local src="${1-$T/dbsrc}" margin="${2-1048576}" cnt bytes harch oarch
  cnt="$(db_src_count "$src")"
  bytes="$(db_src_bytes "$src")"
  harch="$(host_arch_token)"
  if [[ "$harch" == AMD64 ]]; then oarch=ARM64; else oarch=AMD64; fi
  cp -f "$SRC" "$RB"
  chmod 644 "$RB"
  # Host arch gets the real fixture binary; the other arch gets a distinct
  # URL/sha so a wrong arch selection fails loudly (mismatched sha).
  sed -i \
    -e "s|^DB_RCLONE_SOURCE=.*|DB_RCLONE_SOURCE=\"$src\"|" \
    -e "s|^DB_FILE_COUNT=.*|DB_FILE_COUNT=$cnt|" \
    -e "s|^DB_BYTES=.*|DB_BYTES=$bytes|" \
    -e "s|^NODE_BIN_URL_$harch=.*|NODE_BIN_URL_$harch=\"$HTTP/node.bin\"|" \
    -e "s|^NODE_BIN_SHA256_$harch=.*|NODE_BIN_SHA256_$harch=\"$NODE_SHA\"|" \
    -e "s|^NODE_BIN_URL_$oarch=.*|NODE_BIN_URL_$oarch=\"$HTTP/node-other.bin\"|" \
    -e "s|^NODE_BIN_SHA256_$oarch=.*|NODE_BIN_SHA256_$oarch=\"$OTHER_SHA\"|" \
    -e "s|^START_ACTIVE_TIMEOUT=.*|START_ACTIVE_TIMEOUT=10|" \
    -e "s|^START_RPC_TIMEOUT=.*|START_RPC_TIMEOUT=25|" \
    -e "s|^STOP_TIMEOUT=.*|STOP_TIMEOUT=30|" \
    -e "s|^OBSERVE_SECS=.*|OBSERVE_SECS=3|" \
    -e "s|^MARGIN_MIN_BYTES=.*|MARGIN_MIN_BYTES=$margin|" \
    -e "s|^MARGIN_MIN_DISCARD_BYTES=.*|MARGIN_MIN_DISCARD_BYTES=524288|" \
    "$RB"
}

# ---------- manual-directory fixture ----------

new_manual_case() { # <name>; sets INV, DATA
  INV="$T/cases/$1"
  DATA="$INV/data"
  rm -rf "$INV"
  mkdir -p "$DATA/harmony_db_0"
  printf 'x' > "$DATA/harmony_db_0/CURRENT"
  printf 'old' > "$DATA/harmony_db_0/olddata"
  cp "$T/fake-orig" "$INV/harmony"
  cat > "$INV/harmony.conf" <<EOF
Version = "2.6.2"

[General]
NodeType = "validator"
IsArchival = false
DataDir = "./data"

[Network]
NetworkType = "mainnet"

[ShardData]
EnableShardData = false
EOF
  chown -R hmytest: "$INV"
}

start_orig() { # [extra harmony args...]
  ( cd "$INV" && exec runuser -u hmytest -- setsid ./harmony -c ./harmony.conf "$@" ) >/dev/null 2>&1 < /dev/null &
  wait_for 10 bash -c "[[ -n \"\$(readlink /proc/*/exe 2>/dev/null | grep -Fx '$INV/harmony')\" ]]" \
    || { echo "fixture node failed to start"; exit 1; }
}

orig_running() { [[ -n "$(pids_by_exe "$INV/harmony")" ]]; }

# ---------- manual cases ----------

m_happy() {
  cleanup_case; new_manual_case happy; mk_script; start_orig
  local usrsbin_before; usrsbin_before="$(ls /usr/local/sbin 2>/dev/null | sha256sum)"
  local orig_sha_before; orig_sha_before="$(sha256sum "$INV/harmony" | awk '{print $1}')"

  RB_SERVICE="$ABSENT_UNIT" run_rb "$INV" prepare
  expect "m_happy/prepare" 0 "^READY [0-9a-f]{96}(,[0-9a-f]{96})* recovery-92730034$" || return 0
  grep -q 'rclone concurrency: transfers=' "$PRIV"/run-*.log \
    && ok "m_happy/rclone-concurrency-selected" || bad "m_happy/rclone-concurrency-selected"
  [[ "$RB_OUT" == "READY $BLS_SORTED recovery-92730034" ]] \
    && ok "m_happy/bls-sorted" || bad "m_happy/bls-sorted" "$RB_OUT"
  grep -q 'GET node.bin' "$T/http.log" && ! grep -q 'GET node-other.bin' "$T/http.log" \
    && ok "m_happy/arch-artifact-selected" || bad "m_happy/arch-artifact-selected" "$(grep -c node "$T/http.log" || true)"
  [[ "$(sha256sum "$BIN" | awk '{print $1}')" == "$NODE_SHA" ]] \
    && ok "m_happy/staged-bin-sha" || bad "m_happy/staged-bin-sha"
  ! orig_running && ok "m_happy/node-stopped" || bad "m_happy/node-stopped" "still running"
  sleep 2
  ! orig_running && ok "m_happy/stays-stopped" || bad "m_happy/stays-stopped" "reappeared"
  local old; old="$(state_get OLD_DB_NAME)"
  [[ -n "$old" && -d "$DATA/$old" && -f "$DATA/$old/olddata" ]] \
    && ok "m_happy/old-renamed" || bad "m_happy/old-renamed" "old=$old"
  diff -r "$T/dbsrc" "$DATA/harmony_db_0" >/dev/null 2>&1 \
    && ok "m_happy/new-installed" || bad "m_happy/new-installed" "installed tree differs from source"
  [[ "$(stat -c %U "$DATA/harmony_db_0")" == "hmytest" ]] \
    && ok "m_happy/new-owned" || bad "m_happy/new-owned" "$(stat -c %U "$DATA/harmony_db_0")"
  [[ "$(sha256sum "$INV/harmony" | awk '{print $1}')" == "$orig_sha_before" && -f "$INV/harmony" ]] \
    && ok "m_happy/orig-binary-intact" || bad "m_happy/orig-binary-intact"
  [[ "$(ls /usr/local/sbin 2>/dev/null | sha256sum)" == "$usrsbin_before" ]] \
    && ok "m_happy/usr-local-sbin-untouched" || bad "m_happy/usr-local-sbin-untouched"
  [[ "$(stat -c %a:%U "$STATE")" == "600:root" ]] \
    && ok "m_happy/state-root-0600" || bad "m_happy/state-root-0600" "$(stat -c %a:%U "$STATE")"
  ! runuser -u hmytest -- cat "$STATE" >/dev/null 2>&1 \
    && ok "m_happy/state-unreadable-nonroot" || bad "m_happy/state-unreadable-nonroot"
  [[ "$(state_get STATE)" == "READY" ]] && ok "m_happy/state-ready" || bad "m_happy/state-ready" "$(state_get STATE)"
  [[ "$(state_get OLD_DB_DISPOSITION)" == "kept" ]] && ok "m_happy/kept" || bad "m_happy/kept"

  RB_SERVICE="$ABSENT_UNIT" run_rb "$INV" prepare
  expect "m_happy/prepare-rerun-idempotent" 0 "^READY $BLS_SORTED recovery-92730034$"

  RB_SERVICE="$ABSENT_UNIT" run_rb "$INV" start
  expect "m_happy/start" 0 "^RUNNING $BLS_SORTED recovery-92730034$" || return 0
  local np; np="$(state_get NODE_PID)"
  [[ -n "$np" && -d "/proc/$np" ]] || { bad "m_happy/node-pid" "np=$np"; return 0; }
  [[ "$(readlink "/proc/$np/exe")" == "$BIN" ]] && ok "m_happy/pid-exe-staged" || bad "m_happy/pid-exe-staged"
  [[ "$(stat -c %U "/proc/$np")" == "hmytest" ]] && ok "m_happy/runs-as-user" || bad "m_happy/runs-as-user"
  [[ "$(readlink "/proc/$np/cwd")" == "$INV" ]] && ok "m_happy/runs-from-cwd" || bad "m_happy/runs-from-cwd"
  [[ "$(state_get STATE)" == "STARTED" ]] && ok "m_happy/state-started" || bad "m_happy/state-started"

  RB_SERVICE="$ABSENT_UNIT" run_rb "$INV" start
  expect "m_happy/start-rerun" 0 "^RUNNING $BLS_SORTED recovery-92730034$"
}

m_source_mismatch() {
  # Remote metrics differ from the pins: refused before the node is touched
  # and before any transfer. Correct pins then recover to READY.
  cleanup_case; new_manual_case srcmetric; mk_script; start_orig
  sed -i "s|^DB_FILE_COUNT=.*|DB_FILE_COUNT=$(( $(db_src_count "$T/dbsrc") + 1 ))|" "$RB"
  RB_SERVICE="$ABSENT_UNIT" run_rb "$INV" prepare
  expect "m_srcmetric/source-mismatch" 1 "^STOPPED source-mismatch [0-9-]+$"
  orig_running && ok "m_srcmetric/node-untouched" || bad "m_srcmetric/node-untouched"
  [[ -z "$(ls -A "$DATA/.hmy-recovery-92730034/db" 2>/dev/null)" ]] \
    && ok "m_srcmetric/nothing-transferred" || bad "m_srcmetric/nothing-transferred"
  mk_script
  RB_SERVICE="$ABSENT_UNIT" run_rb "$INV" prepare
  expect "m_srcmetric/rerun-ready" 0 "^READY $BLS_SORTED recovery-92730034$"
}

m_resume_partial() {
  # A partially staged tree is completed per file on rerun: the pre-seeded
  # complete file is skipped (same inode), missing files are transferred.
  cleanup_case; new_manual_case resume; mk_script; start_orig
  local dst="$DATA/.hmy-recovery-92730034/db/harmony_db_0" ino
  mkdir -p "$dst"
  cp -p "$T/dbsrc/000001.ldb" "$dst/000001.ldb"
  chown -R hmytest: "$DATA/.hmy-recovery-92730034"
  ino="$(stat -c %i "$dst/000001.ldb")"
  RB_SERVICE="$ABSENT_UNIT" run_rb "$INV" prepare
  expect "m_resume/ready" 0 "^READY $BLS_SORTED recovery-92730034$" || return 0
  [[ "$(stat -c %i "$DATA/harmony_db_0/000001.ldb")" == "$ino" ]] \
    && ok "m_resume/seeded-file-not-recopied" || bad "m_resume/seeded-file-not-recopied"
  diff -r "$T/dbsrc" "$DATA/harmony_db_0" >/dev/null 2>&1 \
    && ok "m_resume/installed-matches-source" || bad "m_resume/installed-matches-source"
}

m_bad_source() {
  # Structurally bad source (CURRENT names a missing MANIFEST) whose metrics
  # match its own pins: rejected by the staged-tree check, node never stopped.
  cleanup_case; new_manual_case badsrc; start_orig
  mk_script "$T/dbsrc-bad"
  RB_SERVICE="$ABSENT_UNIT" run_rb "$INV" prepare
  expect "m_badsrc/db-verify-failed" 1 "^STOPPED db-verify-failed [0-9-]+$"
  orig_running && ok "m_badsrc/node-untouched" || bad "m_badsrc/node-untouched"
  [[ -d "$DATA/harmony_db_0" && -f "$DATA/harmony_db_0/olddata" ]] \
    && ok "m_badsrc/live-db-untouched" || bad "m_badsrc/live-db-untouched"
}

m_tampered_installed() {
  # The installed DB is re-verified before every first launch: extra files and
  # symlink/special entries are rejected with the node kept stopped, and the
  # untampered tree then starts normally.
  cleanup_case; new_manual_case tamperdb; mk_script; start_orig
  RB_SERVICE="$ABSENT_UNIT" run_rb "$INV" prepare
  expect "m_tamperdb/ready" 0 "^READY " || return 0
  printf 'x' > "$DATA/harmony_db_0/extra-file"
  RB_SERVICE="$ABSENT_UNIT" run_rb "$INV" start
  expect "m_tamperdb/extra-file-refused" 1 "^STOPPED db-verify-failed [0-9-]+$"
  [[ -z "$(pids_by_exe "$BIN")" ]] && ok "m_tamperdb/not-launched" || bad "m_tamperdb/not-launched"
  rm -f "$DATA/harmony_db_0/extra-file"
  mv "$DATA/harmony_db_0/LOG" "$T/saved-LOG"
  ln -s /etc/passwd "$DATA/harmony_db_0/LOG"
  RB_SERVICE="$ABSENT_UNIT" run_rb "$INV" start
  expect "m_tamperdb/symlink-refused" 1 "^STOPPED db-verify-failed [0-9-]+$"
  [[ -z "$(pids_by_exe "$BIN")" ]] && ok "m_tamperdb/symlink-not-launched" || bad "m_tamperdb/symlink-not-launched"
  rm -f "$DATA/harmony_db_0/LOG"
  mv "$T/saved-LOG" "$DATA/harmony_db_0/LOG"
  chown hmytest: "$DATA/harmony_db_0/LOG"
  RB_SERVICE="$ABSENT_UNIT" run_rb "$INV" start
  expect "m_tamperdb/recovers" 0 "^RUNNING $BLS_SORTED recovery-92730034$"
}

m_wrong_head() {
  # The head pin is enforced after start over RPC (the raw-directory transfer
  # has no content hash): a wrong hash for block 92,730,034 quarantines the
  # node and LATCHES head-mismatch in the state file. Healthy RPC data alone
  # must not clear the latch or restart the node.
  cleanup_case; new_manual_case wronghead; mk_script; start_orig
  RB_SERVICE="$ABSENT_UNIT" run_rb "$INV" prepare
  expect "m_wronghead/ready" 0 "^READY " || return 0
  rpc_set 92730100 "0xbadbadbadbadbadbadbadbadbadbadbadbadbadbadbadbadbadbadbadbadbad0"
  RB_SERVICE="$ABSENT_UNIT" run_rb "$INV" start
  expect "m_wronghead/head-mismatch" 1 "^STOPPED head-mismatch [0-9-]+$"
  [[ -z "$(pids_by_exe "$BIN")" ]] && ok "m_wronghead/node-stopped" || bad "m_wronghead/node-stopped"
  [[ -n "$(state_get HEAD_MISMATCH)" ]] \
    && ok "m_wronghead/latched" || bad "m_wronghead/latched" "no HEAD_MISMATCH in state"
  local old; old="$(state_get OLD_DB_NAME)"
  [[ -d "$DATA/$old" ]] && ok "m_wronghead/old-kept" || bad "m_wronghead/old-kept"
  rpc_set
  RB_SERVICE="$ABSENT_UNIT" run_rb "$INV" start
  expect "m_wronghead/latch-blocks-restart" 1 "^STOPPED head-mismatch [0-9-]+$"
  [[ -z "$(pids_by_exe "$BIN")" ]] \
    && ok "m_wronghead/latch-not-launched" || bad "m_wronghead/latch-not-launched"
  # Only an explicit team-side clear of the latch allows a restart.
  sed -i '/^HEAD_MISMATCH=/d' "$STATE"
  RB_SERVICE="$ABSENT_UNIT" run_rb "$INV" start
  expect "m_wronghead/recovers-after-clear" 0 "^RUNNING $BLS_SORTED recovery-92730034$"
}

m_latch_active() {
  # Crash-safe latch: state already latched while a staged node is running
  # (crash or out-of-band restart after the latch was saved). The latched
  # rerun must stop the node, prove it, and only then report head-mismatch;
  # the latch itself must survive.
  cleanup_case; new_manual_case latchactive; mk_script; start_orig
  RB_SERVICE="$ABSENT_UNIT" run_rb "$INV" prepare
  expect "m_latchactive/ready" 0 "^READY " || return 0
  rpc_set 92730100 "0xbadbadbadbadbadbadbadbadbadbadbadbadbadbadbadbadbadbadbadbadbad0"
  RB_SERVICE="$ABSENT_UNIT" run_rb "$INV" start
  expect "m_latchactive/latched" 1 "^STOPPED head-mismatch [0-9-]+$" || return 0
  # Simulate the crash window: relaunch the staged node out-of-band.
  ( cd "$INV" && exec runuser -u hmytest -- setsid "$BIN" -c "$INV/harmony.conf" --datadir "$DATA" ) >/dev/null 2>&1 < /dev/null &
  wait_for 10 bash -c "[[ -n \"\$(2>/dev/null readlink /proc/*/exe | grep -Fx '$BIN')\" ]]" \
    && ok "m_latchactive/oob-restart" || bad "m_latchactive/oob-restart" "staged node did not come up"
  RB_SERVICE="$ABSENT_UNIT" run_rb "$INV" start
  expect "m_latchactive/relatch" 1 "^STOPPED head-mismatch [0-9-]+$"
  [[ -z "$(pids_by_exe "$BIN")" ]] \
    && ok "m_latchactive/requarantined" || bad "m_latchactive/requarantined" "staged node survived latched rerun"
  [[ -n "$(state_get HEAD_MISMATCH)" ]] \
    && ok "m_latchactive/latch-kept" || bad "m_latchactive/latch-kept"
  # Manual autostart relaunching the recorded ORIGINAL binary after the latch:
  # the rerun must stop it (exact recorded identity) and still report
  # head-mismatch, never leave it signing.
  start_orig
  RB_SERVICE="$ABSENT_UNIT" run_rb "$INV" start
  expect "m_latchactive/orig-relatch" 1 "^STOPPED head-mismatch [0-9-]+$"
  ! orig_running \
    && ok "m_latchactive/orig-stopped" || bad "m_latchactive/orig-stopped" "original node survived latched rerun"
  # A process that matches the duplicate scan without an unambiguous identity
  # (here: holds a DB fd / has the DataDir on its cmdline) must be left
  # running and reported stop-failed, never head-mismatch.
  setsid bash -c "exec 3< '$DATA/harmony_db_0/CURRENT'; sleep 300" >/dev/null 2>&1 < /dev/null &
  local holder=$!
  sleep 1
  RB_SERVICE="$ABSENT_UNIT" run_rb "$INV" start
  expect "m_latchactive/ambiguous-stop-failed" 1 "^STOPPED stop-failed [0-9-]+$"
  [[ -d "/proc/$holder" ]] \
    && ok "m_latchactive/ambiguous-left-running" || bad "m_latchactive/ambiguous-left-running"
  kill -9 "$holder" 2>/dev/null || true
  sleep 1
  RB_SERVICE="$ABSENT_UNIT" run_rb "$INV" start
  expect "m_latchactive/relatch-after-clear" 1 "^STOPPED head-mismatch [0-9-]+$"
  [[ -n "$(state_get HEAD_MISMATCH)" ]] \
    && ok "m_latchactive/latch-kept-final" || bad "m_latchactive/latch-kept-final"
}

m_bad_name() {
  # Same count/bytes as the pins but a non-goleveldb filename in the source:
  # metrics cannot catch it, the filename-class check must.
  cleanup_case; new_manual_case badname; start_orig
  mk_script
  sed -i "s|^DB_RCLONE_SOURCE=.*|DB_RCLONE_SOURCE=\"$T/dbsrc-badname\"|" "$RB"
  RB_SERVICE="$ABSENT_UNIT" run_rb "$INV" prepare
  expect "m_badname/db-verify-failed" 1 "^STOPPED db-verify-failed [0-9-]+$"
  orig_running && ok "m_badname/node-untouched" || bad "m_badname/node-untouched"
}

m_low_disk() {
  cleanup_case; new_manual_case lowdisk; start_orig
  mk_script "$T/dbsrc" 1152921504606846976   # 1 EiB margin floor
  RB_SERVICE="$ABSENT_UNIT" run_rb "$INV" prepare
  expect "m_lowdisk/low-disk" 1 "^STOPPED low-disk [0-9-]+$"
  orig_running && ok "m_lowdisk/node-untouched" || bad "m_lowdisk/node-untouched"
  [[ ! -e "$DATA/.hmy-recovery-92730034/db" ]] \
    && ok "m_lowdisk/no-transfer" || bad "m_lowdisk/no-transfer"
}

m_discard() {
  cleanup_case; new_manual_case discard; mk_script; start_orig
  RB_SERVICE="$ABSENT_UNIT" run_rb "$INV" prepare --discard-old-db
  expect "m_discard/confirmation-required" 1 "^STOPPED deletion-cancelled [0-9-]+$"
  [[ -f "$DATA/harmony_db_0/olddata" ]] \
    && ok "m_discard/cancel-keeps-old-db" || bad "m_discard/cancel-keeps-old-db"
  ! orig_running \
    && ok "m_discard/cancel-leaves-node-stopped" || bad "m_discard/cancel-leaves-node-stopped"
  RB_SERVICE="$ABSENT_UNIT" run_rb "$INV" prepare --discard-old-db --quiet
  expect "m_discard/ready" 0 "^READY $BLS_SORTED recovery-92730034$" || return 0
  local old; old="$(state_get OLD_DB_NAME)"
  [[ ! -e "$DATA/$old" ]] && ok "m_discard/old-deleted" || bad "m_discard/old-deleted" "$old still present"
  [[ "$(state_get OLD_DB_DISPOSITION)" == "deleted" ]] \
    && ok "m_discard/disposition" || bad "m_discard/disposition" "$(state_get OLD_DB_DISPOSITION)"
  [[ -f "$INV/harmony.conf" && -f "$INV/harmony" ]] \
    && ok "m_discard/config-binary-kept" || bad "m_discard/config-binary-kept"

  # Crash injected in DELETING with the backup still present: resume deletes it.
  mkdir -p "$DATA/$old"; printf 'x' > "$DATA/$old/f"
  sed -i -e 's/^STATE=.*/STATE=DELETING/' -e 's/^OLD_DB_DISPOSITION=.*/OLD_DB_DISPOSITION=kept/' "$STATE"
  RB_SERVICE="$ABSENT_UNIT" run_rb "$INV" prepare
  expect "m_discard/resume-deleting" 0 "^READY $BLS_SORTED recovery-92730034$"
  [[ ! -e "$DATA/$old" ]] && ok "m_discard/resumed-delete" || bad "m_discard/resumed-delete"

  # Crash in DELETING with the path already absent: counts as completed.
  sed -i -e 's/^STATE=.*/STATE=DELETING/' -e 's/^OLD_DB_DISPOSITION=.*/OLD_DB_DISPOSITION=kept/' "$STATE"
  RB_SERVICE="$ABSENT_UNIT" run_rb "$INV" prepare
  expect "m_discard/absent-after-deleting" 0 "^READY $BLS_SORTED recovery-92730034$"
  [[ "$(state_get OLD_DB_DISPOSITION)" == "deleted" ]] \
    && ok "m_discard/absent-disposition" || bad "m_discard/absent-disposition"
}

m_duplicate() {
  # Renamed/copied binary at prepare: caught by exe SHA-256 after the stop.
  cleanup_case; new_manual_case dup; mk_script; start_orig
  mkdir -p "$T/imposter"
  cp "$T/fake-orig" "$T/imposter/definitely-not-harmony"
  setsid "$T/imposter/definitely-not-harmony" >/dev/null 2>&1 < /dev/null &
  wait_for 5 bash -c "[[ -n \"\$(2>/dev/null readlink /proc/*/exe | grep -Fx '$T/imposter/definitely-not-harmony')\" ]]" || true
  RB_SERVICE="$ABSENT_UNIT" run_rb "$INV" prepare
  expect "m_dup/prepare-duplicate" 1 "^STOPPED duplicate-process [0-9-]+$"
  ! orig_running && ok "m_dup/orig-stopped-first" || bad "m_dup/orig-stopped-first"
  kill_fakes

  # Duplicate at start: reach READY cleanly, then run a RENAMED COPY OF THE
  # RECOVERY binary (distinct bytes from fake-orig: exercises the
  # NODE_BIN_SHA256 scan channel).
  cleanup_case; new_manual_case dup2; mk_script; start_orig
  RB_SERVICE="$ABSENT_UNIT" run_rb "$INV" prepare
  expect "m_dup/ready" 0 "^READY $BLS_SORTED recovery-92730034$" || return 0
  cp "$T/fake-recovery" "$T/imposter/definitely-not-harmony"
  setsid "$T/imposter/definitely-not-harmony" >/dev/null 2>&1 < /dev/null &
  wait_for 5 bash -c "[[ -n \"\$(2>/dev/null readlink /proc/*/exe | grep -Fx '$T/imposter/definitely-not-harmony')\" ]]" || true
  RB_SERVICE="$ABSENT_UNIT" run_rb "$INV" start
  expect "m_dup/start-duplicate" 1 "^STOPPED duplicate-process [0-9-]+$"
  [[ -z "$(pids_by_exe "$BIN")" ]] && ok "m_dup/not-launched" || bad "m_dup/not-launched"
  [[ "$(state_get STATE)" == "READY" ]] && ok "m_dup/still-ready" || bad "m_dup/still-ready" "$(state_get STATE)"
  kill_fakes

  # Duplicate appearing AFTER RUNNING: the start rerun must stop the
  # legitimate node before reporting STOPPED duplicate-process.
  cleanup_case; new_manual_case dup3; mk_script; start_orig
  RB_SERVICE="$ABSENT_UNIT" run_rb "$INV" prepare
  expect "m_dup/dup3-ready" 0 "^READY " || return 0
  RB_SERVICE="$ABSENT_UNIT" run_rb "$INV" start
  expect "m_dup/dup3-running" 0 "^RUNNING " || return 0
  cp "$T/fake-recovery" "$T/imposter/definitely-not-harmony"
  setsid "$T/imposter/definitely-not-harmony" >/dev/null 2>&1 < /dev/null &
  wait_for 5 bash -c "[[ -n \"\$(2>/dev/null readlink /proc/*/exe | grep -Fx '$T/imposter/definitely-not-harmony')\" ]]" || true
  RB_SERVICE="$ABSENT_UNIT" run_rb "$INV" start
  expect "m_dup/post-running-duplicate" 1 "^STOPPED duplicate-process [0-9-]+$"
  [[ -z "$(pids_by_exe "$BIN")" ]] \
    && ok "m_dup/legit-node-stopped" || bad "m_dup/legit-node-stopped" "staged binary still running"
}

m_wrong_arch() {
  # Host-arch constants pinned to a wrong-arch ELF: the sha matches the served
  # bytes, so only the ELF e_machine check can (and must) reject it.
  cleanup_case; new_manual_case wrongarch; mk_script; start_orig
  local harch; harch="$(host_arch_token)"
  sed -i \
    -e "s|^NODE_BIN_URL_$harch=.*|NODE_BIN_URL_$harch=\"$HTTP/node-other.bin\"|" \
    -e "s|^NODE_BIN_SHA256_$harch=.*|NODE_BIN_SHA256_$harch=\"$OTHER_SHA\"|" "$RB"
  RB_SERVICE="$ABSENT_UNIT" run_rb "$INV" prepare
  expect "m_wrongarch/rejected" 1 "^STOPPED download-failed [0-9-]+$"
  orig_running && ok "m_wrongarch/node-untouched" || bad "m_wrongarch/node-untouched"
  [[ ! -f "$BIN" ]] && ok "m_wrongarch/not-staged" || bad "m_wrongarch/not-staged"
}

m_rootless() {
  # Full two-command flow as hmytest without sudo: state/staging live under
  # the invocation dir and every touched file stays owned by hmytest.
  cleanup_case; new_manual_case rootless; mk_script; start_orig
  local rwork="$INV/.hmy-recovery-92730034/work"
  local rbin="$rwork/bin/harmony-recovery" rstate="$rwork/private/state" rooty np
  RB_SERVICE="$ABSENT_UNIT" run_rb_user hmytest "$INV" prepare
  expect "m_rootless/prepare" 0 "^READY $BLS_SORTED recovery-92730034$" || return 0
  ! orig_running && ok "m_rootless/node-stopped" || bad "m_rootless/node-stopped"
  [[ ! -e "$WORK" ]] && ok "m_rootless/no-var-lib" || bad "m_rootless/no-var-lib" "created $WORK"
  [[ -f "$rstate" && "$(stat -c %U:%a "$rstate")" == "hmytest:600" ]] \
    && ok "m_rootless/state-user-0600" || bad "m_rootless/state-user-0600" "$(stat -c %U:%a "$rstate" 2>/dev/null)"
  [[ "$(sed -n 's/^STATE=//p' "$rstate")" == "READY" ]] \
    && ok "m_rootless/state-ready" || bad "m_rootless/state-ready" "$(sed -n 's/^STATE=//p' "$rstate")"
  [[ -d "$DATA/$(sed -n 's/^OLD_DB_NAME=//p' "$rstate")" ]] \
    && ok "m_rootless/old-kept" || bad "m_rootless/old-kept"
  rooty="$(find "$INV" ! -user hmytest -print 2>/dev/null | head -5)"
  [[ -z "$rooty" ]] && ok "m_rootless/ownership-preserved" || bad "m_rootless/ownership-preserved" "$rooty"

  RB_SERVICE="$ABSENT_UNIT" run_rb_user hmytest "$INV" start
  expect "m_rootless/start" 0 "^RUNNING $BLS_SORTED recovery-92730034$" || return 0
  np="$(sed -n 's/^NODE_PID=//p' "$rstate")"
  [[ -n "$np" && -d "/proc/$np" && "$(stat -c %U "/proc/$np")" == "hmytest" ]] \
    && ok "m_rootless/runs-as-user" || bad "m_rootless/runs-as-user" "np=$np"
  [[ "$(readlink "/proc/$np/exe" 2>/dev/null)" == "$rbin" ]] \
    && ok "m_rootless/exe-staged" || bad "m_rootless/exe-staged"
  rooty="$(find "$INV" ! -user hmytest -print 2>/dev/null | head -5)"
  [[ -z "$rooty" ]] && ok "m_rootless/ownership-after-start" || bad "m_rootless/ownership-after-start" "$rooty"

  RB_SERVICE="$ABSENT_UNIT" run_rb_user hmytest "$INV" start
  expect "m_rootless/start-rerun" 0 "^RUNNING $BLS_SORTED recovery-92730034$"
  kill_fakes
}

m_extra_flags() {
  cleanup_case; new_manual_case extraflags; mk_script
  mkdir -p "$INV/k"; chown hmytest: "$INV/k"
  start_orig --bls.dir ./k --consensus.min-peers=6 --p2p.port=9001 \
    --http.port=9500 --sync.client=true --log.verb=3
  RB_SERVICE="$ABSENT_UNIT" run_rb "$INV" prepare
  expect "m_extraflags/ready" 0 "^READY $BLS_SORTED recovery-92730034$" || return 0
  [[ "$(state_get ORIG_ARGS)" == "-c ./harmony.conf --bls.dir ./k --consensus.min-peers=6 --p2p.port=9001 --http.port=9500 --sync.client=true --log.verb=3" ]] \
    && ok "m_extraflags/state-preserved" || bad "m_extraflags/state-preserved"
  RB_SERVICE="$ABSENT_UNIT" run_rb "$INV" start
  expect "m_extraflags/running" 0 "^RUNNING $BLS_SORTED recovery-92730034$" || return 0
  local np; np="$(state_get NODE_PID)"
  tr '\0' ' ' < "/proc/$np/cmdline" | grep -q -- '--bls.dir ./k.*--p2p.port=9001.*--sync.client=true' \
    && ok "m_extraflags/running-preserved" || bad "m_extraflags/running-preserved"
}

m_ambiguous() {
  # Two candidate processes anchored in the invocation directory.
  cleanup_case; new_manual_case ambig; mk_script; start_orig
  cp "$T/fake-orig" "$INV/harmony2"; chown hmytest: "$INV/harmony2"
  ( cd "$INV" && exec runuser -u hmytest -- setsid ./harmony2 -c ./harmony.conf ) >/dev/null 2>&1 < /dev/null &
  wait_for 5 bash -c "[[ -n \"\$(2>/dev/null readlink /proc/*/exe | grep -Fx '$INV/harmony2')\" ]]" || true
  RB_SERVICE="$ABSENT_UNIT" run_rb "$INV" prepare
  expect "m_ambig/two-candidates" 1 "^STOPPED unsupported-layout [0-9-]+$"
  orig_running && ok "m_ambig/node-untouched" || bad "m_ambig/node-untouched"
  kill_fakes

  # One candidate but no config on its command line.
  cleanup_case; new_manual_case noconf; mk_script
  ( cd "$INV" && exec runuser -u hmytest -- setsid ./harmony ) >/dev/null 2>&1 < /dev/null &
  wait_for 5 bash -c "[[ -n \"\$(2>/dev/null readlink /proc/*/exe | grep -Fx '$INV/harmony')\" ]]" || true
  RB_SERVICE="$ABSENT_UNIT" run_rb "$INV" prepare
  expect "m_ambig/no-config" 1 "^STOPPED unsupported-layout [0-9-]+$"
}

m_respawner() {
  cleanup_case; new_manual_case respawn; mk_script
  cat > "$T/respawn.sh" <<EOF
while :; do
  ( cd "$INV" && exec ./harmony -c ./harmony.conf ) >/dev/null 2>&1
  sleep 1
done
EOF
  setsid runuser -u hmytest -- bash "$T/respawn.sh" >/dev/null 2>&1 < /dev/null &
  wait_for 10 orig_running || { bad "m_respawn/fixture" "node never started"; return 0; }
  RB_SERVICE="$ABSENT_UNIT" run_rb "$INV" prepare
  expect "m_respawn/refused" 1 "^STOPPED unsupported-layout [0-9-]+$"
  pkill -9 -f 'bash /var/tmp/rbsmoke/respawn.sh' 2>/dev/null || true
  kill_fakes
}

write_state_manual() { # <state> <old-present:cur-present:staged-present>
  local st="$1" spec="$2"
  local old="harmony_db_0.pre-recovery-test"
  install -d -m 0711 "$WORK"; install -d -m 0755 "$WORK/bin"; install -d -m 0700 "$PRIV"
  cp -f "$T/fake-recovery" "$BIN"; chmod 0755 "$BIN"
  rm -rf "${DATA:?}/harmony_db_0" "${DATA:?}/$old" "${DATA:?}/.hmy-recovery-92730034"
  IFS=: read -r oldp curp stagedp <<< "$spec"
  if [[ "$oldp" == 1 ]]; then
    mkdir -p "$DATA/$old"; printf 'old' > "$DATA/$old/olddata"
  fi
  if [[ "$curp" == 1 ]]; then
    # The current DB must pass the pinned-tree verification.
    cp -a "$T/dbsrc" "$DATA/harmony_db_0"
  fi
  if [[ "$stagedp" == 1 ]]; then
    mkdir -p "$DATA/.hmy-recovery-92730034/db"
    cp -a "$T/dbsrc" "$DATA/.hmy-recovery-92730034/db/harmony_db_0"
  fi
  cat > "$STATE" <<EOF
BLS_IDS=$BLS_SORTED
CONFIG=$INV/harmony.conf
DATADIR=$DATA
DISCARD_REQUESTED=0
INVOCATION_DIR=$INV
LAYOUT=manual-directory
OLD_DB_DISPOSITION=kept
OLD_DB_NAME=$old
ORIG_EXE=$INV/harmony
ORIG_EXE_SHA256=$ORIG_SHA
ORIG_PID=999999
RUN_CWD=$INV
RUN_USER=hmytest
STATE=$st
EOF
  chmod 600 "$STATE"
}

m_state_matrix() {
  new_manual_case statematrix
  mk_script
  local variant st spec
  for variant in "SWAP_BEGUN:0:1:1" "SWAP_BEGUN:1:0:1" "OLD_RENAMED:1:0:1" \
                 "OLD_RENAMED:1:1:0" "NEW_INSTALLED:1:1:0" "READY:1:1:0"; do
    st="${variant%%:*}"; spec="${variant#*:}"
    cleanup_case
    write_state_manual "$st" "$spec"
    RB_SERVICE="$ABSENT_UNIT" run_rb "$INV" prepare
    expect "m_matrix/$st-$spec" 0 "^READY $BLS_SORTED recovery-92730034$" || continue
    [[ "$(state_get STATE)" == "READY" && -f "$DATA/harmony_db_0/MANIFEST-000001" && -d "$DATA/harmony_db_0.pre-recovery-test" ]] \
      && ok "m_matrix/$st-$spec-fs" || bad "m_matrix/$st-$spec-fs"
  done
}

m_bad_oldname() {
  # OLD_DB_NAME containing a path traversal must be refused before any
  # deletion: the victim directory outside DataDir survives.
  new_manual_case badold
  mk_script
  cleanup_case
  write_state_manual DELETING "1:1:0"
  mkdir -p "$INV/victim-outside"; printf 'v' > "$INV/victim-outside/keep"
  sed -i 's|^OLD_DB_NAME=.*|OLD_DB_NAME=harmony_db_0.pre-recovery-test/../../victim-outside|' "$STATE"
  RB_SERVICE="$ABSENT_UNIT" run_rb "$INV" prepare
  expect "m_badold/refused" 1 "^STOPPED cannot-determine-state [0-9-]+$"
  [[ -f "$INV/victim-outside/keep" ]] \
    && ok "m_badold/victim-intact" || bad "m_badold/victim-intact" "victim deleted"
}

m_start_reconcile() {
  # STARTING with nothing running: relaunch without the cold pre-launch DB
  # verification (which would fail after this deliberate tamper).
  cleanup_case; new_manual_case reconcile; mk_script; start_orig
  RB_SERVICE="$ABSENT_UNIT" run_rb "$INV" prepare
  expect "m_reconcile/ready" 0 "^READY " || return 0
  sed -i 's/^STATE=.*/STATE=STARTING/' "$STATE"
  printf 'x' > "$DATA/harmony_db_0/extra-file"   # cold verify would fail if re-run
  RB_SERVICE="$ABSENT_UNIT" run_rb "$INV" start
  expect "m_reconcile/starting-relaunch" 0 "^RUNNING $BLS_SORTED recovery-92730034$"

  # STARTING with the node already running: adopt it (same PID recorded).
  local np; np="$(state_get NODE_PID)"
  sed -i 's/^STATE=.*/STATE=STARTING/' "$STATE"
  RB_SERVICE="$ABSENT_UNIT" run_rb "$INV" start
  expect "m_reconcile/starting-adopt" 0 "^RUNNING $BLS_SORTED recovery-92730034$"
  [[ "$(state_get NODE_PID)" == "$np" && -d "/proc/$np" ]] \
    && ok "m_reconcile/adopted-same-pid" || bad "m_reconcile/adopted-same-pid"

  # STARTED with the node down: start again.
  kill -9 "$np" 2>/dev/null || true
  sleep 1
  RB_SERVICE="$ABSENT_UNIT" run_rb "$INV" start
  expect "m_reconcile/started-down-restart" 0 "^RUNNING $BLS_SORTED recovery-92730034$"

  # STARTED but unhealthy (wrong blskey set): stopped again, then recovers.
  rpc_set 92730100 "$TARGET_HASH" "[\"$K1\"]"
  RB_SERVICE="$ABSENT_UNIT" run_rb "$INV" start
  expect "m_reconcile/unhealthy" 1 "^STOPPED unhealthy [0-9-]+$"
  [[ -z "$(pids_by_exe "$BIN")" ]] && ok "m_reconcile/unhealthy-stopped" || bad "m_reconcile/unhealthy-stopped"
  rpc_set
  RB_SERVICE="$ABSENT_UNIT" run_rb "$INV" start
  expect "m_reconcile/recovered" 0 "^RUNNING $BLS_SORTED recovery-92730034$"
}

m_failed_start() {
  cleanup_case; new_manual_case failstart; mk_script; start_orig
  RB_SERVICE="$ABSENT_UNIT" run_rb "$INV" prepare
  expect "m_failstart/ready" 0 "^READY " || return 0
  touch "$DIE"
  RB_SERVICE="$ABSENT_UNIT" run_rb "$INV" start
  expect "m_failstart/start-failed" 1 "^STOPPED start-failed [0-9-]+$"
  [[ -z "$(pids_by_exe "$BIN")" ]] && ok "m_failstart/stopped" || bad "m_failstart/stopped"
  [[ "$(state_get STATE)" == "STARTING" ]] && ok "m_failstart/state" || bad "m_failstart/state" "$(state_get STATE)"
  rm -f "$DIE"
  RB_SERVICE="$ABSENT_UNIT" run_rb "$INV" start
  expect "m_failstart/recovers" 0 "^RUNNING $BLS_SORTED recovery-92730034$"
}

m_usage() {
  cleanup_case; mkdir -p "$T/cases/usage"; mk_script
  run_rb "$T/cases/usage"
  expect "m_usage/no-mode" 2 "^usage: "
  run_rb "$T/cases/usage" bogus
  expect "m_usage/bad-mode" 2 "^usage: "
  run_rb "$T/cases/usage" start --discard-old-db
  expect "m_usage/start-with-flag" 2 "^usage: "
  run_rb "$T/cases/usage" prepare --discard-old-db extra
  expect "m_usage/extra-arg" 2 "^usage: "
  run_rb "$T/cases/usage" start --systemd-unit
  expect "m_usage/missing-unit" 2 "^usage: "
  run_rb "$T/cases/usage" start --systemd-unit ../bad.service
  expect "m_usage/unsafe-unit" 2 "^usage: "
  run_rb "$T/cases/usage" start --systemd-unit 'validator@.service'
  expect "m_usage/template-unit" 2 "^usage: "
  run_rb "$T/cases/usage" start
  expect "m_usage/start-before-prepare" 1 "^STOPPED not-ready [0-9-]+$"
}

run_manual() {
  # `|| true` so one failing case cannot abort the suite before the summary.
  m_happy || true
  m_source_mismatch || true
  m_resume_partial || true
  m_bad_source || true
  m_bad_name || true
  m_tampered_installed || true
  m_wrong_head || true
  m_latch_active || true
  m_low_disk || true
  m_discard || true
  m_duplicate || true
  m_wrong_arch || true
  m_rootless || true
  m_extra_flags || true
  m_ambiguous || true
  m_respawner || true
  m_state_matrix || true
  m_bad_oldname || true
  m_start_reconcile || true
  m_failed_start || true
  m_usage || true
  cleanup_case
}

# ---------- systemd fixture and cases ----------

new_systemd_case() {
  cleanup_case
  systemctl stop harmony.service >/dev/null 2>&1 || true
  rm -rf /home/harmony/data /home/harmony/blskeys /home/harmony/logs
  mkdir -p /home/harmony/data/harmony_db_0 /home/harmony/blskeys /home/harmony/logs /etc/harmony
  : > /home/harmony/p2p.key
  printf 'x' > /home/harmony/data/harmony_db_0/CURRENT
  printf 'old' > /home/harmony/data/harmony_db_0/olddata
  cp -f "$T/fake-orig" /usr/sbin/harmony
  cat > /etc/harmony/harmony.conf <<'EOF'
Version = "2.6.2"

[General]
NodeType = "validator"
IsArchival = false
DataDir = "/home/harmony/data"

[Network]
NetworkType = "mainnet"

[ShardData]
EnableShardData = false
EOF
  cat > /etc/systemd/system/harmony.service <<'EOF'
[Unit]
Description=harmony validator node service
After=network.target

[Service]
Type=simple
Restart=on-failure
RestartSec=1
User=harmony
Group=harmony
WorkingDirectory=/home/harmony
ExecStart=/usr/sbin/harmony -c /etc/harmony/harmony.conf --consensus.aggregate-sig=false --consensus.min-peers=6 --bls.dir=/home/harmony/blskeys --p2p.keyfile=/home/harmony/p2p.key --p2p.port=9000 --http=true --http.port=9500 --sync=true --sync.client=true --log.dir=/home/harmony/logs --prometheus=true --prometheus.port=9900
StartLimitInterval=0

[Install]
WantedBy=multi-user.target
EOF
  chown -R harmony: /home/harmony/data /home/harmony/blskeys /home/harmony/logs /home/harmony/p2p.key
  systemctl daemon-reload
  systemctl enable harmony.service >/dev/null 2>&1
  systemctl start harmony.service
  wait_for 10 bash -c '[[ "$(systemctl is-active harmony.service)" == active ]]' \
    || { echo "systemd fixture failed to start"; exit 1; }
  SDATA=/home/harmony/data
}

s_multiple_units() {
  new_systemd_case; mk_script

  systemctl stop harmony.service
  mv /etc/systemd/system/harmony.service "/etc/systemd/system/$UNIT_A"
  mkdir -p /home/rbsmoke-b/data/harmony_db_0
  printf 'x' > /home/rbsmoke-b/data/harmony_db_0/CURRENT
  printf 'old' > /home/rbsmoke-b/data/harmony_db_0/olddata
  cat > /etc/harmony/rbsmoke-b.conf <<'EOF'
Version = "2.6.2"

[General]
NodeType = "validator"
IsArchival = false
DataDir = "/home/rbsmoke-b/data"

[Network]
NetworkType = "mainnet"

[ShardData]
EnableShardData = false
EOF
  cat > "/etc/systemd/system/$UNIT_B" <<EOF
[Service]
Type=simple
User=harmony
WorkingDirectory=/home/rbsmoke-b
ExecStart=/usr/sbin/harmony -c /etc/harmony/rbsmoke-b.conf --http.port=9600
EOF
  chown -R harmony: /home/rbsmoke-b
  systemctl daemon-reload
  systemctl start "$UNIT_A" "$UNIT_B"
  wait_for 10 bash -c "[[ \"\$(systemctl is-active '$UNIT_A')\" == active && \"\$(systemctl is-active '$UNIT_B')\" == active ]]" \
    || { bad "s_multi/fixture" "units did not start"; return 0; }

  run_rb /root prepare --systemd-unit "$UNIT_A"
  expect "s_multi/a-prepare" 0 "^READY $BLS_SORTED recovery-92730034$" || return 0
  [[ "$(systemctl is-active "$UNIT_B")" == active ]] \
    && ok "s_multi/b-kept-running" || bad "s_multi/b-kept-running"
  [[ "$(unit_state_get "$UNIT_A" UNIT)" == "$UNIT_A" ]] \
    && ok "s_multi/a-state" || bad "s_multi/a-state"

  run_rb /root start --systemd-unit "$UNIT_A"
  expect "s_multi/a-start" 0 "^RUNNING $BLS_SORTED recovery-92730034$" || return 0
  run_rb /root prepare --systemd-unit "$UNIT_B"
  expect "s_multi/b-prepare" 0 "^READY $K3 recovery-92730034$" || return 0
  [[ "$(systemctl is-active "$UNIT_A")" == active ]] \
    && ok "s_multi/a-kept-running" || bad "s_multi/a-kept-running"
  [[ "$(unit_state_get "$UNIT_B" RPC_URL)" == "http://127.0.0.1:9600" ]] \
    && ok "s_multi/b-rpc-port" || bad "s_multi/b-rpc-port"
  [[ -f "$WORK/units/$UNIT_A/private/state" && -f "$WORK/units/$UNIT_B/private/state" ]] \
    && ok "s_multi/separate-state" || bad "s_multi/separate-state"
}

s_happy() {
  new_systemd_case; mk_script
  local orig_sha_before; orig_sha_before="$(sha256sum /usr/sbin/harmony | awk '{print $1}')"
  run_rb /root prepare
  expect "s_happy/prepare" 0 "^READY $BLS_SORTED recovery-92730034$" || return 0
  [[ "$(systemctl is-active harmony.service)" == "inactive" ]] \
    && ok "s_happy/unit-inactive" || bad "s_happy/unit-inactive" "$(systemctl is-active harmony.service)"
  [[ -f /etc/systemd/system/harmony.service.d/99-harmony-recovery-hold.conf ]] \
    && ok "s_happy/hold-present" || bad "s_happy/hold-present"
  [[ -f /etc/systemd/system/harmony.service.d/50-harmony-recovery-exec.conf ]] \
    && ok "s_happy/exec-dropin-present" || bad "s_happy/exec-dropin-present"
  grep -q -- '--consensus.aggregate-sig=false' /etc/systemd/system/harmony.service.d/50-harmony-recovery-exec.conf \
    && ok "s_happy/aggregate-sig-preserved" || bad "s_happy/aggregate-sig-preserved"
  grep -q -- '--bls.dir=/home/harmony/blskeys.*--p2p.port=9000.*--http.port=9500.*--prometheus.port=9900' \
    /etc/systemd/system/harmony.service.d/50-harmony-recovery-exec.conf \
    && ok "s_happy/common-flags-preserved" || bad "s_happy/common-flags-preserved"
  systemctl start harmony.service >/dev/null 2>&1 || true
  sleep 2
  [[ "$(systemctl is-active harmony.service)" != "active" ]] \
    && ok "s_happy/hold-blocks-start" || bad "s_happy/hold-blocks-start"
  local old; old="$(state_get OLD_DB_NAME)"
  [[ -d "$SDATA/$old" ]] && ok "s_happy/old-renamed" || bad "s_happy/old-renamed"
  [[ "$(sha256sum /usr/sbin/harmony | awk '{print $1}')" == "$orig_sha_before" ]] \
    && ok "s_happy/orig-binary-intact" || bad "s_happy/orig-binary-intact"
  [[ "$(stat -c %U "$SDATA/harmony_db_0")" == "harmony" ]] \
    && ok "s_happy/new-owned" || bad "s_happy/new-owned"

  run_rb /root prepare
  expect "s_happy/prepare-rerun" 0 "^READY $BLS_SORTED recovery-92730034$"

  run_rb /root start
  expect "s_happy/start" 0 "^RUNNING $BLS_SORTED recovery-92730034$" || return 0
  [[ "$(systemctl is-active harmony.service)" == "active" ]] \
    && ok "s_happy/unit-active" || bad "s_happy/unit-active"
  local mp; mp="$(systemctl show harmony.service -p MainPID --value)"
  [[ "$(readlink "/proc/$mp/exe")" == "$BIN" ]] \
    && ok "s_happy/unit-runs-staged-bin" || bad "s_happy/unit-runs-staged-bin"
  tr '\0' ' ' < "/proc/$mp/cmdline" | grep -q -- '--consensus.aggregate-sig=false' \
    && ok "s_happy/running-aggregate-sig" || bad "s_happy/running-aggregate-sig"
  tr '\0' ' ' < "/proc/$mp/cmdline" | grep -q -- '--bls.dir=/home/harmony/blskeys.*--sync.client=true.*--log.dir=/home/harmony/logs' \
    && ok "s_happy/running-common-flags" || bad "s_happy/running-common-flags"
  [[ ! -f /etc/systemd/system/harmony.service.d/99-harmony-recovery-hold.conf ]] \
    && ok "s_happy/hold-removed" || bad "s_happy/hold-removed"
  [[ ! -e "$SENT_DIR/GO" ]] && ok "s_happy/go-removed" || bad "s_happy/go-removed"

  run_rb /root start
  expect "s_happy/start-rerun" 0 "^RUNNING $BLS_SORTED recovery-92730034$"
}

s_predeleted_db() {
  # Supervised low-space case: systemd is stopped, the old DB was already
  # deleted, and the operator supplies the known public BLS IDs.
  new_systemd_case; mk_script
  systemctl stop harmony.service
  rm -rf /home/harmony/data/harmony_db_0
  sed -i 's/--http.port=9500/--http.port=19500/' /etc/systemd/system/harmony.service
  systemctl daemon-reload
  run_rb /root prepare --discard-old-db
  expect "s_predeleted/ready" 0 "^READY unknown recovery-92730034$" || return 0
  [[ "$(systemctl is-active harmony.service)" != "active" ]] \
    && ok "s_predeleted/unit-stopped" || bad "s_predeleted/unit-stopped"
  [[ -f /home/harmony/data/harmony_db_0/MANIFEST-000001 ]] \
    && ok "s_predeleted/clean-db-installed" || bad "s_predeleted/clean-db-installed"
  [[ "$(state_get BLS_IDS)" == "unknown" ]] \
    && ok "s_predeleted/bls-recorded" || bad "s_predeleted/bls-recorded"
  [[ "$(state_get OLD_DB_DISPOSITION)" == "deleted" ]] \
    && ok "s_predeleted/disposition" || bad "s_predeleted/disposition"
  local old; old="$(state_get OLD_DB_NAME)"
  [[ -n "$old" && ! -e "/home/harmony/data/$old" ]] \
    && ok "s_predeleted/placeholder-removed" || bad "s_predeleted/placeholder-removed"
}

s_conflicting_dropin() {
  new_systemd_case; mk_script
  cp -f "$T/fake-orig" /usr/sbin/harmony-evil
  mkdir -p /etc/systemd/system/harmony.service.d
  cat > /etc/systemd/system/harmony.service.d/zz-conflict.conf <<'EOF'
[Service]
ExecStart=
ExecStart=/usr/sbin/harmony-evil -c /etc/harmony/harmony.conf
EOF
  systemctl daemon-reload
  systemctl restart harmony.service
  wait_for 10 bash -c '[[ "$(systemctl is-active harmony.service)" == active ]]' || true
  run_rb /root prepare
  expect "s_conflict/refused" 1 "^STOPPED unsupported-layout [0-9-]+$"
  [[ "$(systemctl is-active harmony.service)" != "active" ]] \
    && ok "s_conflict/stopped-held" || bad "s_conflict/stopped-held"
  [[ -f /etc/systemd/system/harmony.service.d/99-harmony-recovery-hold.conf ]] \
    && ok "s_conflict/hold-present" || bad "s_conflict/hold-present"
}

s_leftover_hold() {
  new_systemd_case; mk_script
  run_rb /root prepare
  expect "s_leftover/ready" 0 "^READY " || return 0
  run_rb /root start
  expect "s_leftover/running" 0 "^RUNNING " || return 0
  # Simulate a crash between STATE=STARTED and hold removal.
  mkdir -p /etc/systemd/system/harmony.service.d "$SENT_DIR"
  cat > /etc/systemd/system/harmony.service.d/99-harmony-recovery-hold.conf <<EOF
[Unit]
ConditionPathExists=$SENT_DIR/GO
EOF
  : > "$SENT_DIR/GO"
  systemctl daemon-reload
  run_rb /root start
  expect "s_leftover/heals" 0 "^RUNNING $BLS_SORTED recovery-92730034$"
  [[ ! -f /etc/systemd/system/harmony.service.d/99-harmony-recovery-hold.conf ]] \
    && ok "s_leftover/hold-removed" || bad "s_leftover/hold-removed"
  [[ ! -e "$SENT_DIR/GO" ]] && ok "s_leftover/go-removed" || bad "s_leftover/go-removed"
}

s_unhealthy_post() {
  new_systemd_case; mk_script
  run_rb /root prepare
  expect "s_unhealthy/ready" 0 "^READY " || return 0
  run_rb /root start
  expect "s_unhealthy/running" 0 "^RUNNING " || return 0
  # Wrong BLS key set (target hash stays correct): generic unhealthy, no
  # head-mismatch latch.
  rpc_set 92730100 "$TARGET_HASH" "[\"$K1\"]"
  run_rb /root start
  expect "s_unhealthy/unhealthy" 1 "^STOPPED unhealthy [0-9-]+$"
  [[ "$(systemctl is-active harmony.service)" != "active" ]] \
    && ok "s_unhealthy/stopped" || bad "s_unhealthy/stopped"
  [[ -f /etc/systemd/system/harmony.service.d/99-harmony-recovery-hold.conf ]] \
    && ok "s_unhealthy/hold-reinstalled" || bad "s_unhealthy/hold-reinstalled"
  systemctl start harmony.service >/dev/null 2>&1 || true
  sleep 2
  [[ "$(systemctl is-active harmony.service)" != "active" ]] \
    && ok "s_unhealthy/hold-blocks" || bad "s_unhealthy/hold-blocks"
  rpc_set
  run_rb /root start
  expect "s_unhealthy/recovers" 0 "^RUNNING $BLS_SORTED recovery-92730034$"
}

s_latch_active() {
  # Crash-safe latch on systemd: latch head-mismatch, then simulate the crash
  # window by removing the hold and restarting the unit out-of-band. A
  # latched rerun with a neutered stop must report stop-failed (never
  # head-mismatch while the unit runs); an honest rerun must stop the unit,
  # reinstall the hold, keep the latch, and report head-mismatch.
  new_systemd_case; mk_script
  sed -i 's/^STOP_TIMEOUT=.*/STOP_TIMEOUT=6/' "$RB"
  run_rb /root prepare
  expect "s_latchactive/ready" 0 "^READY " || return 0
  run_rb /root start
  expect "s_latchactive/running" 0 "^RUNNING " || return 0
  rpc_set 92730100 "0xbadbadbadbadbadbadbadbadbadbadbadbadbadbadbadbadbadbadbadbadbad0"
  run_rb /root start
  expect "s_latchactive/latched" 1 "^STOPPED head-mismatch [0-9-]+$" || return 0
  rm -f /etc/systemd/system/harmony.service.d/99-harmony-recovery-hold.conf
  systemctl daemon-reload
  systemctl start harmony.service >/dev/null 2>&1 || true
  wait_for 10 bash -c '[[ "$(systemctl is-active harmony.service)" == "active" ]]' \
    && ok "s_latchactive/oob-restart" || bad "s_latchactive/oob-restart" "$(systemctl is-active harmony.service)"
  mkdir -p "$T/fakectl"
  cat > "$T/fakectl/systemctl" <<'EOF'
#!/usr/bin/env bash
[[ "${1-}" == "stop" ]] && exit 0   # pretend to stop, change nothing
exec /usr/bin/systemctl "$@"
EOF
  chmod 755 "$T/fakectl/systemctl"
  set +e
  RB_OUT="$(cd /root && PATH="$T/fakectl:$PATH" bash "$RB" start)"
  RB_RC=$?
  set -e
  [[ $RB_RC -eq 1 && "$RB_OUT" =~ ^STOPPED\ stop-failed\ [0-9-]+$ ]] \
    && ok "s_latchactive/stop-failed" || bad "s_latchactive/stop-failed" "rc=$RB_RC out=$RB_OUT"
  [[ "$(systemctl is-active harmony.service)" == "active" ]] \
    && ok "s_latchactive/still-active-as-simulated" || bad "s_latchactive/still-active-as-simulated" "$(systemctl is-active harmony.service)"
  rm -rf "$T/fakectl"
  run_rb /root start
  expect "s_latchactive/relatch" 1 "^STOPPED head-mismatch [0-9-]+$"
  [[ "$(systemctl is-active harmony.service)" != "active" ]] \
    && ok "s_latchactive/requarantined" || bad "s_latchactive/requarantined" "$(systemctl is-active harmony.service)"
  [[ -f /etc/systemd/system/harmony.service.d/99-harmony-recovery-hold.conf ]] \
    && ok "s_latchactive/hold-reinstalled" || bad "s_latchactive/hold-reinstalled"
  [[ -n "$(state_get HEAD_MISMATCH)" ]] \
    && ok "s_latchactive/latch-kept" || bad "s_latchactive/latch-kept"
  rpc_set
}

s_failed_start() {
  new_systemd_case; mk_script
  run_rb /root prepare
  expect "s_failstart/ready" 0 "^READY " || return 0
  touch "$DIE"
  run_rb /root start
  expect "s_failstart/start-failed" 1 "^STOPPED start-failed [0-9-]+$"
  [[ "$(systemctl is-active harmony.service)" != "active" ]] \
    && ok "s_failstart/stopped" || bad "s_failstart/stopped"
  [[ -f /etc/systemd/system/harmony.service.d/99-harmony-recovery-hold.conf ]] \
    && ok "s_failstart/hold-intact" || bad "s_failstart/hold-intact"
  [[ ! -e "$SENT_DIR/GO" ]] && ok "s_failstart/go-removed" || bad "s_failstart/go-removed"
  rm -f "$DIE"
  run_rb /root start
  expect "s_failstart/recovers" 0 "^RUNNING $BLS_SORTED recovery-92730034$"
}

s_tampered_exec() {
  # After RUNNING, a tampering drop-in points ExecStart at another binary and
  # the unit is restarted. The start rerun must refuse with receipt-mismatch
  # AND stop + hold the unit instead of accepting it.
  new_systemd_case; mk_script
  run_rb /root prepare
  expect "s_tamper/ready" 0 "^READY " || return 0
  run_rb /root start
  expect "s_tamper/running" 0 "^RUNNING " || return 0
  cp -f "$T/fake-orig" /usr/sbin/harmony-evil
  mkdir -p /etc/systemd/system/harmony.service.d
  cat > /etc/systemd/system/harmony.service.d/zz-conflict.conf <<'EOF'
[Service]
ExecStart=
ExecStart=/usr/sbin/harmony-evil -c /etc/harmony/harmony.conf
EOF
  systemctl daemon-reload
  systemctl restart harmony.service
  wait_for 10 bash -c '[[ "$(systemctl is-active harmony.service)" == active ]]' || true
  run_rb /root start
  expect "s_tamper/receipt-mismatch" 1 "^STOPPED receipt-mismatch [0-9-]+$"
  [[ "$(systemctl is-active harmony.service)" != "active" ]] \
    && ok "s_tamper/stopped" || bad "s_tamper/stopped" "$(systemctl is-active harmony.service)"
  [[ -f /etc/systemd/system/harmony.service.d/99-harmony-recovery-hold.conf ]] \
    && ok "s_tamper/held" || bad "s_tamper/held"
}

s_started_restart_failure() {
  # Node down in STATE=STARTED (hold already removed): a failed relaunch must
  # leave the hold reinstalled so the enabled unit cannot start without GO.
  new_systemd_case; mk_script
  run_rb /root prepare
  expect "s_restartfail/ready" 0 "^READY " || return 0
  run_rb /root start
  expect "s_restartfail/running" 0 "^RUNNING " || return 0
  systemctl stop harmony.service
  sleep 1
  touch "$DIE"
  run_rb /root start
  expect "s_restartfail/start-failed" 1 "^STOPPED start-failed [0-9-]+$"
  [[ -f /etc/systemd/system/harmony.service.d/99-harmony-recovery-hold.conf ]] \
    && ok "s_restartfail/hold-reinstalled" || bad "s_restartfail/hold-reinstalled"
  systemctl start harmony.service >/dev/null 2>&1 || true
  sleep 2
  [[ "$(systemctl is-active harmony.service)" != "active" ]] \
    && ok "s_restartfail/hold-blocks" || bad "s_restartfail/hold-blocks"
  rm -f "$DIE"
  run_rb /root start
  expect "s_restartfail/recovers" 0 "^RUNNING $BLS_SORTED recovery-92730034$"
}

s_started_duplicate() {
  # A second process using the selected config after RUNNING must stop and
  # hold the legitimate unit before reporting the duplicate.
  new_systemd_case; mk_script
  run_rb /root prepare
  expect "s_dup/ready" 0 "^READY " || return 0
  run_rb /root start
  expect "s_dup/running" 0 "^RUNNING " || return 0
  cp "$T/fake-recovery" "$T/imposter/definitely-not-harmony"
  setsid "$T/imposter/definitely-not-harmony" -c /etc/harmony/harmony.conf >/dev/null 2>&1 < /dev/null &
  wait_for 5 bash -c "[[ -n \"\$(2>/dev/null readlink /proc/*/exe | grep -Fx '$T/imposter/definitely-not-harmony')\" ]]" || true
  run_rb /root start
  expect "s_dup/duplicate" 1 "^STOPPED duplicate-process [0-9-]+$"
  [[ "$(systemctl is-active harmony.service)" != "active" ]] \
    && ok "s_dup/legit-stopped" || bad "s_dup/legit-stopped" "$(systemctl is-active harmony.service)"
  [[ -f /etc/systemd/system/harmony.service.d/99-harmony-recovery-hold.conf ]] \
    && ok "s_dup/held" || bad "s_dup/held"
  kill_fakes
}

s_stop_failure() {
  # Quarantine must prove inactivity: with systemctl stop neutered (a PATH
  # shim swallows "stop"), the unhealthy path must report stop-failed - not
  # unhealthy - because the unit is still active.
  new_systemd_case; mk_script
  sed -i 's/^STOP_TIMEOUT=.*/STOP_TIMEOUT=6/' "$RB"
  run_rb /root prepare
  expect "s_stopfail/ready" 0 "^READY " || return 0
  run_rb /root start
  expect "s_stopfail/running" 0 "^RUNNING " || return 0
  mkdir -p "$T/fakectl"
  cat > "$T/fakectl/systemctl" <<'EOF'
#!/usr/bin/env bash
[[ "${1-}" == "stop" ]] && exit 0   # pretend to stop, change nothing
exec /usr/bin/systemctl "$@"
EOF
  chmod 755 "$T/fakectl/systemctl"
  rpc_set 92730100 "0x2222222222222222222222222222222222222222222222222222222222222222"
  set +e
  RB_OUT="$(cd /root && PATH="$T/fakectl:$PATH" bash "$RB" start)"
  RB_RC=$?
  set -e
  [[ $RB_RC -eq 1 && "$RB_OUT" =~ ^STOPPED\ stop-failed\ [0-9-]+$ ]] \
    && ok "s_stopfail/stop-failed" || bad "s_stopfail/stop-failed" "rc=$RB_RC out=$RB_OUT"
  [[ "$(systemctl is-active harmony.service)" == "active" ]] \
    && ok "s_stopfail/unit-still-active-as-simulated" || bad "s_stopfail/unit-still-active-as-simulated" "$(systemctl is-active harmony.service)"
  [[ -f /etc/systemd/system/harmony.service.d/99-harmony-recovery-hold.conf ]] \
    && ok "s_stopfail/hold-installed" || bad "s_stopfail/hold-installed"
  rm -rf "$T/fakectl"
  rpc_set
  systemctl stop harmony.service >/dev/null 2>&1 || true
}

s_failed_live() {
  # systemd "failed" alone is not proof of stop: with SendSIGKILL=no and a
  # SIGTERM-ignoring survivor migrated into the unit cgroup, systemctl stop
  # times out and leaves the unit failed WITH a live process in its cgroup.
  # Quarantine must refuse to treat that as stopped and die stop-failed.
  new_systemd_case; mk_script
  sed -i 's/^STOP_TIMEOUT=.*/STOP_TIMEOUT=6/' "$RB"
  run_rb /root prepare
  expect "s_failedlive/ready" 0 "^READY " || return 0
  run_rb /root start
  expect "s_failedlive/running" 0 "^RUNNING " || return 0
  cat > /etc/systemd/system/harmony.service.d/98-test-stop.conf <<'EOF'
[Service]
SendSIGKILL=no
TimeoutStopSec=2
EOF
  systemctl daemon-reload
  setsid bash -c 'trap "" TERM; sleep 300' > /dev/null 2>&1 < /dev/null &
  local surv=$! cg
  cg="$(systemctl show -p ControlGroup --value harmony.service)"
  if [[ -z "$cg" ]] || ! echo "$surv" > "/sys/fs/cgroup${cg}/cgroup.procs" 2>/dev/null; then
    bad "s_failedlive/survivor-migrated" "cg=$cg"
    kill -9 "$surv" 2>/dev/null || true
    return 0
  fi
  ok "s_failedlive/survivor-migrated"
  rpc_set 92730100 "0x2222222222222222222222222222222222222222222222222222222222222222"
  run_rb /root start
  expect "s_failedlive/stop-failed" 1 "^STOPPED stop-failed [0-9-]+$"
  [[ "$(systemctl is-active harmony.service)" == "failed" ]] \
    && ok "s_failedlive/unit-failed-as-simulated" || bad "s_failedlive/unit-failed-as-simulated" "$(systemctl is-active harmony.service)"
  [[ -d "/proc/$surv" ]] \
    && ok "s_failedlive/survivor-alive" || bad "s_failedlive/survivor-alive"
  grep -qx "$surv" "/sys/fs/cgroup${cg}/cgroup.procs" 2>/dev/null \
    && ok "s_failedlive/survivor-in-cgroup" || bad "s_failedlive/survivor-in-cgroup"
  [[ -f /etc/systemd/system/harmony.service.d/99-harmony-recovery-hold.conf ]] \
    && ok "s_failedlive/hold-installed" || bad "s_failedlive/hold-installed"
  kill -9 "$surv" 2>/dev/null || true
  rm -f /etc/systemd/system/harmony.service.d/98-test-stop.conf
  systemctl daemon-reload
  systemctl reset-failed harmony.service 2>/dev/null || true
  rpc_set
}

s_cgroup_query_failure() {
  # Cgroup verification must fail closed: a shim makes every ControlGroup
  # query fail while all other systemctl calls pass through. The stop itself
  # succeeds, but with the cgroup state unknowable the installer must refuse
  # to certify the stop and report stop-failed.
  new_systemd_case; mk_script
  sed -i 's/^STOP_TIMEOUT=.*/STOP_TIMEOUT=6/' "$RB"
  run_rb /root prepare
  expect "s_cgqueryfail/ready" 0 "^READY " || return 0
  run_rb /root start
  expect "s_cgqueryfail/running" 0 "^RUNNING " || return 0
  mkdir -p "$T/fakectl"
  cat > "$T/fakectl/systemctl" <<'EOF'
#!/usr/bin/env bash
[[ "$*" == *ControlGroup* ]] && exit 1   # cgroup queries fail
exec /usr/bin/systemctl "$@"
EOF
  chmod 755 "$T/fakectl/systemctl"
  rpc_set 92730100 "0x2222222222222222222222222222222222222222222222222222222222222222"
  set +e
  RB_OUT="$(cd /root && PATH="$T/fakectl:$PATH" bash "$RB" start)"
  RB_RC=$?
  set -e
  [[ $RB_RC -eq 1 && "$RB_OUT" =~ ^STOPPED\ stop-failed\ [0-9-]+$ ]] \
    && ok "s_cgqueryfail/stop-failed" || bad "s_cgqueryfail/stop-failed" "rc=$RB_RC out=$RB_OUT"
  # the unit really did stop; only the unknowable cgroup state blocked the claim
  [[ "$(systemctl is-active harmony.service)" != "active" ]] \
    && ok "s_cgqueryfail/unit-actually-stopped" || bad "s_cgqueryfail/unit-actually-stopped" "$(systemctl is-active harmony.service)"
  [[ -f /etc/systemd/system/harmony.service.d/99-harmony-recovery-hold.conf ]] \
    && ok "s_cgqueryfail/hold-installed" || bad "s_cgqueryfail/hold-installed"
  rm -rf "$T/fakectl"
  rpc_set
}

s_needs_root() {
  # Rootless invocation while a harmony.service unit is loaded: refused with
  # needs-root before anything is touched.
  new_systemd_case; mk_script
  set +e
  local out rc
  out="$(cd /home/hmytest && runuser -u hmytest -- bash "$RB" prepare)"
  rc=$?
  set -e
  [[ $rc -eq 1 && "$out" =~ ^STOPPED\ needs-root\ [0-9-]+$ ]] \
    && ok "s_needsroot/refused" || bad "s_needsroot/refused" "rc=$rc out=$out"
  [[ "$(systemctl is-active harmony.service)" == "active" ]] \
    && ok "s_needsroot/unit-untouched" || bad "s_needsroot/unit-untouched" "$(systemctl is-active harmony.service)"
  rm -rf /home/hmytest/.hmy-recovery-92730034
}

s_cgroup_refusal() {
  new_systemd_case
  systemctl stop harmony.service >/dev/null 2>&1 || true
  mk_script
  local inv="$T/cases/cgroup"
  rm -rf "$inv"; mkdir -p "$inv/data/harmony_db_0"
  printf 'x' > "$inv/data/harmony_db_0/CURRENT"
  cp "$T/fake-orig" "$inv/harmony"
  cat > "$inv/harmony.conf" <<EOF
Version = "2.6.2"

[General]
NodeType = "validator"
IsArchival = false
DataDir = "$inv/data"

[Network]
NetworkType = "mainnet"

[ShardData]
EnableShardData = false
EOF
  systemd-run --unit=rbsmoke-fake --property=WorkingDirectory="$inv" "$inv/harmony" -c "$inv/harmony.conf" >/dev/null 2>&1
  wait_for 5 bash -c "[[ -n \"\$(2>/dev/null readlink /proc/*/exe | grep -Fx '$inv/harmony')\" ]]" || true
  RB_SERVICE="$ABSENT_UNIT" run_rb "$inv" prepare
  expect "s_cgroup/refused" 1 "^STOPPED unsupported-layout [0-9-]+$"
  systemctl stop rbsmoke-fake.service >/dev/null 2>&1 || true
  systemctl reset-failed rbsmoke-fake.service >/dev/null 2>&1 || true
}

run_systemd() {
  if ! have_systemd; then
    skip "systemd-cases" "PID 1 is not systemd here; run this group on the systemd VM/container"
    return 0
  fi
  s_multiple_units || true
  s_happy || true
  s_predeleted_db || true
  s_conflicting_dropin || true
  s_leftover_hold || true
  s_unhealthy_post || true
  s_latch_active || true
  s_failed_start || true
  s_tampered_exec || true
  s_started_restart_failure || true
  s_started_duplicate || true
  s_stop_failure || true
  s_failed_live || true
  s_cgroup_query_failure || true
  s_needs_root || true
  s_cgroup_refusal || true
  cleanup_case
}

# ---------- reboot split ----------

pre_reboot() {
  cleanup_case
  if have_systemd; then
    new_systemd_case; mk_script
    run_rb /root prepare
    expect "pre-reboot/systemd-ready" 0 "^READY $BLS_SORTED recovery-92730034$"
  else
    new_manual_case reboot; mk_script; start_orig
    RB_SERVICE="$ABSENT_UNIT" run_rb "$INV" prepare
    expect "pre-reboot/manual-ready" 0 "^READY $BLS_SORTED recovery-92730034$"
  fi
  echo "PRE-REBOOT DONE - now reboot this box and run: $0 post-reboot"
}

post_reboot() {
  if have_systemd; then
    [[ "$(systemctl is-active harmony.service)" != "active" ]] \
      && ok "post-reboot/unit-stayed-stopped" || bad "post-reboot/unit-stayed-stopped" "$(systemctl is-active harmony.service)"
    [[ -f /etc/systemd/system/harmony.service.d/99-harmony-recovery-hold.conf ]] \
      && ok "post-reboot/hold-survived" || bad "post-reboot/hold-survived"
    run_rb /root start
    expect "post-reboot/systemd-start" 0 "^RUNNING $BLS_SORTED recovery-92730034$"
  else
    INV="$T/cases/reboot"; DATA="$INV/data"
    [[ -z "$(pids_by_exe "$INV/harmony")" && -z "$(pids_by_exe "$BIN")" ]] \
      && ok "post-reboot/manual-stayed-stopped" || bad "post-reboot/manual-stayed-stopped"
    RB_SERVICE="$ABSENT_UNIT" run_rb "$INV" start
    expect "post-reboot/manual-start" 0 "^RUNNING $BLS_SORTED recovery-92730034$"
  fi
}

# ---------- entry ----------

[[ "$(id -u)" == "0" ]] || { echo "must run as root on a disposable box"; exit 1; }
[[ "$(uname -s)" == "Linux" ]] || { echo "Linux only"; exit 1; }
for tool in cc python3 curl rclone sha256sum flock stat df du awk sed grep jq od find pgrep fuser install getent readlink sync runuser setsid; do
  command -v "$tool" >/dev/null 2>&1 || { echo "missing tool: $tool"; exit 1; }
done
[[ -f "$SRC" ]] || { echo "installer script not found at $SRC"; exit 1; }

GROUP="${1-all}"
build_fixtures
case "$GROUP" in
  manual)      run_manual ;;
  systemd)     run_systemd ;;
  all)         run_manual; run_systemd ;;
  pre-reboot)  pre_reboot ;;
  post-reboot) post_reboot ;;
  *) echo "usage: $0 [manual|systemd|pre-reboot|post-reboot|all]"; exit 2 ;;
esac

echo "----------------------------------------"
echo "PASS=$PASS FAIL=$FAIL${FAILED:+ failed:$FAILED}"
[[ $FAIL -eq 0 ]]
