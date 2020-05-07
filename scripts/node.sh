#!/usr/bin/env bash

version="v1 20200427.0"

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
      msg "you may use -S option to run as normal user"
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
   ### KERNEL TUNING ###
   # Increase size of file handles and inode cache
   sysctl -w fs.file-max=2097152
   # Do less swapping
   sysctl -w vm.swappiness=10
   sysctl -w vm.dirty_ratio=60
   sysctl -w vm.dirty_background_ratio=2
   # Sets the time before the kernel considers migrating a proccess to another core
   sysctl -w kernel.sched_migration_cost_ns=5000000
   ### GENERAL NETWORK SECURITY OPTIONS ###
   # Number of times SYNACKs for passive TCP connection.
   sysctl -w net.ipv4.tcp_synack_retries=2
   # Allowed local port range
   sysctl -w net.ipv4.ip_local_port_range='2000 65535'
   # Protect Against TCP Time-Wait
   sysctl -w net.ipv4.tcp_rfc1337=1
   # Control Syncookies
   sysctl -w net.ipv4.tcp_syncookies=1
   # Decrease the time default value for tcp_fin_timeout connection
   sysctl -w net.ipv4.tcp_fin_timeout=15
   # Decrease the time default value for connections to keep alive
   sysctl -w net.ipv4.tcp_keepalive_time=300
   sysctl -w net.ipv4.tcp_keepalive_probes=5
   sysctl -w net.ipv4.tcp_keepalive_intvl=15
   ### TUNING NETWORK PERFORMANCE ###
   # Default Socket Receive Buffer
   sysctl -w net.core.rmem_default=31457280
   # Maximum Socket Receive Buffer
   sysctl -w net.core.rmem_max=33554432
   # Default Socket Send Buffer
   sysctl -w net.core.wmem_default=31457280
   # Maximum Socket Send Buffer
   sysctl -w net.core.wmem_max=33554432
   # Increase number of incoming connections
   sysctl -w net.core.somaxconn=8096
   # Increase number of incoming connections backlog
   sysctl -w net.core.netdev_max_backlog=65536
   # Increase the maximum amount of option memory buffers
   sysctl -w net.core.optmem_max=25165824
   sysctl -w net.ipv4.tcp_max_syn_backlog=8192
   # Increase the maximum total buffer-space allocatable
   # This is measured in units of pages (4096 bytes)
   sysctl -w net.ipv4.tcp_mem='786432 1048576 26777216'
   sysctl -w net.ipv4.udp_mem='65536 131072 262144'
   # Increase the read-buffer space allocatable
   sysctl -w net.ipv4.tcp_rmem='8192 87380 33554432'
   sysctl -w net.ipv4.udp_rmem_min=16384
   # Increase the write-buffer-space allocatable
   sysctl -w net.ipv4.tcp_wmem='8192 65536 33554432'
   sysctl -w net.ipv4.udp_wmem_min=16384
   # Increase the tcp-time-wait buckets pool size to prevent simple DOS attacks
   sysctl -w net.ipv4.tcp_max_tw_buckets=1440000
   sysctl -w net.ipv4.tcp_tw_reuse=1
   sysctl -w net.ipv4.tcp_fastopen=3
   sysctl -w net.ipv4.tcp_window_scaling=1
   
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

function check_pkg_management
{
   if which yum > /dev/null; then
      PKG_INSTALL='sudo yum install -y'
      return
   fi
   if which apt-get > /dev/null; then
      PKG_INSTALL='sudo apt-get install -y'
      return
   fi
}

