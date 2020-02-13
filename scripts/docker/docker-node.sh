#!/bin/bash

set -x

BLSKEY=
BLSPASS=

ports=9000,6000,9500,9800
port_base=9000
tag=latest
db_dir=db

DOCKER_REPO=harmonyone/node
DOCKER_IMAGE=$DOCKER_REPO:$tag

use_local=false

function usage()
{
  cat << EOU

usage: $(basename $0) [options] blskey blspass

options:
  -l                          : use local docker image (default: $use_local)
  -t tag                      : tag of the image, default: $tag
  -p base,sync,rpc,wss        : all port setting, default: $ports
  -n network                  : network type
  -z dns_zone                 : dns zone
  -d db_dir                   : harmony db directory

  -k                          : kill running node
  -h                          : print this message

  blskey                      : blskey file name, keyfile
  blspass                     : blspass file name, passphase in file

examples:

$(basename $0) -t test -p 9001 -d db blskey blspass

EOU

  exit 1
}

if [ -z "$(which docker)" ]; then
  echo "docker is not installed."
  echo "Please check https://docs.docker.com/install/ to get docker installed."
  exit 1
fi

kill_only=

while getopts "t:p:d:khl" opt; do
  case "$opt" in
    l) use_local=true;;
    t) tag="$OPTARG"
       DOCKER_IMAGE=$DOCKER_REPO:$tag;;
    p) ports="$OPTARG"
      ;;
    d) db_dir="$OPTARG";;
    k) kill_only="true";;
    *) usage;;
  esac
done

shift $(($OPTIND-1))

BLSKEY=$1
BLSPASS=$2

if [ -z "$BLSKEY" ]; then
  echo "Please provide blskey file."
  usage
fi

if [ -z "$BLSPASS" ]; then
  echo "Please provide blspass file."
  usage
fi

port_base=$(echo $ports | cut -f1 -d,)
port_sync=$(echo $ports | cut -f2 -d,)
port_rpc=$(echo $ports | cut -f3 -d,)
port_wss=$(echo $ports | cut -f4 -d,)

if [ "$port_base" -lt 8000 ]; then
  echo "port base cannot be less than 8000"
  exit 1
fi

if [ "$port_base" -gt 64000 ]; then
  echo "port base cannot be greater than 64000"
  exit 1
fi

if [ -n "$(docker ps -q -a -f name=^harmony-${tag}-${port_base}$)" ]; then
  echo "Stop node for tag: $tag, port: $port_base"
  docker rm -v -f harmony-${tag}-${port_base} >/dev/null
elif [ "$kill_only" = "true" ]; then
  echo "Cannot find exist node for port $port_base"
  exit 1
fi

if [ "$kill_only" = "true" ]; then
  exit
fi

if ! $use_local; then
# Pull latest image
   echo "Pull latest node image"
   docker pull $DOCKER_IMAGE >/dev/null
else
   echo "Use local docker image"
fi

mkdir -p ${db_dir}/harmony_db_0
mkdir -p ${db_dir}/harmony_db_1
mkdir -p ${db_dir}/harmony_db_2
mkdir -p ${db_dir}/harmony_db_3

echo 'Port mapping'
echo 9000 =\> $port_base
echo 6000 =\> $port_sync
echo 9500 =\> $port_rpc
echo 9800 =\> $port_wss

docker run -it -d \
  --name harmony-$tag-$port_base \
  -p 9000:$port_base -p 6000:$port_sync -p 9500:$port_rpc -p 9800:$port_wss \
  -e NODE_PORT=$port_base \
  -e NODE_BLSKEY=$BLSKEY \
  -e NODE_BLSPASS=$BLSPASS \
  -v $(realpath ${db_dir}/harmony_db_0):/harmony/harmony_db_0 \
  -v $(realpath ${db_dir}/harmony_db_1):/harmony/harmony_db_1 \
  -v $(realpath ${db_dir}/harmony_db_2):/harmony/harmony_db_2 \
  -v $(realpath ${db_dir}/harmony_db_3):/harmony/harmony_db_3 \
  -v $(realpath keys):/harmony/.hmy \
  -v $(realpath logs):/harmony/log \
  $DOCKER_IMAGE >/dev/null

echo
echo "======================================"
echo "Node for tag ($tag) (port $port_base) is running in container 'harmony-$tag-$port_base'"
echo
echo "To check console log, please run \`docker logs -f harmony-$tag-$port_base\`"
echo "To stop node, please run \`$0 -t $tag -p $port_base -k blskey blspass\`"
echo "======================================"

# vim: ai ts=2 sw=2 et sts=2 ft=sh
