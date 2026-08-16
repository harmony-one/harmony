#!/usr/bin/env bash
# rollback-92730034.sh - Harmony shard-0 clean-DB installer (emergency recovery).
#
# "rollback" is a label only: this script copies the frozen clean shard-0
# database ending at block 92,730,034 (a raw LevelDB directory transferred
# with rclone from a pinned read-only source) plus the pinned official
# v2026.1.2 harmony binary, and keeps the node stopped until the coordinated
# GO. It reverts nothing and never restores the old DB automatically.
# Requires rclone in addition to standard tools.
#
#   sudo bash ./rollback-92730034.sh prepare [--discard-old-db]
#   sudo bash ./rollback-92730034.sh start
#
# Rootless manual mode: a manual-directory validator whose harmony binary,
# config, and database are owned by a non-root user may run both commands AS
# THAT USER without sudo (state and staged artifacts then live under
# ./.hmy-recovery-92730034/work in the invocation directory, owned by that
# user). systemd layouts always require root.
#
# Supported platforms: Linux x86_64 and Linux aarch64 (e.g. Raspberry Pi 5);
# the matching pinned recovery-binary artifact is selected automatically.
#
# Exactly one line is printed to stdout:
#   READY <bls-ids> recovery-92730034      (prepare succeeded; node stopped)
#   RUNNING <bls-ids> recovery-92730034    (start succeeded; node healthy)
#   STOPPED <reason> <log-id>              (see private run log for detail)
#
# Supported layouts: packaged harmony.service systemd, or manual-directory
# (exactly one directly launched Harmony process anchored by the invocation
# directory). Manual-directory validators MUST run both commands from the
# directory containing their harmony binary or harmony config file, as the
# same user both times, MUST disable any cron/boot/supervisor autostart before
# `prepare`, and MUST leave the node stopped until GO. Keep this script file
# on a persistent filesystem: the same file runs both commands, possibly
# across a reboot.
#
# This script is standalone: it is never installed, copied, or chmod-ed
# anywhere. Run it with `bash` from where it was downloaded.
#
# The whole body is one brace group closing at end-of-file: a partially
# downloaded copy is a parse error and executes nothing.
{

set -euo pipefail
shopt -s nullglob
umask 077

# --- BEGIN FREEZE CONSTANTS (filled once at freeze; leave structure intact) ---
SCRIPT_URL="https://raw.githubusercontent.com/harmony-one/harmony/refs/heads/rollback-92730034/scripts/rollback-92730034.sh"
# Public read-only shard-0 SnapDB source documented by Harmony (docs.harmony.one,
# "Shard 0 validator Snap DB sync"), accepted by the team as the clean DB for
# block 92,730,034. Self-contained: needs no local rclone config. The WebDAV
# service exposes no file hashes, so integrity relies on the pinned count and
# bytes below, the LevelDB structure check, and the post-start RPC check of
# the block-92,730,034 hash.
DB_RCLONE_SOURCE=":webdav,url='http://snapdb.s0.t.hmny.io/webdav',vendor=other,user=snap,pass=ufbTDtK0fENuutwuDOHae57xT8URsZVIcdotK30T5A:"
DB_FILE_COUNT=184510             # exact file count from rclone size --json
DB_BYTES=371422947984            # exact total bytes from rclone size --json
NODE_BIN_URL_AMD64="https://github.com/harmony-one/harmony/releases/download/v2026.1.2/harmony-amd64"
NODE_BIN_SHA256_AMD64="a01314f8fb7a279fffad48ced03e2615f5b28b8c9afb7831700ff83e7f6506df"
NODE_BIN_URL_ARM64="https://github.com/harmony-one/harmony/releases/download/v2026.1.2/harmony-arm64"
NODE_BIN_SHA256_ARM64="3399cc969fa02b43215b90f6ced7f9e98250d2a9242d0e11410e92bd09ddc0ad"
# --- END FREEZE CONSTANTS ---

# Fixed recovery target (from the 2026-08-13 emergency handoff).
TARGET_HEIGHT=92730034
TARGET_HASH="0x30c35d2f2291e4b27debe7862956cf7a0cc7abefc044273d6823567335086d8d"

# Tunables.
START_ACTIVE_TIMEOUT=30          # seconds for the process/unit to come up
START_RPC_TIMEOUT=180            # seconds for loopback RPC health
STOP_TIMEOUT=120                 # seconds for a clean stop
OBSERVE_SECS=15                  # manual-layout respawn observation window
MARGIN_MIN_BYTES=21474836480     # disk margin floor: 20 GiB
MARGIN_MIN_DISCARD_BYTES=10737418240  # relaxed floor with --discard-old-db: 10 GiB
RPC_URL="http://127.0.0.1:9500"

# Fixed paths (never anything under /usr/local). Rootless manual mode moves
# WORK (and everything derived from it) under the invocation directory; see
# main(). The sentinel is systemd-only and therefore always root-owned.
WORK=/var/lib/harmony-recovery-92730034
BIN="$WORK/bin/harmony-recovery"
PRIV="$WORK/private"
STATE_FILE="$PRIV/state"
LOCK_FILE="$PRIV/lock"
SENTINEL_DIR=/run/harmony-recovery-92730034
SENTINEL="$SENTINEL_DIR/GO"
STAGING_NAME=".hmy-recovery-92730034"
HOLD_DROPIN_NAME="99-harmony-recovery-hold.conf"
EXEC_DROPIN_NAME="50-harmony-recovery-exec.conf"
SUFFIX="recovery-92730034"

MODE=""
DISCARD_FLAG=0
QUIET=0
LOGID=""
PRINTED=0
INVOCATION_DIR=""
STAMP=""
declare -A S=()   # state file key/value store

usage_exit() {
  # Usage errors touch nothing and print the one line themselves.
  printf 'usage: [sudo] bash ./rollback-92730034.sh prepare [--discard-old-db] [--quiet] | start   (sudo required for systemd validators; rootless manual validators run as the node user)\n'
  exit 2
}

log() {
  local line
  printf -v line '[%s] %s' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$*"
  printf '%s\n' "$line"       # detailed run log
  printf '%s\n' "$line" >&4   # live operator progress on stderr
}

emit() { printf '%s\n' "$*" >&3; PRINTED=1; }

die() { # die <reason> [detail...]
  local reason="$1"; shift || true
  log "FAILURE reason=$reason detail: $*"
  emit "STOPPED $reason $LOGID"
  exit 1
}

notice() {
  # Plain operator instructions, copied to both the run log and live stderr.
  printf '%s\n' "$*"
  printf '%s\n' "$*" >&4
}

confirm_db_deletion() { # <absolute-db-path>
  local path="$1" answer=""
  (( QUIET )) && return 0
  notice ""
  notice "The validator is stopped. The script is ready to permanently delete:"
  notice "  $path"
  notice "This cannot be undone. Type y and press Enter to continue; any other answer cancels."
  printf 'Delete %s? [y/N] ' "$path" >&4
  IFS= read -r answer || true
  printf '\n' >&4
  [[ "$answer" == "y" || "$answer" == "Y" ]] \
    || die deletion-cancelled "operator did not confirm deletion of $path"
}

package_for_tool() { # <tool> <apt|dnf|pacman>
  local tool="$1" family="$2"
  case "$tool" in
    curl|rclone|jq|sed|grep) printf '%s\n' "$tool" ;;
    systemctl) printf '%s\n' systemd ;;
    find) printf '%s\n' findutils ;;
    sha256sum|stat|df|du|od|cat|install|readlink|sync|tee) printf '%s\n' coreutils ;;
    flock|setsid|runuser) printf '%s\n' util-linux ;;
    awk) printf '%s\n' gawk ;;
    pgrep) [[ "$family" == apt ]] && printf '%s\n' procps || printf '%s\n' procps-ng ;;
    fuser) printf '%s\n' psmisc ;;
    getent)
      case "$family" in
        apt) printf '%s\n' libc-bin ;;
        dnf) printf '%s\n' glibc-common ;;
        pacman) printf '%s\n' glibc ;;
      esac
      ;;
    *) printf '%s\n' "$tool" ;;
  esac
}

require_tools() {
  local tool family="" manager="" package
  local missing=() packages=()
  local -A seen=()

  for tool in "$@"; do
    command -v "$tool" >/dev/null 2>&1 || missing+=("$tool")
  done
  (( ${#missing[@]} > 0 )) || return 0

  notice ""
  notice "Missing required commands: ${missing[*]}"
  if command -v apt-get >/dev/null 2>&1; then
    family=apt; manager="apt-get"
  elif command -v dnf >/dev/null 2>&1; then
    family=dnf; manager="dnf"
  elif command -v yum >/dev/null 2>&1; then
    family=dnf; manager="yum"
  elif command -v pacman >/dev/null 2>&1; then
    family=pacman; manager="pacman"
  fi

  if [[ -n "$family" ]]; then
    for tool in "${missing[@]}"; do
      package="$(package_for_tool "$tool" "$family")"
      if [[ -n "$package" && -z "${seen[$package]+x}" ]]; then
        seen["$package"]=1
        packages+=("$package")
      fi
    done
    notice "Install the missing packages, then run the same recovery command again:"
    case "$manager" in
      apt-get)
        notice "  sudo apt-get update && sudo apt-get install -y ${packages[*]}"
        ;;
      dnf|yum)
        notice "  sudo $manager install -y ${packages[*]}"
        ;;
      pacman)
        notice "  sudo pacman -S --needed ${packages[*]}"
        ;;
    esac
  else
    notice "Install the commands listed above with this system's package manager, then run the same recovery command again."
  fi
  notice ""
  die missing-dependencies "missing commands: ${missing[*]}"
}