######## main #########
print_usage() {
   cat <<- ENDEND

usage: ${progname} [options]

options:
   -c             back up database/logs and start clean (not for mainnet)
                  (use only when directed by Harmony)
   -1             do not loop; run once and exit
   -h             print this help and exit
   -k KEYFILE     use the given BLS key file (default: autodetect)
   -s             run setup env only (must run as root)
   -S             run the ${progname} as non-root user (default: run as root)
   -p passfile    use the given BLS passphrase file
   -d             just download the Harmony binaries (default: off)
   -D             do not download Harmony binaries (default: download when start)
   -N network     join the given network (mainnet, testnet, staking, partner, stress, devnet, tnet; default: mainnet)
   -n port        specify the public base port of the node (default: 9000)
   -T nodetype    specify the node type (validator, explorer; default: validator)
   -i shardid     specify the shard id (valid only with explorer node; default: 1)
   -b             download harmony_db files from shard specified by -i <shardid> (default: off)
   -a dbfile      specify the db file to download (default:off)
   -U FOLDER      specify the upgrade folder to download binaries
   -P             enable public rpc end point (default:off)
   -v             print out the version of the node.sh
   -V             print out the version of the Harmony binary
   -z             run in staking mode
   -y             run in legacy, foundational-node mode (default)
   -Y             verify the signature of the downloaded binaries (default: off)
   -m minpeer     specify minpeers for bootstrap (default: 6)
   -M             support multi-key mode (default: off)
   -f blsfolder   folder that stores the bls keys and corresponding passphrases (default: ./.hmy/blskeys)
   -A             enable archival node mode (default: off)
   -B blacklist   specify file containing blacklisted accounts as a newline delimited file (default: ./.hmy/blacklist.txt)
   -r address     start a pprof profiling server listening on the specified address
   -I             use statically linked Harmony binary (default: true)
   -R tracefile   enable p2p trace using tracefile (default: off)

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

# start the node in a different port 9010
   ${progname} -n 9010

# multi-bls: place all keys/passphrases under .hmy/blskeys
# e.g. <blskey>.key and <blskey>.pass
   ${progname} -S -M 

# multi-bls: specify folder that contains bls keys
   ${progname} -S -M -f /home/xyz/myfolder

# multi-bls using default passphrase: place all keys under .hmy/blskeys
# supply passphrase file using -p option (single passphrase will be used for all bls keys)
   ${progname} -S -M -p blspass.txt

# multi-bls using user input passphrase: place all keys under .hmy/blskeys
# supply passphrase for each of the bls key file when prompted
   ${progname} -S -M

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

unset start_clean loop run_as_root blspass do_not_download download_only network node_type shard_id download_harmony_db db_file_to_dl
unset upgrade_rel public_rpc staking_mode pub_port multi_key blsfolder blacklist verify TRACEFILE minpeers
start_clean=false
loop=true
run_as_root=true
do_not_download=false
download_only=false
network=mainnet
node_type=validator
shard_id=-1
download_harmony_db=false
public_rpc=false
staking_mode=false
multi_key=false
blsfolder=./.hmy/blskeys
archival=false
blacklist=./.hmy/blacklist.txt
pprof=""
static=true
verify=false
minpeers=6
${BLSKEYFILE=}
${TRACEFILE=}

unset OPTIND OPTARG opt
OPTIND=1
while getopts :1chk:sSp:dDN:T:i:ba:U:PvVyzn:MAIB:r:Y:f:R:m: opt
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
   m) minpeers="${OPTARG}";;
   M) multi_key=true;;
   f) blsfolder="${OPTARG}";;
   N) network="${OPTARG}";;
   n) pub_port="${OPTARG}";;
   T) node_type="${OPTARG}";;
   i) shard_id="${OPTARG}";;
   I) static=true;;
   a) db_file_to_dl="${OPTARG}";;
   U) upgrade_rel="${OPTARG}";;
   P) public_rpc=true;;
   B) blacklist="${OPTARG}";;
   r) pprof="${OPTARG}";;
   v) msg "version: $version"
      exit 0 ;;
   V) LD_LIBRARY_PATH=. ./harmony -version
      exit 0 ;;
   Y) verify=true;;
   z) staking_mode=true;;
   y) staking_mode=false;;
   A) archival=true;;
   R) TRACEFILE="${OPTARG}";;
   *) err 70 "unhandled option -${OPTARG}";;  # EX_SOFTWARE
   esac
done
shift $((${OPTIND} - 1))

unset -v bootnodes REL network_type dns_zone syncdir

case "${node_type}" in
validator) ;;
explorer) archival=true;;
*)
   usage ;;
esac

case "${network}" in
mainnet)
  bootnodes=(
    /ip4/100.26.90.187/tcp/9874/p2p/Qmdfjtk6hPoyrH1zVD9PEH4zfWLo38dP2mDvvKXfh3tnEv
    /ip4/54.213.43.194/tcp/9874/p2p/QmZJJx6AdaoEkGLrYG4JeLCKeCKDjnFz2wfHNHxAqFSGA9
    /ip4/13.113.101.219/tcp/12019/p2p/QmQayinFSgMMw5cSpDUiD9pQ2WeP6WNmGxpZ6ou3mdVFJX
    /ip4/99.81.170.167/tcp/12019/p2p/QmRVbTpEYup8dSaURZfF6ByrMTSKa4UyUzJhSjahFzRqNj
  )
  REL=mainnet
  network_type=mainnet
  dns_zone=t.hmny.io
  syncdir=mainnet.min
  ;;
