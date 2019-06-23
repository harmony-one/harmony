#!/bin/bash

DOCKER_IMAGE=harmonyone/node:R3

function usage()
{
  echo "usage: $(basename $0) [-p base_port] [-k] account_id"
  echo "  -p base_port: base port, default; 9000"
  echo "  -k          : kill running node"
  exit 1
}

if [ -z "$(which docker)" ]; then
  echo "docker is not installed."
  echo "Please check https://docs.docker.com/install/ to get docker installed."
  exit 1
fi

port_base=
kill_only=

while getopts "p:k" opt; do
  case "$opt" in
    p) port_base="$OPTARG";;
    k) kill_only="true";;
    *) usage;;
  esac
done

shift $(($OPTIND-1))

account_id=$1

if [ -z "$account_id" ]; then
  echo "Please provide account index.  Valid ranges are 44-49, 94-99, 144-149, 194-199."
  echo "Please contact us in #nodes channel of discord if not sure which one to use."
  usage
fi

if [ -z "$port_base" ]; then
  echo "Using default port: 9000"
  port_base=9000
fi

if [ "$port_base" -lt 4000 ]; then
  echo "port base cannot be less than 4000"
  exit 1
fi

if [ "$port_base" -gt 59900 ]; then
  echo "port base cannot be greater than 59900"
  exit 1
fi

if [ -n "$(docker ps -q -a -f name=^harmony-$account_id-$port_base$)" ]; then
  echo "Stop node for account id: $account_id (port $port_base)"
  docker rm -v -f harmony-$account_id-$port_base >/dev/null
elif [ "$kill_only" = "true" ]; then
  echo "Cannot find exist node for account id: $account_id (port $port_base)"
  exit 1
fi

if [ "$kill_only" = "true" ]; then
  exit
fi

port_rest=$(( $port_base - 3000 ))
port_rpc=$(( $port_base + 5555 ))

# Pull latest image
echo "Pull latest node image"
docker pull $DOCKER_IMAGE >/dev/null

docker run -it -d \
  --name harmony-$account_id-$port_base \
  -p $port_base:$port_base -p $port_rest:$port_rest -p $port_rpc:$port_rpc \
  -e NODE_PORT=$port_base \
  -e NODE_ACCOUNT_ID=$account_id \
  --mount type=volume,source=db-$account_id-$port_base,destination=/harmony/db \
  --mount type=volume,source=log-$account_id-$port_base,destination=/harmony/log \
  $DOCKER_IMAGE >/dev/null


echo
echo "======================================"
echo "Node for account $account_id (port $port_base) is running in container 'harmony-$account_id-$port_base'"
echo
echo "To check console log, please run \`docker logs -f harmony-$account_id-$port_base\`"
echo "To stop node, please run \`$0 -k -p $port_base $account_id\`"
echo "======================================"

# vim: ai ts=2 sw=2 et sts=2 ft=sh