on_exit() {
  local rc=$?
  if [[ $rc -ne 0 && $PRINTED -eq 0 ]]; then
    printf 'STOPPED cannot-determine-state %s\n' "$LOGID" >&3
  fi
}

# fsync a file or directory (GNU coreutils sync accepts path arguments).
sync_path() { sync "$1"; }

# ---------- state file (root-owned, atomic, fsynced) ----------

load_state() {
  [[ -f "$STATE_FILE" ]] || return 1
  local line
  while IFS= read -r line; do
    [[ -z "$line" ]] && continue
    if [[ "$line" =~ ^([A-Z_][A-Z0-9_]*)=(.*)$ ]]; then
      S["${BASH_REMATCH[1]}"]="${BASH_REMATCH[2]}"
    else
      die cannot-determine-state "malformed state line: $line"
    fi
  done < "$STATE_FILE"
  return 0
}

save_state() {
  local tmp="$STATE_FILE.tmp.$$" k
  : > "$tmp"
  chmod 600 "$tmp"
  for k in $(printf '%s\n' "${!S[@]}" | sort); do
    printf '%s=%s\n' "$k" "${S[$k]}" >> "$tmp"
  done
  sync_path "$tmp"
  mv -f "$tmp" "$STATE_FILE"
  sync_path "$PRIV"
}

set_state() { S[STATE]="$1"; save_state; log "state -> $1"; }

state_rank() {
  case "${1-}" in
    PREPARED) echo 1 ;; SWAP_BEGUN) echo 2 ;; OLD_RENAMED) echo 3 ;;
    NEW_INSTALLED) echo 4 ;; DELETING) echo 5 ;; READY) echo 6 ;;
    STARTING) echo 7 ;; STARTED) echo 8 ;; *) echo 0 ;;
  esac
}

# ---------- small helpers ----------

path_sane() { [[ "$1" =~ ^/[A-Za-z0-9._/-]+$ ]]; }

resolve_against() { # <base> <path> -> canonical absolute path (must exist)
  local p="$2"
  [[ "$p" == /* ]] || p="$1/$p"
  readlink -f -- "$p"
}

toml_get() { # <file> <section> <key> -> value (quotes stripped), empty if absent
  awk -v sec="$2" -v key="$3" '
    /^[ \t]*\[/ {
      s = $0; gsub(/^[ \t]*\[|\][ \t\r]*$/, "", s)
      insec = (s == sec); next
    }
    insec {
      line = $0; sub(/#.*/, "", line)
      if (match(line, "^[ \t]*" key "[ \t]*=")) {
        v = substr(line, RSTART + RLENGTH)
        gsub(/^[ \t]+|[ \t\r]+$/, "", v); gsub(/^"|"$/, "", v)
        print v; exit
      }
    }' "$1"
}

rpc_call() { # <method> <params-json> -> raw response body (empty on failure)
  curl -sS -m 10 -H 'Content-Type: application/json' \
    -d "{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"$1\",\"params\":$2}" \
    "$RPC_URL" 2>/dev/null || true
}

rpc_blskeys() { # -> sorted comma-joined blskey list (empty on failure)
  rpc_call hmyv2_getNodeMetadata '[]' | jq -r '.result.blskey | sort | join(",")' 2>/dev/null || true
}

# Parse harmony argv tokens (after argv[0]). Sets PARSED_CONFIG,
# PARSED_DATADIR, PARSED_AGGREGATE_SIG, and PARSED_EXTRA. The aggregate-sig
# flag is the one additional original flag preserved by the recovery launch.
parse_harmony_args() {
  PARSED_CONFIG=""; PARSED_DATADIR=""; PARSED_AGGREGATE_SIG=""; PARSED_EXTRA=""
  local args=("$@") i=0 n=$#
  while (( i < n )); do
    case "${args[$i]}" in
      -c|--config)
        (( i + 1 < n )) || { PARSED_EXTRA="${args[$i]}"; return 0; }
        PARSED_CONFIG="${args[$((i+1))]}"; i=$((i+2)) ;;
      --config=*) PARSED_CONFIG="${args[$i]#--config=}"; i=$((i+1)) ;;
      --datadir)
        (( i + 1 < n )) || { PARSED_EXTRA="${args[$i]}"; return 0; }
        PARSED_DATADIR="${args[$((i+1))]}"; i=$((i+2)) ;;
      --datadir=*) PARSED_DATADIR="${args[$i]#--datadir=}"; i=$((i+1)) ;;
      --consensus.aggregate-sig=*)
        PARSED_AGGREGATE_SIG="${args[$i]#--consensus.aggregate-sig=}"
        [[ "$PARSED_AGGREGATE_SIG" == "true" || "$PARSED_AGGREGATE_SIG" == "false" ]] \
          || PARSED_EXTRA="${args[$i]}"
        i=$((i+1)) ;;
      *) PARSED_EXTRA="${args[$i]}"; return 0 ;;
    esac
  done
}