testnet)  # TODO: update Testnet configs once LRTN is upgraded
  bootnodes=(
    /ip4/54.218.73.167/tcp/9876/p2p/QmWBVCPXQmc2ULigm3b9ayCZa15gj25kywiQQwPhHCZeXj
    /ip4/18.232.171.117/tcp/9876/p2p/QmfJ71Eb7XTDs8hX2vPJ8un4L7b7RiDk6zCzWVxLXGA6MA
  )
  REL=testnet
  network_type=testnet
  dns_zone=p.hmny.io
  syncdir=lrtn
  ;;
tnet)
  bootnodes=(
    /ip4/54.86.126.90/tcp/9889/p2p/Qmdfjtk6hPoyrH1zVD9PEH4zfWLo38dP2mDvvKXfh3tnEv
    /ip4/52.40.84.2/tcp/9889/p2p/QmbPVwrqWsTYXq1RxGWcxx9SWaTUCfoo1wA6wmdbduWe29
  )
  REL=tnet
  network_type=testnet
  dns_zone=tn.hmny.io
  syncdir=tnet
  ;;
staking)
  bootnodes=(
    /ip4/54.86.126.90/tcp/9867/p2p/Qmdfjtk6hPoyrH1zVD9PEH4zfWLo38dP2mDvvKXfh3tnEv
    /ip4/52.40.84.2/tcp/9867/p2p/QmbPVwrqWsTYXq1RxGWcxx9SWaTUCfoo1wA6wmdbduWe29
  )
  REL=pangaea
  network_type=pangaea
  dns_zone=os.hmny.io
  syncdir=ostn
  ;;
partner)
  bootnodes=(
    /ip4/52.40.84.2/tcp/9800/p2p/QmbPVwrqWsTYXq1RxGWcxx9SWaTUCfoo1wA6wmdbduWe29
    /ip4/54.86.126.90/tcp/9800/p2p/Qmdfjtk6hPoyrH1zVD9PEH4zfWLo38dP2mDvvKXfh3tnEv
  )
  REL=partner
  network_type=partner
  dns_zone=ps.hmny.io
  syncdir=pstn
  ;;
stn|stress|stressnet)
  bootnodes=(
    /ip4/52.40.84.2/tcp/9842/p2p/QmbPVwrqWsTYXq1RxGWcxx9SWaTUCfoo1wA6wmdbduWe29
  )
  REL=stressnet
  network_type=stressnet
  dns_zone=stn.hmny.io
  syncdir=stn
  ;;
devnet)
  bootnodes=(
    /ip4/54.86.126.90/tcp/9870/p2p/Qmdfjtk6hPoyrH1zVD9PEH4zfWLo38dP2mDvvKXfh3tnEv
    /ip4/52.40.84.2/tcp/9870/p2p/QmbPVwrqWsTYXq1RxGWcxx9SWaTUCfoo1wA6wmdbduWe29
  )
  REL=devnet
  network_type=devnet
  dns_zone=pga.hmny.io
  syncdir=devnet
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
   FOLDER=release/darwin-x86_64/$REL
fi
if [ "$OS" == "Linux" ]; then
   FOLDER=release/linux-x86_64/$REL
   if [ "$static" == "true" ]; then
      FOLDER=${FOLDER}/static
   fi
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

verify_signature() {
   local dir file 
   dir="${1}"
   file="${dir}/${2}"
   sigfile="${dir}/${2}.sig"

   result=$(openssl dgst -sha256 -verify "${outdir}/harmony_pubkey.pem" -signature "${sigfile}" "${file}" 2>&1)
   echo ${result}
   if  [[ ${result} != "Verified OK" ]]; then
	   return 1
   fi
   return 0
}

