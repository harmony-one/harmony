#!/usr/bin/env bash

version="v1 20190924.0"

unset -v progname
progname="${0##*/}"

unset -f msg err

msg() {
   case $# in
   [1-9]*)
      echo "${progname}: $*" >&2
      ;;
   esac
}

err() {
   local code
   code="${1}"
   shift 1
   msg "$@"
   exit "${code}"
}

# https://www.linuxjournal.com/content/validating-ip-address-bash-script
function valid_ip()
{
    local  ip=$1
    local  stat=1

    if [[ $ip =~ ^[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}$ ]]; then
        OIFS=$IFS
        IFS='.'
        ip=($ip)
        IFS=$OIFS
        [[ ${ip[0]} -le 255 && ${ip[1]} -le 255 \
            && ${ip[2]} -le 255 && ${ip[3]} -le 255 ]]
        stat=$?
    fi
    return $stat
}

function myip() {
# get ipv4 address only, right now only support ipv4 addresses
   PUB_IP=$(dig -4 @resolver1.opendns.com ANY myip.opendns.com +short)
   if valid_ip $PUB_IP; then
      msg "public IP address autodetected: $PUB_IP"
   else
      err 1 "NO valid public IP found: $PUB_IP"
   fi
}

function check_root
{
   if [[ $EUID -ne 0 ]]; then
      msg "this script must be run as root to setup environment"
      msg please use \"sudo ${progname}\"
      exit 1
   fi
}

function add_env
{
   filename=$1
   shift
   grep -qxF "$@" $filename || echo "$@" >> $filename
}

function setup_env
{
   check_root

# setup environment variables, may not be nessary
   sysctl -w net.core.somaxconn=1024
   sysctl -w net.core.netdev_max_backlog=65536
   sysctl -w net.ipv4.tcp_tw_reuse=1
   sysctl -w net.ipv4.tcp_rmem='4096 65536 16777216'
   sysctl -w net.ipv4.tcp_wmem='4096 65536 16777216'
   sysctl -w net.ipv4.tcp_mem='65536 131072 262144'

   add_env /etc/security/limits.conf "* soft     nproc          65535"
   add_env /etc/security/limits.conf "* hard     nproc          65535"
   add_env /etc/security/limits.conf "* soft     nofile         65535"
   add_env /etc/security/limits.conf "* hard     nofile         65535"
   add_env /etc/security/limits.conf "root soft     nproc          65535"
   add_env /etc/security/limits.conf "root hard     nproc          65535"
   add_env /etc/security/limits.conf "root soft     nofile         65535"
   add_env /etc/security/limits.conf "root hard     nofile         65535"
   add_env /etc/pam.d/common-session "session required pam_limits.so"
}

######## main #########
print_usage() {
   cat <<- ENDEND

usage: ${progname} [-1ch] [-k KEYFILE]
   -c             back up database/logs and start clean
                  (use only when directed by Harmony)
   -1             do not loop; run once and exit
   -h             print this help and exit
   -k KEYFILE     use the given BLS key file (default: autodetect)
   -s             run setup env only (must run as root)
   -S             run the ${progname} as non-root user (default: run as root)
   -p passfile    use the given BLS passphrase file
   -d             just download the Harmony binaries (default: off)
   -D             do not download Harmony binaries (default: download when start)
   -m             collect and upload node metrics to harmony prometheus + grafana
   -N network     join the given network (main, beta, pangaea; default: main)
   -t             equivalent to -N pangaea (deprecated)
   -T nodetype    specify the node type (validator, explorer; default: validator)
   -i shardid     specify the shard id (valid only with explorer node; default: 1)
   -b             download harmony_db files from shard specified by -i <shardid> (default: off)
   -a dbfile      specify the db file to download (default:off)
   -U FOLDER      specify the upgrade folder to download binaries
   -P             enable public rpc end point (default:off)
   -v             print out the version of the node.sh
   -V             print out the version of the Harmony binary

examples:

# start node program w/o root account
   ${progname} -S -k mybls.key

# download beacon chain (shard0) db snapshot
   ${progname} -i 0 -b

# just re-download the harmony binaries
   ${progname} -d

# start a non-validating node in shard 1
# you need to have a dummy BLSKEY/pass file using 'touch BLSKEY; touch blspass'
   ${progname} -S -k BLSKEY -p blspass -T explorer -i 1

# upgrade harmony binaries from specified repo
   ${progname} -1 -U upgrade

ENDEND
}