# ---------- duplicate-signer scan (/proc based) ----------
# Flags any process whose exe path or exe SHA-256 matches the original or the
# staged recovery binary (including renamed/copied/deleted variants), whose
# cmdline references DATADIR or CONFIG, or whose open fds point under
# <DATADIR>/harmony_db_0. $1 = legitimate PID to exclude ("" when the node is
# supposed to be down). The legitimate PID's ancestor chain is excluded too:
# the runuser/setsid launcher we started stays alive as the node's parent and
# carries the node's own arguments on its command line.
scan_duplicates() {
  local exclude="${1-}" d pid exe tgt hit cmd h f t a
  local -A hcache=() excl=()
  if [[ -n "$exclude" ]]; then
    a="$exclude"
    while [[ "$a" =~ ^[0-9]+$ && "$a" != "0" && "$a" != "1" && -z "${excl[$a]+x}" ]]; do
      excl[$a]=1
      a="$(awk '/^PPid:/{print $2}' "/proc/$a/status" 2>/dev/null || true)"
    done
  fi
  DUP_HITS=""
  set +x
  for d in /proc/[0-9]*; do
    pid="${d#/proc/}"
    [[ "$pid" == "$$" || "$pid" == "$PPID" ]] && continue
    [[ -n "${excl[$pid]+x}" ]] && continue
    [[ -d "$d" ]] || continue
    hit=""
    exe="$(readlink "$d/exe" 2>/dev/null || true)"
    tgt="${exe% (deleted)}"
    if [[ -n "$tgt" ]]; then
      if [[ "$tgt" == "$ORIG_EXE" || "$tgt" == "$BIN" ]]; then
        hit="exe-path:$tgt"
      else
        if [[ -z "${hcache[$tgt]+x}" ]]; then
          h="$(sha256sum "$d/exe" 2>/dev/null | awk '{print $1}' || true)"
          hcache[$tgt]="$h"
        fi
        h="${hcache[$tgt]}"
        if [[ -n "$h" && ( "$h" == "$ORIG_EXE_SHA256" || "$h" == "$NODE_BIN_SHA256" ) ]]; then
          hit="exe-sha256:$tgt"
        fi
      fi
    fi
    if [[ -z "$hit" ]]; then
      cmd="$(tr '\0' ' ' < "$d/cmdline" 2>/dev/null || true)"
      if [[ -n "$cmd" && ( "$cmd" == *"$DATADIR"* || "$cmd" == *"$CONFIG"* ) ]]; then
        hit="cmdline"
      fi
    fi
    if [[ -z "$hit" ]]; then
      for f in "$d"/fd/*; do
        t="$(readlink "$f" 2>/dev/null || true)"
        if [[ "$t" == "$DATADIR/harmony_db_0"* ]]; then hit="fd"; break; fi
      done
    fi
    if [[ -n "$hit" ]]; then
      DUP_HITS+="pid=$pid($hit) "
    fi
  done
  set -x
  [[ -z "$DUP_HITS" ]]
}

require_no_duplicates() { # $1 = excluded legitimate pid or ""
  scan_duplicates "${1-}" || die duplicate-process "hits: $DUP_HITS"
}

# ---------- manual-directory process discovery ----------
# A candidate is a process whose resolved exe parent dir is INVOCATION_DIR, or
# whose -c/--config (resolved against its cwd) has parent dir INVOCATION_DIR.
manual_candidates() {
  CAND_PIDS=()
  local d pid exe tgt cwd cfg
  set +x
  for d in /proc/[0-9]*; do
    pid="${d#/proc/}"
    [[ "$pid" == "$$" || "$pid" == "$PPID" ]] && continue
    [[ -r "$d/cmdline" ]] || continue
    local argv=()
    mapfile -d '' -t argv < "$d/cmdline" 2>/dev/null || true
    (( ${#argv[@]} )) || continue
    exe="$(readlink "$d/exe" 2>/dev/null || true)"
    tgt="${exe% (deleted)}"
    if [[ -n "$tgt" && "$(dirname -- "$tgt")" == "$INVOCATION_DIR" ]]; then
      CAND_PIDS+=("$pid"); continue
    fi
    parse_harmony_args "${argv[@]:1}"
    if [[ -n "$PARSED_CONFIG" ]]; then
      cwd="$(readlink "$d/cwd" 2>/dev/null || true)"
      [[ -n "$cwd" ]] || continue
      cfg="$(resolve_against "$cwd" "$PARSED_CONFIG" 2>/dev/null || true)"
      if [[ -n "$cfg" && "$(dirname -- "$cfg")" == "$INVOCATION_DIR" ]]; then
        CAND_PIDS+=("$pid")
      fi
    fi
  done
  set -x
}

pid_matches_orig() { # is <pid> plausibly the recorded original harmony process
  local d="/proc/$1" exe tgt cmd
  [[ -d "$d" ]] || return 1
  exe="$(readlink "$d/exe" 2>/dev/null || true)"; tgt="${exe% (deleted)}"
  [[ "$tgt" == "$ORIG_EXE" ]] && return 0
  cmd="$(tr '\0' ' ' < "$d/cmdline" 2>/dev/null || true)"
  [[ -n "$cmd" && ( "$cmd" == *"$CONFIG"* || "$cmd" == *"$DATADIR"* ) ]]
}

find_recovery_pids() { # processes running the staged binary with our CONFIG
  RECOVERY_PIDS=()
  local d pid exe tgt cmd
  set +x
  for d in /proc/[0-9]*; do
    pid="${d#/proc/}"
    exe="$(readlink "$d/exe" 2>/dev/null || true)"; tgt="${exe% (deleted)}"
    [[ "$tgt" == "$BIN" ]] || continue
    cmd="$(tr '\0' ' ' < "$d/cmdline" 2>/dev/null || true)"
    [[ "$cmd" == *"$CONFIG"* ]] && RECOVERY_PIDS+=("$pid")
  done
  set -x
}

find_orig_exe_pids() { # processes whose exe is the recorded original binary
  ORIG_EXE_PIDS=()
  local d pid exe tgt
  set +x
  for d in /proc/[0-9]*; do
    pid="${d#/proc/}"
    exe="$(readlink "$d/exe" 2>/dev/null || true)"; tgt="${exe% (deleted)}"
    [[ -n "$ORIG_EXE" && "$tgt" == "$ORIG_EXE" ]] && ORIG_EXE_PIDS+=("$pid")
  done
  set -x
}

kill_pid_proven() { # <pid>: TERM, then KILL; success only when the pid is gone
  kill -TERM "$1" 2>/dev/null || true
  local deadline=$(( SECONDS + 10 ))
  while (( SECONDS < deadline )) && [[ -d "/proc/$1" ]]; do sleep 1; done
  if [[ -d "/proc/$1" ]]; then
    kill -KILL "$1" 2>/dev/null || true
    deadline=$(( SECONDS + 5 ))
    while (( SECONDS < deadline )) && [[ -d "/proc/$1" ]]; do sleep 1; done
  fi
  [[ ! -d "/proc/$1" ]]
}

# ---------- systemd helpers ----------

unit_active_state() { systemctl is-active "$UNIT" 2>/dev/null || true; }

# A unit can be inactive/failed (e.g. stop timeout with SendSIGKILL=no) while
# live processes remain in its cgroup, so state alone is not proof of stop.
# Fail closed: any inability to determine the cgroup path or read its process
# list counts as "not proven empty", never as empty. Only a genuinely absent
# cgroup (successful query returning none, or the directory already removed)
# counts as empty.
unit_cgroup_empty() {
  local cg dir procs
  cg="$(systemctl show -p ControlGroup --value "$UNIT" 2>/dev/null)" || return 1
  [[ -z "$cg" ]] && return 0     # unit has no cgroup at all
  [[ "$cg" == /* ]] || return 1  # unparseable answer: unknown
  for dir in "/sys/fs/cgroup${cg}" "/sys/fs/cgroup/systemd${cg}"; do
    [[ -d "$dir" ]] || continue
    procs="$(cat "$dir/cgroup.procs" 2>/dev/null)" || return 1  # unreadable: unknown
    [[ -z "$procs" ]]
    return
  done
  return 0   # cgroup directory gone: nothing can be left in it
}

unit_is_stopped() {
  local st; st="$(unit_active_state)"
  [[ "$st" == "inactive" || "$st" == "failed" ]] && unit_cgroup_empty
}

wait_unit_stopped() {
  local deadline=$(( SECONDS + STOP_TIMEOUT ))
  while (( SECONDS < deadline )); do
    unit_is_stopped && return 0
    sleep 2
  done
  return 1
}

write_dropin() { # <filename> <content>
  local dir="/etc/systemd/system/$UNIT.d" tmp
  mkdir -p "$dir"
  tmp="$dir/.$1.tmp.$$"
  printf '%s\n' "$2" > "$tmp"
  chmod 644 "$tmp"
  sync_path "$tmp"
  mv -f "$tmp" "$dir/$1"
  sync_path "$dir"
}

hold_dropin_path() { printf '/etc/systemd/system/%s.d/%s' "$UNIT" "$HOLD_DROPIN_NAME"; }
exec_dropin_path() { printf '/etc/systemd/system/%s.d/%s' "$UNIT" "$EXEC_DROPIN_NAME"; }

install_hold() {
  write_dropin "$HOLD_DROPIN_NAME" "[Unit]
ConditionPathExists=$SENTINEL"
  systemctl daemon-reload
}

remove_hold() {
  if [[ -e "$(hold_dropin_path)" ]]; then
    rm -f "$(hold_dropin_path)"
    sync_path "/etc/systemd/system/$UNIT.d"
    systemctl daemon-reload
  fi
}

# The staged binary is the ordinary official v2026.1.2 build, launched with
# the supported original config/datadir shape (no forced sync/DNS/RPC flags),
# so normal config validation applies. Health is judged afterwards over the
# node's own loopback RPC, which prepare already required to be reachable.
recovery_exec_line() {
  printf '%s -c %s --datadir %s' "$BIN" "$CONFIG" "$DATADIR"
  if [[ -n "$CONSENSUS_AGGREGATE_SIG" ]]; then
    printf ' --consensus.aggregate-sig=%s' "$CONSENSUS_AGGREGATE_SIG"
  fi
}

install_exec_dropin() {
  write_dropin "$EXEC_DROPIN_NAME" "[Service]
ExecStart=
ExecStart=$(recovery_exec_line)"
  systemctl daemon-reload
}

# Verify the unit's effective ExecStart is exactly the staged recovery command.
verify_effective_exec() {
  local raw n argv path
  raw="$(systemctl show "$UNIT" -p ExecStart --value)"
  n="$(grep -o 'argv\[\]=' <<< "$raw" | wc -l | tr -d ' ')"
  [[ "$n" == "1" ]] || { log "effective ExecStart has $n commands: $raw"; return 1; }
  path="$(sed -n 's/.*{ path=\([^ ;]*\) .*/\1/p' <<< "$raw")"
  argv="$(sed -n 's/.*argv\[\]=\(.*\) ; \(ignore_errors\|flags\)=.*/\1/p' <<< "$raw")"
  [[ "$path" == "$BIN" && "$argv" == "$(recovery_exec_line)" ]] \
    || { log "effective ExecStart mismatch: path=$path argv=$argv"; return 1; }
}

systemd_service_user() {
  local u; u="$(systemctl show "$UNIT" -p User --value)"
  printf '%s' "${u:-root}"
}

# ---------- artifact staging ----------

sha_ok() { # <file> <sha>
  local h; h="$(sha256sum "$1" | awk '{print $1}')"
  [[ "$h" == "$2" ]]
}

elf_arch_ok() { # <file>: little-endian 64-bit ELF whose e_machine matches the host
  local hdr
  hdr="$(od -An -tx1 -N20 "$1" 2>/dev/null | tr -d ' \n')"
  [[ "${hdr:0:12}" == "7f454c460201" ]] || return 1   # \x7fELF, ELFCLASS64, LE
  [[ "${hdr:36:4}" == "$ELF_MACHINE" ]]               # full 16-bit e_machine, LE (offset 18-19)
}

# Staged binary present with the pinned hash and the right ELF architecture.
binary_ok() { [[ -f "$BIN" ]] && sha_ok "$BIN" "$NODE_BIN_SHA256" && elf_arch_ok "$BIN"; }

ensure_binary() { # $1 = "download" to allow fetching, anything else verify-only
  if binary_ok; then return 0; fi
  if [[ "${1-}" != "download" ]]; then
    die not-ready "staged recovery binary missing, hash mismatch, or wrong ELF"
  fi
  log "fetching recovery binary from $NODE_BIN_URL"
  local tmp="$WORK/bin/.harmony-recovery.tmp.$$"
  rm -f "$tmp"
  curl -fSL --retry 5 -o "$tmp" "$NODE_BIN_URL" || die download-failed "binary fetch"
  sha_ok "$tmp" "$NODE_BIN_SHA256" || die checksum-mismatch "recovery binary"
  elf_arch_ok "$tmp" || die download-failed "downloaded binary is not a 64-bit linux-$ARCH ELF"
  chmod 0755 "$tmp"
  sync_path "$tmp"
  mv -f "$tmp" "$BIN"
  sync_path "$WORK/bin"
}