download_binaries() {
   local outdir status
   ${do_not_download} && return 0
   outdir="${1}"
   mkdir -p "${outdir}"
   for bin in $(cut -c35- "${outdir}/md5sum.txt"); do
      status=0
      curl -sSf http://${BUCKET}.s3.amazonaws.com/${FOLDER}/${bin} -o "${outdir}/${bin}" || status=$?
      case "${status}" in
      0) ;;
      *)
         msg "cannot download ${bin} (status ${status})"
         return ${status}
         ;;
      esac

      if $verify; then
         curl -sSf http://${BUCKET}.s3.amazonaws.com/${FOLDER}/${bin}.sig -o "${outdir}/${bin}.sig" || status=$?
         case "${status}" in
         0) ;;
         *)
            msg "cannot download ${bin}.sig (status ${status})"
            return ${status}
            ;;
         esac
         verify_signature "${outdir}" "${bin}" || return $?
      fi
      verify_checksum "${outdir}" "${bin}" md5sum.txt || return $?
      msg "downloaded ${bin}"
   done
   chmod +x "${outdir}/harmony"
   (cd "${outdir}" && exec openssl sha256 $(cut -c35- md5sum.txt)) > "${outdir}/harmony-checksums.txt"
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

   url="http://${BUCKET}.s3.amazonaws.com/${FOLDER}/db/md5sum.txt"
   rm -f "${outdir}/md5sum.txt"
   if ! _curl_download $url "${outdir}" md5sum.txt; then
      err 70 "cannot download md5sum.txt"
   fi

   if [ -n "${file_to_dl}" ]; then
      if grep -q "${file_to_dl}" "${outdir}/md5sum.txt"; then
         url="http://${BUCKET}.s3.amazonaws.com/${FOLDER}/db/${file_to_dl}"
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
            url="http://${BUCKET}.s3.amazonaws.com/${FOLDER}/db/$file"
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

any_new_binaries() {
   local outdir
   ${do_not_download} && return 0
   outdir="${1}"
   mkdir -p "${outdir}"
   curl -L https://harmony.one/pubkey -o "${outdir}/harmony_pubkey.pem"
   if ! grep -q "BEGIN\ PUBLIC\ KEY" "${outdir}/harmony_pubkey.pem"; then
      msg "failed to downloaded harmony public signing key"
      return 1
   fi
   curl -sSf http://${BUCKET}.s3.amazonaws.com/${FOLDER}/md5sum.txt -o "${outdir}/md5sum.txt.new" || return $?
   if diff $outdir/md5sum.txt.new md5sum.txt
   then
      rm "${outdir}/md5sum.txt.new"
   else
      mv "${outdir}/md5sum.txt.new" "${outdir}/md5sum.txt"
      return 1
   fi
}

if ${download_only}; then
   if any_new_binaries staging
   then
      msg "binaries did not change in staging"
   else
      download_binaries staging || err 69 "download node software failed"
      msg "downloaded files are in staging direectory"
   fi
   exit 0
fi

if ${download_harmony_db}; then
   download_harmony_db_file "${shard_id}" "${db_file_to_dl}" || err 70 "download harmony_db file failed"
   exit 0
fi

if ${run_as_root}; then
   check_root
fi

# multi_key mode will use specified keys in .hmy/multikeys directory
if ! ${multi_key}; then
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
fi

if any_new_binaries .
then
   msg "binaries did not change"
else
   download_binaries . || err 69 "initial node software update failed"
fi

NODE_PORT=${pub_port:-9000}
PUB_IP=
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
check_pkg_management

unset -v BN_MA bn
for bn in "${bootnodes[@]}"
do
  BN_MA="${BN_MA+"${BN_MA},"}${bn}"
done

