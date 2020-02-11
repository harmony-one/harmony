#!/bin/bash

BLSKEY=
BLSPASS=

port_base=9000
tag=latest
db_dir=db

DOCKER_REPO=harmonyone/node
DOCKER_IMAGE=$DOCKER_REPO:$tag

function usage()
{
  cat << EOU

usage: $(basename $0) options blskey blspass

options:
  -t tag      : tag of the image, default: $tag
  -p base_port: base port, default: $port_base
  -n network  : network type
  -z dns_zone : dns zone
  -d db_dir   : harmony db directory
  -X "extra"  : extra parameters to docker 'run' script

  -k          : kill running node
  -h          : print this message

  blskey        : blskey file name, keyfile
  blspass       : blspass file name, passphase in file

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

while getopts "t:p:d:khX:" opt; do
  case "$opt" in
    t) tag="$OPTARG"
       DOCKER_IMAGE=$DOCKER_REPO:$tag;;
    p) port_base="$OPTARG";;
    d) db_dir="$OPTARG";;
    k) kill_only="true";;
    X) extra="$OPTARG";;
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

if [ "$port_base" -lt 4000 ]; then
  echo "port base cannot be less than 4000"
  exit 1
fi

if [ "$port_base" -gt 59900 ]; then
  echo "port base cannot be greater than 59900"
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

port_ss=$(( $port_base - 3000 ))
port_rpc=$(( $port_base + 500 ))
port_wss=$(( $port_base + 800 ))

# Pull latest image
echo "Pull latest node image"
docker pull $DOCKER_IMAGE >/dev/null

mkdir -p ${db_dir}/harmony_db_0
mkdir -p ${db_dir}/harmony_db_1
mkdir -p ${db_dir}/harmony_db_2
mkdir -p ${db_dir}/harmony_db_3

docker run -it -d \
  --name harmony-$tag-$port_base \
  -p $port_base:$port_base -p $port_ss:$port_ss -p $port_rpc:$port_rpc -p $port_wss:$port_wss \
  -e NODE_PORT=$port_base \
  -e NODE_BLSKEY=$BLSKEY \
  -e NODE_BLSPASS=$BLSPASS \
  -e NODE_EXTRA_OPTIONS="$extra" \
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