# Verify <dir> is exactly the pinned raw LevelDB directory: only regular files
# directly inside it (no subdirectories, symlinks, or special entries), only
# goleveldb filename classes (CURRENT, LOCK, LOG, LOG.old, one
# MANIFEST-<digits>, numeric .ldb/.sst/.log tables and journals), the pinned
# file count and total bytes, and CURRENT naming the one and only MANIFEST,
# which must exist. The WebDAV source publishes no content hashes, so this
# structural check plus the pinned count/bytes plus the post-start RPC pin of
# block 92,730,034 ARE the DB trust controls.
# Fail closed: sets DB_TREE_ERR and returns 1 on any doubt.
db_tree_ok() { # <dir>
  local dir="$1" line ty sz name n=0 bytes=0 cur manifests=0
  DB_TREE_ERR=""
  [[ -d "$dir" && ! -L "$dir" ]] || { DB_TREE_ERR="not a directory: $dir"; return 1; }
  set +x
  while IFS= read -r line; do
    ty="${line%% *}"; line="${line#* }"
    sz="${line%% *}"; name="${line#* }"
    [[ "$ty" == "f" ]] || { DB_TREE_ERR="non-regular entry (type $ty): $name"; set -x; return 1; }
    [[ "$name" =~ ^(CURRENT|LOCK|LOG|LOG\.old|MANIFEST-[0-9]+|[0-9]+\.(ldb|sst|log))$ ]] \
      || { DB_TREE_ERR="unexpected filename: $name"; set -x; return 1; }
    [[ "$name" == MANIFEST-* ]] && manifests=$((manifests+1))
    n=$((n+1)); bytes=$((bytes+sz))
  done < <(find "$dir" -mindepth 1 -printf '%y %s %P\n')
  set -x
  (( n == DB_FILE_COUNT )) || { DB_TREE_ERR="file count $n != pinned $DB_FILE_COUNT"; return 1; }
  (( bytes == DB_BYTES )) || { DB_TREE_ERR="total bytes $bytes != pinned $DB_BYTES"; return 1; }
  (( manifests == 1 )) || { DB_TREE_ERR="expected exactly one MANIFEST, found $manifests"; return 1; }
  [[ -f "$dir/CURRENT" && ! -L "$dir/CURRENT" ]] || { DB_TREE_ERR="missing CURRENT"; return 1; }
  cur="$(tr -d '\n' < "$dir/CURRENT")"
  [[ "$cur" =~ ^MANIFEST-[0-9]+$ ]] || { DB_TREE_ERR="malformed CURRENT: '$cur'"; return 1; }
  [[ -f "$dir/$cur" && ! -L "$dir/$cur" ]] || { DB_TREE_ERR="CURRENT names missing manifest: $cur"; return 1; }
  return 0
}

verify_db_dir() { # <dir> <label>: die unless the tree matches the pins
  db_tree_ok "$1" || die db-verify-failed "$2: $DB_TREE_ERR"
  log "DB tree verified ($2): $1"
}

# Remote source must report exactly the pinned file count and total bytes.
rclone_visible() {
  # Keep stdout available to callers (for example `rclone size --json`), and
  # copy stderr/stats to both the run log and the operator's terminal.
  rclone "$@" --stats=10s --stats-one-line --stats-log-level NOTICE \
    2> >(tee /dev/fd/4 >&2)
}

require_db_source_metrics() { # <phase>
  local json count bytes
  log "checking DB source size ($1); listing $DB_FILE_COUNT files can take about a minute"
  json="$(rclone_visible size --json --config=/dev/null "$DB_RCLONE_SOURCE")" \
    || die download-failed "cannot query DB source metrics ($1)"
  count="$(jq -r '.count // empty' <<< "$json" 2>/dev/null || true)"
  bytes="$(jq -r '.bytes // empty' <<< "$json" 2>/dev/null || true)"
  [[ "$count" == "$DB_FILE_COUNT" && "$bytes" == "$DB_BYTES" ]] \
    || die source-mismatch "remote count=$count bytes=$bytes want count=$DB_FILE_COUNT bytes=$DB_BYTES ($1)"
  log "DB source metrics match pins ($1)"
}

disk_gate() { # one raw-directory copy is staged, then renamed into place
  local free staged margin need pct
  free="$(df -B1 --output=avail "$DATADIR" | tail -1 | tr -d ' ')"
  staged=0
  [[ -d "$STAGING" ]] && staged="$(du -sb "$STAGING" | awk '{print $1}')"
  (( staged > DB_BYTES )) && staged=$DB_BYTES
  if [[ "${S[DISCARD_REQUESTED]-}" == "1" ]]; then
    pct=$(( DB_BYTES / 20 )); margin=$MARGIN_MIN_DISCARD_BYTES
  else
    pct=$(( DB_BYTES / 10 )); margin=$MARGIN_MIN_BYTES
  fi
  (( pct > margin )) && margin=$pct
  need=$(( DB_BYTES - staged + margin ))
  log "disk gate: free=$free need=$need (staged=$staged margin=$margin)"
  (( free >= need )) || die low-disk "free=$free need=$need"
}

# Sync the frozen DB directory into the staging area only (never the live
# DataDir DB). rclone reruns resume per file; a valid already-staged tree
# skips the network entirely. The remote metrics are checked before and after
# the transfer, and the staged tree must then pass the structural pin checks.
ensure_db_staged() {
  local dst="$STAGING/db/harmony_db_0"
  mkdir -p "$STAGING/db"
  if db_tree_ok "$dst"; then
    log "clean DB already staged and verified; skipping transfer"
    return 0
  fi
  log "staged DB not usable yet ($DB_TREE_ERR); starting rclone transfer"
  require_db_source_metrics pre-transfer
  log "downloading the clean DB; rclone will report bytes, speed, percentage, and ETA every 10 seconds"
  rclone_visible sync --config=/dev/null --retries 5 "$DB_RCLONE_SOURCE" "$dst" \
    || die download-failed "rclone sync into staging"
  require_db_source_metrics post-transfer
  verify_db_dir "$dst" "staged clean DB"
}

# ---------- layout discovery (prepare, fresh run only) ----------

discover_layout() {
  local unit="${SERVICE:-harmony.service}"
  [[ ! -d /run/systemd/system ]] || require_tools systemctl
  if command -v systemctl >/dev/null 2>&1 \
     && [[ "$(systemctl show "$unit" -p LoadState --value 2>/dev/null)" == "loaded" ]]; then
    (( ROOTLESS )) && die needs-root "unit $unit is loaded; systemd layouts require sudo/root"
    UNIT="$unit"
    discover_systemd
  else
    require_tools setsid
    if (( ! ROOTLESS )); then
      require_tools runuser
    fi
    discover_manual
  fi
  validate_common
}

discover_systemd() {
  LAYOUT=systemd
  local raw n path argv wd
  raw="$(systemctl show "$UNIT" -p ExecStart --value)"
  n="$(grep -o 'argv\[\]=' <<< "$raw" | wc -l | tr -d ' ')"
  [[ "$n" == "1" ]] || die unsupported-layout "ExecStart has $n commands"
  path="$(sed -n 's/.*{ path=\([^ ;]*\) .*/\1/p' <<< "$raw")"
  argv="$(sed -n 's/.*argv\[\]=\(.*\) ; \(ignore_errors\|flags\)=.*/\1/p' <<< "$raw")"
  [[ -n "$path" && -n "$argv" ]] || die unsupported-layout "cannot parse ExecStart: $raw"
  local tokens=()
  read -r -a tokens <<< "$argv"
  parse_harmony_args "${tokens[@]:1}"
  [[ -z "$PARSED_EXTRA" ]] || die unsupported-layout "unsupported ExecStart flag: $PARSED_EXTRA"
  CONSENSUS_AGGREGATE_SIG="$PARSED_AGGREGATE_SIG"
  [[ -n "$PARSED_CONFIG" ]] || die unsupported-layout "no -c/--config in ExecStart"
  wd="$(systemctl show "$UNIT" -p WorkingDirectory --value)"
  [[ "$wd" == "~" || -z "$wd" ]] && wd="$(getent passwd "$(systemd_service_user)" | cut -d: -f6)"
  [[ -n "$wd" ]] || die unsupported-layout "cannot resolve WorkingDirectory"
  ORIG_EXE="$path"
  CONFIG="$(resolve_against "$wd" "$PARSED_CONFIG" || true)"
  [[ -n "$CONFIG" && -f "$CONFIG" ]] || die unsupported-layout "config not found: $PARSED_CONFIG"
  local dd
  if [[ -n "$PARSED_DATADIR" ]]; then
    dd="$PARSED_DATADIR"
  else
    dd="$(toml_get "$CONFIG" General DataDir)"
  fi
  [[ -n "$dd" ]] || dd="."
  DATADIR="$(resolve_against "$wd" "$dd" || true)"
  [[ -n "$DATADIR" && -d "$DATADIR" ]] || die unsupported-layout "cannot resolve DataDir"
  S[UNIT]="$UNIT"
}