usage() {
   msg "$@"
   print_usage >&2
   exit 64  # EX_USAGE
}

# =======
BUCKET=pub.harmony.one
OS=$(uname -s)

unset start_clean loop run_as_root blspass do_not_download download_only metrics network node_type shard_id download_harmony_db db_file_to_dl
unset upgrade_rel public_rpc
start_clean=false
loop=true
run_as_root=true
do_not_download=false
download_only=false
metrics=false
network=main
node_type=validator
shard_id=1
download_harmony_db=false
public_rpc=false
${BLSKEYFILE=}

unset OPTIND OPTARG opt
OPTIND=1
while getopts :1chk:sSp:dDmN:tT:i:ba:U:PvV opt
do
   case "${opt}" in
   '?') usage "unrecognized option -${OPTARG}";;
   ':') usage "missing argument for -${OPTARG}";;
   b) download_harmony_db=true;;
   c) start_clean=true;;
   1) loop=false;;
   h) print_usage; exit 0;;
   k) BLSKEYFILE="${OPTARG}";;
   s) setup_env; exit 0;;
   S) run_as_root=false ;;
   p) blspass="${OPTARG}";;
   d) download_only=true;;
   D) do_not_download=true;;
   m) metrics=true;;
   N) network="${OPTARG}";;
   t) network=pangaea;;
   T) node_type="${OPTARG}";;
   i) shard_id="${OPTARG}";;
   a) db_file_to_dl="${OPTARG}";;
   U) upgrade_rel="${OPTARG}";;
   P) public_rpc=true;;
   v) msg "version: $version"
      exit 0 ;;
   V) LD_LIBRARY_PATH=. ./harmony -version
      exit 0 ;;
   *) err 70 "unhandled option -${OPTARG}";;  # EX_SOFTWARE
   esac
done
shift $((${OPTIND} - 1))

unset -v bootnodes REL network_type dns_zone

case "${node_type}" in
validator|explorer) ;;
*)
   usage ;;
esac

case "${network}" in
main)
  bootnodes=(
    /ip4/100.26.90.187/tcp/9874/p2p/Qmdfjtk6hPoyrH1zVD9PEH4zfWLo38dP2mDvvKXfh3tnEv
    /ip4/54.213.43.194/tcp/9874/p2p/QmZJJx6AdaoEkGLrYG4JeLCKeCKDjnFz2wfHNHxAqFSGA9
    /ip4/13.113.101.219/tcp/12019/p2p/QmQayinFSgMMw5cSpDUiD9pQ2WeP6WNmGxpZ6ou3mdVFJX
    /ip4/99.81.170.167/tcp/12019/p2p/QmRVbTpEYup8dSaURZfF6ByrMTSKa4UyUzJhSjahFzRqNj
  )
  REL=mainnet
  network_type=mainnet
  dns_zone=t.hmny.io
  ;;
pangaea)
  bootnodes=(
    /ip4/54.218.73.167/tcp/9876/p2p/QmWBVCPXQmc2ULigm3b9ayCZa15gj25kywiQQwPhHCZeXj
    /ip4/18.232.171.117/tcp/9876/p2p/QmfJ71Eb7XTDs8hX2vPJ8un4L7b7RiDk6zCzWVxLXGA6MA
  )
  REL=pangaea
  network_type=pangaea
  dns_zone=p.hmny.io
  ;;
*)
  err 64 "${network}: invalid network"
  ;;
esac

case $# in
[1-9]*)
   usage "extra arguments at the end ($*)"
   ;;
esac

# reset REL if upgrade_rel is set
if [ -n "$upgrade_rel" ]; then
   REL="${upgrade_rel}"
fi

if [ "$OS" == "Darwin" ]; then
   FOLDER=release/darwin-x86_64/$REL/
   BIN=( harmony libbls384_256.dylib libcrypto.1.0.0.dylib libgmp.10.dylib libgmpxx.4.dylib libmcl.dylib md5sum.txt )