if [[ "${start_clean}" == "true" && "${network_type}" != "mainnet" ]]
then
   msg "cleaning up old database (-c)"
   # set a 2s timeout, and set its default return value to Y (true)
   read -t 2 -rp "Remove old database? (Y/n) " yesno
   yesno=${yesno:-Y}
   echo
   if [[ "$yesno" == "y" || "$yesno" == "Y" ]]; then
      unset -v backup_dir now
      now=$(date -u +%Y-%m-%dT%H:%M:%SZ)
      mkdir -p backups; rm -rf backups/*
      backup_dir=$(mktemp -d "backups/${now}.XXXXXX")
      mv -f harmony_db_* .dht* "${backup_dir}/" 2>/dev/null || :
   fi

   # install unzip as dependency of rclone
   if ! which unzip > /dev/null; then
      $PKG_INSTALL unzip
   fi
   # do rclone sync
   if ! which rclone > /dev/null; then
      msg "installing rclone to fast sync db"
      msg "curl https://rclone.org/install.sh | sudo bash"
      curl https://rclone.org/install.sh | sudo bash
      mkdir -p ~/.config/rclone
   fi
   if ! grep -q 'hmy' ~/.config/rclone/rclone.conf 2> /dev/null; then
      msg "adding [hmy] profile to rclone.conf"
      cat<<-EOT>>~/.config/rclone/rclone.conf
[hmy]
type = s3
provider = AWS
env_auth = false
region = us-west-1
acl = public-read
EOT
   fi
   msg "Syncing harmony_db_0"
   rclone sync -P hmy://pub.harmony.one/$syncdir/harmony_db_0 harmony_db_0
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

unset -v save_pass_file
save_pass_file=true
prompt_save=false
read_bls_pass() {
   for f in ${blsfolder}/*.key
   do
      if [ ! -f $f ]; then
         err 10 "could not find bls key file: $f"
      fi
      passfile=${blsfolder}/$(basename "${f%.*}").pass
      if [ ! -f "$passfile" ]; then
         unset -v passphrase
         read -rsp "Enter passphrase for the BLS key file $f: " passphrase
         echo ${passphrase} | tee $passfile
         echo "Passphrase is temporarely saved to: $passfile"
         prompt_save=true
      fi
   done
   if ${prompt_save} ; then
      while true
      do
         read -t 3 -rp "Do you wish to delete the saved passphrase files after successful start of node? (y|n):" yn
         yn=${yn:-Y}
         case $yn in
            [Yy]*) save_pass_file=false
            break;;
            [Nn]*) save_pass_file=true
            break;;
            *) sleep 1 && echo "Please answer yes (y|Y) or no (n|N)";;
         esac
      done
      prompt_save=false
   fi
}

rm_bls_pass() {
   if ! ${save_pass_file} ; then
      for f in ${blsfolder}/*.pass
      do
         if [ -f $f ]; then
            rm $f
         fi
      done
   fi
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
      (cd staging; exec mv harmony-checksums.txt $(cut -c35- md5sum.txt) ..) || continue
      msg "binaries updated, killing node to restart"
      kill_node
   done
} > harmony-update.out 2>&1 &
check_update_pid=$!

if ! ${multi_key}; then
   if [ -z "${blspass}" ]; then
      unset -v passphrase
      read -rsp "Enter passphrase for the BLS key file ${BLSKEYFILE}: " passphrase
      echo
   elif [ ! -f "${blspass}" ]; then
      err 10 "can't find the ${blspass} file"
   fi
else
   read_bls_pass
fi

while :
do
   msg "############### Running Harmony Process ###############"
   args=(
      -bootnodes "${BN_MA}"
      -ip "${PUB_IP}"
      -port "${NODE_PORT}"
      -network_type="${network_type}"
      -dns_zone="${dns_zone}"
      -blacklist="${blacklist}"
      -min_peers="${minpeers}"
   )
   args+=(
      -is_archival="${archival}"
   )
   if ! ${multi_key}; then
      args+=(
      -blskey_file "${BLSKEYFILE}"
      )
   fi
   if ${multi_key}; then
      args+=(
         -blsfolder "${blsfolder}"
      )
   fi
   if ${public_rpc}; then
      args+=(
      -public_rpc
      )
   fi
   if [ ! -z "${pprof}" ]; then
      args+=(
      -pprof "${pprof}"
      )
   fi
# backward compatible with older harmony node software
   case "${node_type}" in
   validator)
      case "${shard_id}" in
      ?*)
         args+=(
            -shard_id="${shard_id}"
         )
         if ${staking_mode}
         then
            args+=(-staking="${staking_mode}")
         fi
         ;;
      esac
      ;;
   explorer)
      args+=(
      -node_type="${node_type}"
      -shard_id="${shard_id}"
      )
      ;;
   esac
   case "${TRACEFILE}" in
      "") ;;
      *) msg "WARN: enabled p2p tracefile: $TRACEFILE. Be aware of the file size."
         export P2P_TRACEFILE=${TRACEFILE} ;;
   esac
   case "$OS" in
   Darwin) ld_path_var=DYLD_FALLBACK_LIBRARY_PATH;;
   *) ld_path_var=LD_LIBRARY_PATH;;
   esac
   run() {
      (sleep 30 && rm_bls_pass)&
      env "${ld_path_var}=$(pwd)" ./harmony "${args[@]}" "${@}"
   }
   case "${blspass:+set}" in
   "") echo -n "${passphrase}" | run -blspass stdin;;
   *) run -blspass file:${blspass};;
   esac || msg "node process finished with status $?"
   ${loop} || break
   msg "restarting in 10s..."
   save_pass_file=false
   rm_bls_pass
   sleep 10
   if ${multi_key}; then
      read_bls_pass
   fi
done

# vim: set expandtab:ts=3