discover_manual() {
  LAYOUT=manual-directory
  manual_candidates
  case "${#CAND_PIDS[@]}" in
    0) die unsupported-layout "no running Harmony process anchored at $INVOCATION_DIR" ;;
    1) : ;;
    *) die unsupported-layout "multiple candidate processes: ${CAND_PIDS[*]}" ;;
  esac
  local pid="${CAND_PIDS[0]}" d exe tgt cgline
  d="/proc/$pid"
  # Refuse membership in any systemd service cgroup (user@N.service, the user
  # manager every login process sits under, is not a supervising service).
  while IFS= read -r cgline; do
    if [[ "$cgline" =~ \.service ]] && ! [[ "$cgline" =~ user@[0-9]+\.service ]]; then
      die unsupported-layout "process $pid is in a systemd service cgroup: $cgline"
    fi
  done < "$d/cgroup"
  local argv=()
  mapfile -d '' -t argv < "$d/cmdline" || die unsupported-layout "process $pid vanished during discovery"
  parse_harmony_args "${argv[@]:1}"
  [[ -z "$PARSED_EXTRA" ]] || die unsupported-layout "unsupported original flag: $PARSED_EXTRA"
  CONSENSUS_AGGREGATE_SIG="$PARSED_AGGREGATE_SIG"
  [[ -n "$PARSED_CONFIG" ]] || die unsupported-layout "no -c/--config on original command line"
  exe="$(readlink "$d/exe" 2>/dev/null || true)"; tgt="${exe% (deleted)}"
  [[ -n "$tgt" && "$tgt" != *" (deleted)"* && -f "$tgt" ]] || die unsupported-layout "cannot resolve process executable"
  RUN_CWD="$(readlink "$d/cwd" 2>/dev/null || true)"
  [[ -n "$RUN_CWD" && -d "$RUN_CWD" ]] || die unsupported-layout "cannot resolve process cwd"
  local uid
  uid="$(stat -c %u "$d")"
  RUN_USER="$(getent passwd "$uid" | cut -d: -f1)"
  [[ -n "$RUN_USER" ]] || die unsupported-layout "no passwd entry for uid $uid"
  if (( ROOTLESS )) && [[ "$uid" != "$(id -u)" ]]; then
    die needs-root "harmony process is owned by $RUN_USER (uid $uid); rerun as that user or with sudo"
  fi
  ORIG_EXE="$tgt"
  ORIG_PID="$pid"
  CONFIG="$(resolve_against "$RUN_CWD" "$PARSED_CONFIG" || true)"
  [[ -n "$CONFIG" && -f "$CONFIG" ]] || die unsupported-layout "config not found: $PARSED_CONFIG"
  local dd
  if [[ -n "$PARSED_DATADIR" ]]; then
    dd="$PARSED_DATADIR"
  else
    dd="$(toml_get "$CONFIG" General DataDir)"
  fi
  [[ -n "$dd" ]] || dd="."
  DATADIR="$(resolve_against "$RUN_CWD" "$dd" || true)"
  [[ -n "$DATADIR" && -d "$DATADIR" ]] || die unsupported-layout "cannot resolve DataDir"
  S[INVOCATION_DIR]="$INVOCATION_DIR"
  S[RUN_USER]="$RUN_USER"
  S[RUN_CWD]="$RUN_CWD"
  S[ORIG_PID]="$ORIG_PID"
}