fi
if [ "$OS" == "Linux" ]; then
   FOLDER=release/linux-x86_64/$REL/
   BIN=( harmony libbls384_256.so libcrypto.so.10 libgmp.so.10 libgmpxx.so.4 libmcl.so md5sum.txt )
fi

extract_checksum() {
   awk -v basename="${1}" '
      {
         s = $0;
      }
      # strip hash and following space; skip line if unsuccessful
      sub(/^[0-9a-f]+ /, "", s) == 0 { next; }
      # save hash
      { hash = substr($0, 1, length($0) - length(s) - 1); }
      # strip executable indicator (space or asterisk); skip line if unsuccessful
      sub(/^[* ]/, "", s) == 0 { next; }
      # leave basename only
      { sub(/^.*\//, "", s); }
      # if basename matches, print the hash and basename
      s == basename { printf "%s  %s\n", hash, basename; }
   '
}

verify_checksum() {
   local dir file checksum_file checksum_for_file
   dir="${1}"
   file="${2}"
   checksum_file="${3}"
   [ -f "${dir}/${checksum_file}" ] || return 0
   checksum_for_file="${dir}/${checksum_file}::${file}"
   extract_checksum "${file}" < "${dir}/${checksum_file}" > "${checksum_for_file}"
   [ -s "${dir}/${checksum_for_file}" ] || return 0
   if ! (cd "${dir}" && exec md5sum -c --status "${checksum_for_file}")
   then
      msg "checksum FAILED for ${file}"
      return 1
   fi
   return 0
}

download_binaries() {
   local outdir
   ${do_not_download} && return 0
   outdir="${1:-.}"
   mkdir -p "${outdir}"
   for bin in "${BIN[@]}"; do
      curl -sSf http://${BUCKET}.s3.amazonaws.com/${FOLDER}${bin} -o "${outdir}/${bin}" || return $?
      verify_checksum "${outdir}" "${bin}" md5sum.txt || return $?
      msg "downloaded ${bin}"
   done
   chmod +x "${outdir}/harmony"
   (cd "${outdir}" && exec openssl sha256 "${BIN[@]}") > "${outdir}/harmony-checksums.txt"
}

check_free_disk() {
   local dir
   dir="${1:-.}"
   local free_disk=$(df -BG $dir | tail -n 1 | awk ' { print $4 } ' | tr -d G)
   # need at least 50G free disk space
   local need_disk=50

   if [ $free_disk -gt $need_disk ]; then
      return 0
   else
      return 1
   fi
}

_curl_check_exist() {
   local url=$1
   local statuscode=$(curl -I --silent --output /dev/null --write-out "%{http_code}" $url)
   if [ $statuscode -ne 200 ]; then
      return 1
   else
      return 0
   fi
}

_curl_download() {
   local url=$1
   local outdir=$2
   local filename=$3

   mkdir -p "${outdir}"
   if _curl_check_exist $url; then
      curl --progress-bar -Sf $url -o "${outdir}/$filename" || return $?
      return 0
   else
      msg "failed to find/download $url"
      return 1
   fi
}

download_harmony_db_file() {
   local shard_id
   shard_id="${1}"
   local file_to_dl="${2}"
   local outdir=db
   if ! check_free_disk; then
      err 70 "do not have enough free disk space to download db tarball"
   fi

   url="http://${BUCKET}.s3.amazonaws.com/${FOLDER}db/md5sum.txt"
   rm -f "${outdir}/md5sum.txt"
   if ! _curl_download $url "${outdir}" md5sum.txt; then
      err 70 "cannot download md5sum.txt"
   fi

   if [ -n "${file_to_dl}" ]; then
      if grep -q "${file_to_dl}" "${outdir}/md5sum.txt"; then
         url="http://${BUCKET}.s3.amazonaws.com/${FOLDER}db/${file_to_dl}"
         if _curl_download $url "${outdir}" ${file_to_dl}; then
            verify_checksum "${outdir}" "${file_to_dl}" md5sum.txt || return $?
            msg "downloaded ${file_to_dl}, extracting ..."
            tar -C "${outdir}" -xvf "${outdir}/${file_to_dl}"
         else
            msg "can't download ${file_to_dl}"
         fi
      fi
      return
   fi

   files=$(awk '{ print $2 }' ${outdir}/md5sum.txt)
   echo "[available harmony db files for shard ${shard_id}]"
   grep -oE "harmony_db_${shard_id}"-.*.tar "${outdir}/md5sum.txt"
   echo
   for file in $files; do
      if [[ $file =~ "harmony_db_${shard_id}" ]]; then
         echo -n "Do you want to download ${file} (choose one only) [y/n]?"
         read yesno
         if [[ "$yesno" = "y" || "$yesno" = "Y" ]]; then
            url="http://${BUCKET}.s3.amazonaws.com/${FOLDER}db/$file"
            if _curl_download $url "${outdir}" $file; then
               verify_checksum "${outdir}" "${file}" md5sum.txt || return $?
               msg "downloaded $file, extracting ..."
               tar -C "${outdir}" -xvf "${outdir}/${file}"
            else
               msg "can't download $file"
            fi
            break
         fi
      fi
   done
}

if ${download_only}; then
   download_binaries staging || err 69 "download node software failed"
   msg "downloaded files are in staging direectory"
   exit 0
fi

if ${download_harmony_db}; then
   download_harmony_db_file "${shard_id}" "${db_file_to_dl}" || err 70 "download harmony_db file failed"
   exit 0
fi

if ${run_as_root}; then
   check_root
fi

case "${BLSKEYFILE}" in
"")
   unset -v f
   for f in \
      ~/*--????-??-??T??-??-??.*Z--bls_???????????????????????????????????????????????????????????????????????????????????????????????? \
      ~/????????????????????????????????????????????????????????????????????????????????????????????????.key \
      *--????-??-??T??-??-??.*Z--bls_???????????????????????????????????????????????????????????????????????????????????????????????? \
      ????????????????????????????????????????????????????????????????????????????????????????????????.key
   do
      [ -f "${f}" ] || continue
      case "${BLSKEYFILE}" in
      "")
         BLSKEYFILE="${f}"
         ;;
      *)
         [ "${f}" -ef "${BLSKEYFILE}" ] || \
            err 69 "multiple key files found (${f}, ${BLSKEYFILE}); please use -k to specify"
         ;;
      esac
   done
   case "${BLSKEYFILE}" in
   "") err 69 "could not autodetect BLS key file; please use -k to specify";;
   esac
   msg "autodetected BLS key file: ${BLSKEYFILE}"
   ;;
*)
   msg "using manually specified BLS key file: ${BLSKEYFILE}"
   ;;
esac

any_new_binaries() {
   local outdir
   ${do_not_download} && return 0
   outdir="${1:-.}"
   mkdir -p "${outdir}"
   curl -sSf http://${BUCKET}.s3.amazonaws.com/${FOLDER}md5sum.txt -o "${outdir}/md5sum.txt.new" || return $?
   if diff $outdir/md5sum.txt.new md5sum.txt
   then
      rm "${outdir}/md5sum.txt.new"
   else
      mv "${outdir}/md5sum.txt.new" "${outdir}/md5sum.txt"
      return 1
   fi
}

if any_new_binaries
then
   msg "binaries did not change"
else
   download_binaries || err 69 "initial node software update failed"
fi

NODE_PORT=9000
PUB_IP=
METRICS=
PUSHGATEWAY_IP=
PUSHGATEWAY_PORT=

if [ "$OS" == "Linux" ]; then
   if ${run_as_root}; then
      setup_env
   fi
# Kill existing soldier/node
   fuser -k -n tcp $NODE_PORT
fi

# find my public ip address
myip

unset -v BN_MA bn
for bn in "${bootnodes[@]}"
do
  BN_MA="${BN_MA+"${BN_MA},"}${bn}"
done

if ${start_clean}
then
   msg "backing up old database/logs (-c)"
   unset -v backup_dir now
   now=$(date -u +%Y-%m-%dT%H:%M:%SZ)
   mkdir -p backups
   backup_dir=$(mktemp -d "backups/${now}.XXXXXX")
   mv harmony_db_* latest "${backup_dir}/" || :
   rm -rf latest
fi
mkdir -p latest

unset -v check_update_pid

cleanup() {
   local trap_sig kill_sig

   trap_sig="${1:-EXIT}"

   kill_sig="${trap_sig}"
   case "${kill_sig}" in
   0|EXIT|2|INT) kill_sig=TERM;;
   esac

   case "${check_update_pid+set}" in
   set)
      msg "terminating update checker (pid ${check_update_pid})"
      kill -${kill_sig} "${check_update_pid}"
      ;;
   esac
}

unset -v trap_sigs trap_sig
trap_sigs="EXIT HUP INT TERM"

trap_func() {
   local trap_sig="${1-EXIT}"
   case "${trap_sig}" in
   0|EXIT) msg "exiting";;
   *) msg "received SIG${trap_sig}";;
   esac

   trap - ${trap_sigs}

   cleanup "${trap_sig}"

   case "${trap_sig}" in
   ""|0|EXIT) ;;
   *) kill -"${trap_sig}" "$$";;
   esac
}

for trap_sig in ${trap_sigs}
do
   trap "trap_func ${trap_sig}" ${trap_sig}
done

# Kill the given PID, ensuring that it is a child of this script ($$).
kill_child() {
   local pid
   pid="${1}"
   case $(($(ps -oppid= -p"${pid}" || :) + 0)) in
   $$) ;;
   *) return 1;;
   esac
   msg "killing pid ${pid}"
   kill "${pid}"
}

# Kill nodes that are direct child of this script (pid $$),
# i.e. run directly from main loop.
kill_node() {
   local pids pid delay

   msg "finding node processes that are our children"
   pids=$(
      ps axcwwo "pid=,ppid=,command=" |
      awk -v me=$$ '$2 == me && $3 == "harmony" { print $1; }'
   )
   msg "found node processes: ${pids:-"<none>"}"
   for pid in ${pids}
   do
      delay=0
      while kill_child ${pid}
      do
         sleep ${delay}
         delay=1
      done
      msg "pid ${pid} no longer running"
   done
}

{
   while ${loop}
   do
      msg "re-downloading binaries in 5m"
      sleep 300
      if any_new_binaries staging
      then
         msg "binaries did not change"
         continue
      fi
      while ! download_binaries staging
      do
         msg "staging download failed; retrying in 30s"
         sleep 30
      done
      if diff staging/harmony-checksums.txt harmony-checksums.txt
      then
         msg "binaries did not change"
         continue
      fi
      msg "binaries changed; moving from staging into main"
      (cd staging; exec mv harmony-checksums.txt "${BIN[@]}" ..) || continue
      msg "binaries updated, killing node to restart"
      kill_node
   done
} > harmony-update.out 2>&1 &
check_update_pid=$!

if [ -z "${blspass}" ]; then
   unset -v passphrase
   read -rsp "Enter passphrase for the BLS key file ${BLSKEYFILE}: " passphrase
   echo
elif [ ! -f "${blspass}" ]; then
   err 10 "can't find the ${blspass} file"
fi

while :
do
   msg "############### Running Harmony Process ###############"
   args=(
      -bootnodes "${BN_MA}"
      -ip "${PUB_IP}"
      -port "${NODE_PORT}"
      -is_genesis
      -blskey_file "${BLSKEYFILE}"
      -network_type="${network_type}"
      -dns_zone="${dns_zone}"
   )
   if ${public_rpc}; then
      args+=(
      -public_rpc
      )
   fi
# backward compatible with older harmony node software
   case "${node_type}" in
   explorer)
      args+=(
      -node_type="${node_type}"
      -shard_id="${shard_id}"
      )
      ;;
   esac
   case "${metrics}" in
   true)
      args+=(
         -metrics "${metrics}"
         -pushgateway_ip "${PUSHGATEWAY_IP}"
         -pushgateway_port "${PUSHGATEWAY_PORT}"
      )
      ;;
   esac
   case "$OS" in
   Darwin) ld_path_var=DYLD_FALLBACK_LIBRARY_PATH;;
   *) ld_path_var=LD_LIBRARY_PATH;;
   esac
   run() {
      env "${ld_path_var}=$(pwd)" ./harmony "${args[@]}" "${@}"
   }
   case "${blspass:+set}" in
   "") echo -n "${passphrase}" | run -blspass stdin;;
   *) run -blspass file:${blspass};;
   esac || msg "node process finished with status $?"
   ${loop} || break
   msg "restarting in 10s..."
   sleep 10
done