validate_common() {
  local paths=("$CONFIG" "$DATADIR" "$ORIG_EXE")
  [[ "$LAYOUT" == "manual-directory" ]] && paths+=("$RUN_CWD" "$INVOCATION_DIR")
  for p in "${paths[@]}"; do
    path_sane "$p" || die unsupported-layout "path contains unsupported characters: $p"
  done
  if [[ ! -f "$DATADIR/harmony_db_0/CURRENT" ]]; then
    if (( DISCARD_FLAG )) && [[ "$LAYOUT" == "systemd" \
       && ! -e "$DATADIR/harmony_db_0" ]]; then
      # Supervised low-space recovery: the operator already stopped Harmony
      # and deleted the old DB. Keep an empty placeholder so the existing
      # journaled swap can rename and remove it after installing the clean DB.
      unit_is_stopped \
        || die stop-failed "harmony.service must be fully stopped before recovering an already-deleted DB"
      mkdir -p "$DATADIR/harmony_db_0"
      S[OLD_DB_PREDELETED]=1
      log "old harmony_db_0 is already absent; created an empty discard placeholder"
    else
      die unsupported-layout "no harmony_db_0/CURRENT under $DATADIR"
    fi
  fi
  local nt at arch shard
  nt="$(toml_get "$CONFIG" Network NetworkType)"
  [[ "$nt" == "mainnet" ]] || die unsupported-layout "NetworkType=$nt"
  at="$(toml_get "$CONFIG" General NodeType)"
  [[ "$at" == "validator" ]] || die unsupported-layout "NodeType=$at"
  arch="$(toml_get "$CONFIG" General IsArchival)"
  [[ "$arch" != "true" ]] || die unsupported-layout "IsArchival=true"
  shard="$(toml_get "$CONFIG" ShardData EnableShardData)"
  [[ "$shard" != "true" ]] || die unsupported-layout "EnableShardData=true"
  local other=( "$DATADIR"/harmony_db_[1-9]* )
  (( ${#other[@]} == 0 )) || die unsupported-layout "extra shard DBs present: ${other[*]}"
  if (( ROOTLESS )); then
    # Rootless mode must never need chown: everything it touches or replaces
    # has to already belong to the invoking user.
    local own
    for p in "$CONFIG" "$ORIG_EXE" "$DATADIR" "$DATADIR/harmony_db_0"; do
      own="$(stat -c %u "$p")"
      [[ "$own" == "$(id -u)" ]] || die needs-root "not owned by invoking user (uid $own): $p"
    done
    [[ -w "$DATADIR" ]] || die needs-root "datadir not writable by invoking user: $DATADIR"
  fi
  ORIG_EXE_SHA256="$(sha256sum "$ORIG_EXE" | awk '{print $1}')" \
    || die unsupported-layout "cannot hash original binary $ORIG_EXE"
  BLS_IDS="$(rpc_blskeys)"
  if ! [[ "$BLS_IDS" =~ ^[0-9a-fA-F]{96}(,[0-9a-fA-F]{96})*$ ]]; then
    if [[ "${S[OLD_DB_PREDELETED]-}" == "1" ]]; then
      BLS_IDS=unknown
      log "live RPC is unavailable; READY will report unknown BLS IDs and cannot count toward the tally"
    else
      die unsupported-layout "loopback RPC unreachable or no BLS keys loaded (got: '$BLS_IDS')"
    fi
  fi
  S[LAYOUT]="$LAYOUT"
  S[CONFIG]="$CONFIG"
  S[DATADIR]="$DATADIR"
  S[BLS_IDS]="$BLS_IDS"
  S[ORIG_EXE]="$ORIG_EXE"
  S[ORIG_EXE_SHA256]="$ORIG_EXE_SHA256"
  S[CONSENSUS_AGGREGATE_SIG]="$CONSENSUS_AGGREGATE_SIG"
  S[OLD_DB_DISPOSITION]="${S[OLD_DB_DISPOSITION]:-kept}"
}

load_facts() { # populate globals from the state file (start / prepare rerun)
  LAYOUT="${S[LAYOUT]-}"
  CONFIG="${S[CONFIG]-}"
  DATADIR="${S[DATADIR]-}"
  BLS_IDS="${S[BLS_IDS]-}"
  ORIG_EXE="${S[ORIG_EXE]-}"
  ORIG_EXE_SHA256="${S[ORIG_EXE_SHA256]-}"
  UNIT="${S[UNIT]-}"
  RUN_USER="${S[RUN_USER]-}"
  RUN_CWD="${S[RUN_CWD]-}"
  ORIG_PID="${S[ORIG_PID]-}"
  CONSENSUS_AGGREGATE_SIG="${S[CONSENSUS_AGGREGATE_SIG]-}"
  [[ -n "$LAYOUT" && -n "$CONFIG" && -n "$DATADIR" && -n "$BLS_IDS" ]] \
    || die cannot-determine-state "state file missing core facts"
  [[ -z "$CONSENSUS_AGGREGATE_SIG" || "$CONSENSUS_AGGREGATE_SIG" == "true" \
     || "$CONSENSUS_AGGREGATE_SIG" == "false" ]] \
    || die cannot-determine-state "invalid CONSENSUS_AGGREGATE_SIG in state"
  if [[ "$LAYOUT" == "systemd" ]]; then
    [[ -n "$UNIT" ]] || die cannot-determine-state "systemd layout without UNIT"
  fi
  STAGING="$DATADIR/$STAGING_NAME"
}

# ---------- stop phase ----------

stop_systemd() { # $1 = "prove" to prove the hold with a blocked start attempt
  if [[ -z "${S[WAS_ENABLED]-}" ]]; then
    S[WAS_ENABLED]="$(systemctl is-enabled "$UNIT" 2>/dev/null || true)"
    save_state
  fi
  install_hold
  rm -f "$SENTINEL"
  if ! unit_is_stopped; then
    systemctl stop "$UNIT" || true
    wait_unit_stopped || die stop-failed "unit still active after ${STOP_TIMEOUT}s"
  fi
  # Prove the hold on the initial stop only: a start attempt must leave the
  # unit inactive. (Not re-proven on later reruns: once the exec drop-in is
  # installed a start attempt would launch the recovery binary if the hold
  # had been tampered with, so reruns only verify the hold file is present.)
  if [[ "${1-}" == "prove" ]]; then
    systemctl start "$UNIT" || true
    sleep 2
    if ! unit_is_stopped; then
      systemctl stop "$UNIT" || true
      wait_unit_stopped || true
      die stop-failed "hold did not prevent unit start"
    fi
  fi
  log "systemd unit $UNIT stopped and held"
}

stop_manual() {
  local killed=0
  if [[ -n "$ORIG_PID" && -d "/proc/$ORIG_PID" ]] && pid_matches_orig "$ORIG_PID"; then
    log "stopping original harmony pid $ORIG_PID"
    kill -TERM "$ORIG_PID" 2>/dev/null || true
    killed=1
    local deadline=$(( SECONDS + STOP_TIMEOUT ))
    while (( SECONDS < deadline )); do
      [[ -d "/proc/$ORIG_PID" ]] || break
      sleep 2
    done
    [[ -d "/proc/$ORIG_PID" ]] && die stop-failed "pid $ORIG_PID still alive after ${STOP_TIMEOUT}s"
  fi
  manual_candidates
  (( ${#CAND_PIDS[@]} == 0 )) || die unsupported-layout "harmony process still present after stop: ${CAND_PIDS[*]}"
  if (( killed )); then
    log "observing ${OBSERVE_SECS}s for supervisor respawn"
    sleep "$OBSERVE_SECS"
    manual_candidates
    (( ${#CAND_PIDS[@]} == 0 )) || die unsupported-layout "harmony process reappeared (active supervisor?): ${CAND_PIDS[*]}"
  fi
  log "manual harmony process stopped and absent"
}

ensure_stopped() { # $1 = "prove" on the initial (pre-swap) stop
  if [[ "$LAYOUT" == "systemd" ]]; then stop_systemd "${1-}"; else stop_manual; fi
}

quarantine() {
  local q="$DATADIR/pre-recovery-$STAMP" moved=0 f
  for f in "$DATADIR/transactions.rlp" "$DATADIR/transactions.rlp.new" "$DATADIR/cache"; do
    if [[ -e "$f" ]]; then
      mkdir -p "$q"
      mv "$f" "$q/"
      moved=1
    fi
  done
  (( moved )) && log "quarantined txpool journal / sync caches into $q"
  return 0
}

discard_old_before_download() {
  local cur="$DATADIR/harmony_db_0" free staged old_bytes margin pct need projected

  if [[ "${S[OLD_DB_PREDELETED]-}" != "1" ]]; then
    free="$(df -B1 --output=avail "$DATADIR" | tail -1 | tr -d ' ')"
    staged=0
    [[ -d "$STAGING" ]] && staged="$(du -sb "$STAGING" | awk '{print $1}')"
    (( staged > DB_BYTES )) && staged=$DB_BYTES
    old_bytes="$(du -s -B1 "$cur" | awk '{print $1}')"
    pct=$(( DB_BYTES / 20 )); margin=$MARGIN_MIN_DISCARD_BYTES
    (( pct > margin )) && margin=$pct
    need=$(( DB_BYTES - staged + margin ))
    projected=$(( free + old_bytes ))
    log "discard projection: free=$free old-db=$old_bytes projected=$projected need=$need"
    (( projected >= need )) \
      || die low-disk "even deleting the old DB would leave projected=$projected need=$need"
  fi

  # Prove the remote and replacement binary are available before destroying
  # the only local DB. The explicit discard flag is the destructive consent.
  require_db_source_metrics pre-discard
  ensure_binary download
  ensure_stopped prove
  [[ "$LAYOUT" != "systemd" ]] || rm -f "$SENTINEL"
  require_no_duplicates ""
  quarantine

  if [[ "${S[OLD_DB_PREDELETED]-}" != "1" ]]; then
    confirm_db_deletion "$cur"
    S[OLD_DB_PREDELETED]=1
    save_state
  fi
  rm -rf "$cur"
  mkdir -p "$cur"
  sync_path "$DATADIR"
  log "old harmony_db_0 deleted; empty swap placeholder created; Harmony remains stopped"
}

# ---------- write-ahead swap machine ----------

# OLD_DB_NAME must be a plain single-path-component backup name: no slashes,
# so "$DATADIR/$OLD_DB_NAME" can never resolve outside DATADIR.
old_name_ok() {
  [[ "${S[OLD_DB_NAME]-}" =~ ^harmony_db_0\.pre-recovery-[A-Za-z0-9._-]+$ ]]
}

chown_new_db() {
  if (( ROOTLESS )); then
    # Everything was created by the invoking user; ownership is already right
    # (enforced at discovery) and chown as non-root could not change it anyway.
    chmod -R go-w "$DATADIR/harmony_db_0"
    return 0
  fi
  local u
  if [[ "$LAYOUT" == "systemd" ]]; then u="$(systemd_service_user)"; else u="$RUN_USER"; fi
  chown -R "$u:" "$DATADIR/harmony_db_0"
  chmod -R go-w "$DATADIR/harmony_db_0"
}

swap_machine() {
  local cur="$DATADIR/harmony_db_0" old extdir="$STAGING/db"
  DID_VERIFY_DB=0
  while :; do
    case "${S[STATE]}" in
      PREPARED)
        if [[ -z "${S[OLD_DB_NAME]-}" ]]; then
          S[OLD_DB_NAME]="harmony_db_0.pre-recovery-$STAMP"
        fi
        set_state SWAP_BEGUN
        ;;
      SWAP_BEGUN)
        old_name_ok \
          || die cannot-determine-state "OLD_DB_NAME missing or malformed: '${S[OLD_DB_NAME]-}'"
        old="$DATADIR/${S[OLD_DB_NAME]}"
        if [[ -d "$cur" && ! -e "$old" ]]; then
          mv "$cur" "$old"
          sync_path "$DATADIR"
        fi
        [[ -d "$old" && ! -e "$cur" ]] || die cannot-determine-state "swap: unexpected layout of $cur / $old"
        set_state OLD_RENAMED
        ;;
      OLD_RENAMED)
        if [[ ! -e "$cur" ]]; then
          [[ -d "$extdir/harmony_db_0" ]] || die cannot-determine-state "staged clean DB missing at $extdir"
          mv "$extdir/harmony_db_0" "$cur"
          sync   # flush staged contents and the rename before advancing
        fi
        [[ -d "$cur" ]] || die cannot-determine-state "install: $cur missing"
        set_state NEW_INSTALLED
        ;;
      NEW_INSTALLED)
        verify_db_dir "$cur" "installed clean DB"
        DID_VERIFY_DB=1
        chown_new_db
        if [[ "$LAYOUT" == "systemd" ]]; then
          install_exec_dropin
          verify_effective_exec || die unsupported-layout "a later drop-in still overrides ExecStart"
        fi
        if [[ "${S[DISCARD_REQUESTED]-}" == "1" && "${S[OLD_DB_DISPOSITION]-kept}" != "deleted" ]]; then
          set_state DELETING
        else
          set_state READY
        fi
        ;;
      DELETING)
        old_name_ok \
          || die cannot-determine-state "OLD_DB_NAME missing or malformed: '${S[OLD_DB_NAME]-}'"
        old="$DATADIR/${S[OLD_DB_NAME]}"
        if [[ -e "$old" ]]; then
          [[ -d "$old" && ! -L "$old" ]] || die cannot-determine-state "refusing to delete non-directory $old"
          rm -rf "$old"
          sync_path "$DATADIR"
        fi
        S[OLD_DB_DISPOSITION]="deleted"
        set_state READY
        ;;
      READY)
        if [[ "${S[DISCARD_REQUESTED]-}" == "1" && "${S[OLD_DB_DISPOSITION]-kept}" != "deleted" ]]; then
          set_state DELETING
          continue
        fi
        break
        ;;
      *)
        die cannot-determine-state "unexpected state ${S[STATE]} in swap"
        ;;
    esac
  done
}

# ---------- prepare ----------

prepare_mode() {
  local have_state=0 rank
  if load_state; then have_state=1; fi
  if (( have_state )); then
    rank="$(state_rank "${S[STATE]-}")"
    (( rank >= 1 )) || die cannot-determine-state "state file present but STATE invalid: '${S[STATE]-}'"
    if (( rank >= 7 )); then
      die cannot-determine-state "node already past READY (state ${S[STATE]}); use start"
    fi
    if (( DISCARD_FLAG )) && [[ "${S[DISCARD_REQUESTED]-}" != "1" ]]; then
      S[DISCARD_REQUESTED]=1
      save_state
    fi
    load_facts
    log "rerun with recorded state ${S[STATE]} (layout $LAYOUT)"
  else
    discover_layout
    STAGING="$DATADIR/$STAGING_NAME"
    (( DISCARD_FLAG )) && S[DISCARD_REQUESTED]=1
    set_state PREPARED   # discovery facts recorded before any change
    rank=1
  fi

  if [[ "$LAYOUT" == "systemd" ]]; then
    require_tools systemctl
  else
    require_tools setsid
    if (( ! ROOTLESS )); then
      require_tools runuser
    fi
  fi

  # Normal downloads happen while the node runs. Explicit discard mode first
  # proves the source/binary, stops the validator, and deletes the old DB so
  # low-space machines can stage the clean DB. Reruns resume from PREPARED.
  if (( rank <= 1 )) && [[ "${S[DISCARD_REQUESTED]-}" == "1" ]]; then
    discard_old_before_download
  fi

  # Skip the disk gate and DB staging once the swap has begun (the staged DB
  # is consumed by it), but always ensure the binary.
  if (( rank <= 1 )); then
    disk_gate
    ensure_db_staged
  fi
  ensure_binary download

  if (( rank <= 1 )); then ensure_stopped prove; else ensure_stopped; fi
  if [[ "$LAYOUT" == "systemd" ]]; then rm -f "$SENTINEL"; fi
  require_no_duplicates ""
  quarantine
  swap_machine

  # Final READY re-verification (also covers reruns landing directly on READY).
  ensure_binary verify
  if [[ "$LAYOUT" == "systemd" ]]; then
    [[ -f "$(hold_dropin_path)" ]] || die cannot-determine-state "hold drop-in missing at READY"
    verify_effective_exec || die unsupported-layout "a later drop-in still overrides ExecStart"
  fi
  (( DID_VERIFY_DB )) || verify_db_dir "$DATADIR/harmony_db_0" "installed clean DB (rerun)"
  emit "READY $BLS_IDS $SUFFIX"
}

# ---------- start ----------

rpc_healthy() {
  local bn h keys
  bn="$(rpc_call hmyv2_blockNumber '[]' | jq -r '.result // empty' 2>/dev/null || true)"
  [[ "$bn" =~ ^[0-9]+$ ]] || return 1
  (( bn >= TARGET_HEIGHT )) || return 1
  h="$(rpc_call hmyv2_getBlockByNumber "[$TARGET_HEIGHT,{}]" | jq -r '.result.hash // empty' 2>/dev/null || true)"
  [[ "$h" == "$TARGET_HASH" ]] || return 1
  if [[ "$BLS_IDS" != "unknown" ]]; then
    keys="$(rpc_blskeys)"
    [[ "$keys" == "$BLS_IDS" ]] || return 1
  fi
  return 0
}

target_hash_answer() { # hash the node reports for the pinned height ("" if unknown)
  rpc_call hmyv2_getBlockByNumber "[$TARGET_HEIGHT,{}]" | jq -r '.result.hash // empty' 2>/dev/null || true
}

# A definite wrong answer for the pinned block is fatal and LATCHED: the
# mismatch is written to the state file before the stop, so ordinary start
# reruns refuse to relaunch until the team investigates and removes the
# HEAD_MISMATCH line. The stop is the proven quarantine (hold plus verified
# inactivity on systemd; verified process death on manual).
latch_head_mismatch() { # <observed-hash>
  S[HEAD_MISMATCH]="$1"
  save_state
  quarantine_unhealthy_node
  die head-mismatch "block $TARGET_HEIGHT hash $1 != pinned $TARGET_HASH (latched; node stays stopped)"
}

node_is_up() { # sets LEGIT_PID when up
  LEGIT_PID=""
  if [[ "$LAYOUT" == "systemd" ]]; then
    [[ "$(unit_active_state)" == "active" ]] || return 1
    LEGIT_PID="$(systemctl show "$UNIT" -p MainPID --value)"
    return 0
  fi
  local pid="${S[NODE_PID]-}"
  if [[ -n "$pid" && -d "/proc/$pid" ]]; then
    local exe tgt cmd
    exe="$(readlink "/proc/$pid/exe" 2>/dev/null || true)"; tgt="${exe% (deleted)}"
    cmd="$(tr '\0' ' ' < "/proc/$pid/cmdline" 2>/dev/null || true)"
    if [[ "$tgt" == "$BIN" && "$cmd" == *"$CONFIG"* ]]; then
      LEGIT_PID="$pid"
      return 0
    fi
  fi
  find_recovery_pids
  if (( ${#RECOVERY_PIDS[@]} == 1 )); then
    LEGIT_PID="${RECOVERY_PIDS[0]}"
    S[NODE_PID]="$LEGIT_PID"
    save_state
    return 0
  fi
  return 1
}

# For systemd, an "up" unit must actually be running our staged command: the
# effective ExecStart must be the receipt and MainPID must execute the staged
# binary. (Manual layout needs no extra check: node_is_up already requires
# exe == staged binary and our config on the command line.) On mismatch the
# unit is stopped and held before dying.
verify_running_identity() {
  [[ "$LAYOUT" == "systemd" ]] || return 0
  local exe
  exe="$(readlink "/proc/$LEGIT_PID/exe" 2>/dev/null || true)"
  if ! verify_effective_exec || [[ "${exe% (deleted)}" != "$BIN" ]]; then
    quarantine_unhealthy_node
    die receipt-mismatch "active unit is not running the staged recovery binary (pid=$LEGIT_PID exe=$exe)"
  fi
}

stop_started_node() { # stop whatever we started; systemd hold stays installed
  if [[ "$LAYOUT" == "systemd" ]]; then
    systemctl stop "$UNIT" || true
    rm -f "$SENTINEL"
    wait_unit_stopped || true
  else
    local pid="${S[NODE_PID]-}"
    if [[ -n "$pid" && -d "/proc/$pid" ]]; then
      kill -TERM "$pid" 2>/dev/null || true
      local deadline=$(( SECONDS + 10 ))
      while (( SECONDS < deadline )) && [[ -d "/proc/$pid" ]]; do sleep 1; done
      [[ -d "/proc/$pid" ]] && kill -KILL "$pid" 2>/dev/null || true
    fi
  fi
}

# Unhealthy after STARTED: hold must block Restart=on-failure before the stop.
# The stop is proven: if the node cannot be confirmed inactive, this dies
# stop-failed rather than letting the caller report any other STOPPED reason
# while a signer may still be running.
quarantine_unhealthy_node() {
  if [[ "$LAYOUT" == "systemd" ]]; then
    install_hold
    rm -f "$SENTINEL"
    systemctl stop "$UNIT" || true
    wait_unit_stopped || die stop-failed "unit still active after ${STOP_TIMEOUT}s while quarantining"
  else
    stop_started_node
    local pid="${S[NODE_PID]-}" deadline=$(( SECONDS + 10 ))
    while [[ -n "$pid" && -d "/proc/$pid" ]] && (( SECONDS < deadline )); do sleep 1; done
    if [[ -n "$pid" && -d "/proc/$pid" ]]; then
      die stop-failed "node pid $pid still alive after quarantine stop"
    fi
  fi
}

# Manual latched rerun: autostart may have relaunched the recorded original
# binary, and anything else may be touching the recorded config/DataDir/DB.
# Stop only processes whose identity is unambiguous under the manual layout
# contract: the staged node (exe is the staged binary, our config on the
# command line) and the recorded original binary (exe is exactly ORIG_EXE).
# Then require the full duplicate scan (exe path/SHA of original and staged
# binary, config/DataDir on the command line, open fds under the DB) to come
# back clean; any remaining match has no identity safe to stop, so it is
# named and reported stop-failed, never head-mismatch.
latched_manual_requarantine() {
  local pid killed=0
  find_recovery_pids
  for pid in ${RECOVERY_PIDS[@]+"${RECOVERY_PIDS[@]}"}; do
    log "head-mismatch latch: stopping staged node pid $pid"
    kill_pid_proven "$pid" \
      || die stop-failed "staged node pid $pid still alive under head-mismatch latch"
    killed=1
  done
  find_orig_exe_pids
  for pid in ${ORIG_EXE_PIDS[@]+"${ORIG_EXE_PIDS[@]}"}; do
    log "head-mismatch latch: stopping relaunched original node pid $pid ($ORIG_EXE)"
    kill_pid_proven "$pid" \
      || die stop-failed "original node pid $pid still alive under head-mismatch latch"
    killed=1
  done
  if (( killed )); then
    log "observing ${OBSERVE_SECS}s for supervisor respawn"
    sleep "$OBSERVE_SECS"
  fi
  scan_duplicates "" \
    || die stop-failed "signer(s) still active under head-mismatch latch: $DUP_HITS"
}

launch_node() {
  if [[ "$LAYOUT" == "systemd" ]]; then
    verify_effective_exec || die receipt-mismatch "effective ExecStart is not the staged recovery command"
    mkdir -p "$SENTINEL_DIR"
    : > "$SENTINEL"
    if [[ "${S[WAS_ENABLED]-}" == enabled* ]]; then
      systemctl enable "$UNIT" >/dev/null 2>&1 || true
    fi
    systemctl start "$UNIT" || { stop_started_node; die start-failed "systemctl start failed"; }
    local deadline=$(( SECONDS + START_ACTIVE_TIMEOUT ))
    while (( SECONDS < deadline )); do
      [[ "$(unit_active_state)" == "active" ]] && break
      sleep 1
    done
    [[ "$(unit_active_state)" == "active" ]] || { stop_started_node; die start-failed "unit not active after ${START_ACTIVE_TIMEOUT}s"; }
    sleep 3
    [[ "$(unit_active_state)" == "active" ]] || { stop_started_node; die start-failed "unit did not stay active"; }
    S[NODE_PID]="$(systemctl show "$UNIT" -p MainPID --value)"
    save_state
  else
    [[ -d "$RUN_CWD" ]] || die start-failed "recorded cwd $RUN_CWD missing"
    log "launching staged binary as $RUN_USER from $RUN_CWD (rootless=$ROOTLESS)"
    local nodecmd=(setsid "$BIN" -c "$CONFIG" --datadir "$DATADIR")
    if [[ -n "$CONSENSUS_AGGREGATE_SIG" ]]; then
      nodecmd+=("--consensus.aggregate-sig=$CONSENSUS_AGGREGATE_SIG")
    fi
    (( ROOTLESS )) || nodecmd=(runuser -u "$RUN_USER" -- "${nodecmd[@]}")
    # Close fd3 (caller stdout), fd4 (operator progress), and fd9 (the flock)
    # so the daemon inherits none of them; otherwise callers capturing output
    # could hang until the node exits, and the node would hold the lock.
    ( cd "$RUN_CWD" && exec "${nodecmd[@]}" ) \
      >> "$PRIV/node.log" 2>&1 < /dev/null 3>&- 4>&- 9>&- &
    local launcher=$!
    # $! may be a wrapper: rediscover the actual harmony PID via /proc.
    local deadline=$(( SECONDS + START_ACTIVE_TIMEOUT )) found=0
    while (( SECONDS < deadline )); do
      find_recovery_pids
      if (( ${#RECOVERY_PIDS[@]} >= 1 )); then found=1; break; fi
      sleep 1
    done
    if (( ! found )); then
      kill -TERM "$launcher" 2>/dev/null || true
      die start-failed "no process running the staged binary after ${START_ACTIVE_TIMEOUT}s (see node.log)"
    fi
    if (( ${#RECOVERY_PIDS[@]} > 1 )); then
      local p
      for p in "${RECOVERY_PIDS[@]}"; do kill -TERM "$p" 2>/dev/null || true; done
      die start-failed "multiple processes running the staged binary: ${RECOVERY_PIDS[*]}"
    fi
    S[NODE_PID]="${RECOVERY_PIDS[0]}"
    save_state
    sleep 3
    [[ -d "/proc/${S[NODE_PID]}" ]] || die start-failed "node exited immediately (see node.log)"
  fi
}

wait_healthy() {
  local deadline=$(( SECONDS + START_RPC_TIMEOUT )) h
  while (( SECONDS < deadline )); do
    node_is_up || { stop_started_node; die start-failed "node died before becoming healthy"; }
    # There is no content hash on the DB transfer, so this RPC pin is the head
    # trust anchor: a definite wrong answer quarantines and latches.
    h="$(target_hash_answer)"
    if [[ -n "$h" && "$h" != "$TARGET_HASH" ]]; then
      latch_head_mismatch "$h"
    fi
    if rpc_healthy; then return 0; fi
    sleep 5
  done
  stop_started_node
  die unhealthy "RPC health not reached in ${START_RPC_TIMEOUT}s (blockNumber/target-hash/blskey pin)"
}

finish_running() {
  if [[ "$BLS_IDS" == "unknown" ]]; then
    local keys
    keys="$(rpc_blskeys)"
    if [[ "$keys" =~ ^[0-9a-fA-F]{96}(,[0-9a-fA-F]{96})*$ ]]; then
      BLS_IDS="$keys"
      S[BLS_IDS]="$keys"
      save_state
      log "captured public BLS IDs from the recovered node"
    fi
  fi
  set_state STARTED
  if [[ "$LAYOUT" == "systemd" ]]; then
    remove_hold
    rm -f "$SENTINEL"
  fi
  emit "RUNNING $BLS_IDS $SUFFIX"
}

start_mode() {
  load_state || die not-ready "no state file; run prepare first"
  local rank h; rank="$(state_rank "${S[STATE]-}")"
  (( rank >= 1 )) || die cannot-determine-state "STATE invalid: '${S[STATE]-}'"
  load_facts
  # A latched head mismatch blocks every ordinary start rerun: the node must
  # stay stopped until the team investigates and removes the HEAD_MISMATCH
  # line from the state file. The latch is saved before the quarantine stop,
  # so a crash or failed stop can leave the wrong-head node running: every
  # latched rerun re-proves the quarantine (systemd: hold reapplied, unit
  # stopped with verified inactivity; manual: verified process death) before
  # reporting head-mismatch. An unprovable stop is stop-failed, never
  # head-mismatch.
  if [[ -n "${S[HEAD_MISMATCH]-}" ]]; then
    if [[ "$LAYOUT" == "systemd" ]]; then
      if node_is_up; then
        quarantine_unhealthy_node
      fi
      install_hold
      rm -f "$SENTINEL"
      if ! unit_is_stopped; then
        systemctl stop "$UNIT" || true
        wait_unit_stopped \
          || die stop-failed "unit not provably stopped under head-mismatch latch"
      fi
    else
      latched_manual_requarantine
    fi
    die head-mismatch "latched: block $TARGET_HEIGHT previously reported ${S[HEAD_MISMATCH]}; node stays stopped"
  fi
  case "${S[STATE]}" in
    READY)
      ensure_binary verify
      require_no_duplicates ""
      if [[ "$LAYOUT" == "systemd" ]]; then
        unit_is_stopped || die not-ready "unit is not inactive"
        [[ -f "$(hold_dropin_path)" ]] || die not-ready "hold drop-in missing"
        [[ -f "$(exec_dropin_path)" ]] || die not-ready "exec drop-in missing"
        verify_effective_exec || die receipt-mismatch "effective ExecStart is not the staged recovery command"
      fi
      verify_db_dir "$DATADIR/harmony_db_0" "installed clean DB (pre-launch)"
      chown_new_db     # re-assert run-user ownership before launch
      set_state STARTING
      launch_node
      wait_healthy
      finish_running
      ;;
    STARTING)
      if node_is_up; then
        log "adopting already-running node pid=$LEGIT_PID"
        verify_running_identity
        binary_ok || { quarantine_unhealthy_node; die not-ready "staged recovery binary missing, hash mismatch, or wrong ELF"; }
        scan_duplicates "$LEGIT_PID" || { quarantine_unhealthy_node; die duplicate-process "hits: $DUP_HITS"; }
      else
        ensure_binary verify
        require_no_duplicates ""
        launch_node
      fi
      wait_healthy
      finish_running
      ;;
    STARTED)
      if node_is_up; then
        verify_running_identity
        binary_ok || { quarantine_unhealthy_node; die not-ready "staged recovery binary missing, hash mismatch, or wrong ELF"; }
        scan_duplicates "$LEGIT_PID" || { quarantine_unhealthy_node; die duplicate-process "hits: $DUP_HITS"; }
        if rpc_healthy; then
          # Heal a crash that landed between STARTED and hold removal.
          if [[ "$LAYOUT" == "systemd" ]]; then remove_hold; rm -f "$SENTINEL"; fi
          emit "RUNNING $BLS_IDS $SUFFIX"
        else
          # A definite wrong target hash is head-mismatch (latched), never a
          # generic unhealthy.
          h="$(target_hash_answer)"
          if [[ -n "$h" && "$h" != "$TARGET_HASH" ]]; then
            latch_head_mismatch "$h"
          fi
          quarantine_unhealthy_node
          die unhealthy "post-success node failed the RPC pin; stopped again"
        fi
      else
        # STATE=STARTED means the hold was already removed: reinstall it
        # before anything can fail or relaunch, so an enabled unit can never
        # start without GO again. Removed only after health succeeds.
        if [[ "$LAYOUT" == "systemd" ]]; then install_hold; rm -f "$SENTINEL"; fi
        ensure_binary verify
        require_no_duplicates ""
        launch_node
        wait_healthy
        finish_running
      fi
      ;;
    PREPARED|SWAP_BEGUN|OLD_RENAMED|NEW_INSTALLED|DELETING)
      die not-ready "prepare has not reached READY (state ${S[STATE]})"
      ;;
    *)
      die cannot-determine-state "unexpected state ${S[STATE]}"
      ;;
  esac
}

# ---------- entry ----------

main() {
  INVOCATION_DIR="$(pwd -P)"

  case "${1-}" in
    prepare)
      MODE=prepare; shift
      while (( $# > 0 )); do
        case "$1" in
          --discard-old-db) DISCARD_FLAG=1 ;;
          --quiet) QUIET=1 ;;
          *) usage_exit ;;
        esac
        shift
      done
      ;;
    start)
      MODE=start; shift
      [[ $# -eq 0 ]] || usage_exit
      ;;
    *) usage_exit ;;
  esac

  ROOTLESS=0
  if [[ "$(id -u)" != "0" ]]; then
    # Rootless manual-validator mode: all state, staging, and the staged
    # binary live under the invocation directory, owned by the invoking user.
    ROOTLESS=1
    WORK="$INVOCATION_DIR/$STAGING_NAME/work"
    BIN="$WORK/bin/harmony-recovery"
    PRIV="$WORK/private"
    STATE_FILE="$PRIV/state"
    LOCK_FILE="$PRIV/lock"
  fi

  LOGID="$(date +%Y%m%d-%H%M%S)-$$"
  if (( ROOTLESS )); then
    install -d -m 0700 "$WORK" "$WORK/bin"
  else
    install -d -m 0711 "$WORK"
    install -d -m 0755 "$WORK/bin"
  fi
  install -d -m 0700 "$PRIV"
  local logfile="$PRIV/run-$LOGID.log"
  : > "$logfile"
  chmod 600 "$logfile"

  # fd3 = the single final stdout line; fd4 = live progress on the original
  # stderr; detailed output and xtrace continue to the run log.
  exec 3>&1 4>&2 1>>"$logfile" 2>&1
  trap on_exit EXIT
  PS4='+ rollback:${LINENO}: '
  set -x

  exec 9>"$LOCK_FILE"
  flock -n 9 || die cannot-determine-state "another invocation holds the lock"

  log "mode=$MODE discard=$DISCARD_FLAG quiet=$QUIET invocation_dir=$INVOCATION_DIR rootless=$ROOTLESS logid=$LOGID script_url=$SCRIPT_URL"
  case "$(uname -sm)" in
    "Linux x86_64")
      ARCH=amd64; ELF_MACHINE=3e00
      NODE_BIN_URL="$NODE_BIN_URL_AMD64"; NODE_BIN_SHA256="$NODE_BIN_SHA256_AMD64" ;;
    "Linux aarch64")
      ARCH=arm64; ELF_MACHINE=b700
      NODE_BIN_URL="$NODE_BIN_URL_ARM64"; NODE_BIN_SHA256="$NODE_BIN_SHA256_ARM64" ;;
    *)
      die unsupported-platform "uname: $(uname -sm) (need Linux x86_64 or Linux aarch64)" ;;
  esac
  log "platform $(uname -sm) -> linux-$ARCH artifact"
  require_tools curl rclone find sha256sum flock stat df du awk sed grep jq od cat pgrep fuser install getent readlink sync tee

  STAMP="$(date +%Y%m%d-%H%M%S)"
  ORIG_EXE=""; ORIG_EXE_SHA256=""; CONFIG=""; DATADIR=""; BLS_IDS=""
  CONSENSUS_AGGREGATE_SIG=""
  UNIT=""; RUN_USER=""; RUN_CWD=""; ORIG_PID=""; STAGING=""; DID_VERIFY_DB=0

  if [[ "$MODE" == "prepare" ]]; then
    prepare_mode
  else
    start_mode
  fi
}

main "$@"
}
