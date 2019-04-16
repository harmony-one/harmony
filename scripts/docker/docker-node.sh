#!/bin/bash

DOCKER_IMAGE=harmonyone/node:master

function usage()
{
  echo "usage: $(basename $0) [-p base_port] account_id"
  exit 1
}

if [ -z "$(which docker)" ]; then
  echo "docker is not installed."
  echo "Please check https://docs.docker.com/install/ to get docker installed."
  exit 1
fi

port_base=

while getopts "p:" opt; do
  case "$opt" in
    p) port_base="$OPTARG";;
    *) usage;;
  esac
done

shift $(($OPTIND-1))

account_id=$1

if [ -z "$account_id" ]; then
  echo "Please provide account id"
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

port_rest=$(( $port_base - 3000 ))
port_rpc=$(( $port_base + 5555 ))
# Pull latest image
docker pull $DOCKER_IMAGE
# Stop running container
docker rm -v -f harmony-$account_id-$port_base
docker run -it -d \
  --name harmony-$account_id-$port_base \
  -p $port_base:$port_base -p $port_rest:$port_rest -p $port_rpc:$port_rpc \
  -e NODE_PORT=$port_base \
  -e NODE_ACCOUNT_ID=$account_id \
  --mount type=volume,source=data-$port_base,destination=/harmony/db \
  --mount type=volume,source=log-$port_base,destination=/harmony/log \
  $DOCKER_IMAGE

echo "Please run \`docker logs -f harmony-$account_id-$port_base\` to check console logs"

# vim: ai ts=2 sw=2 et sts=2 ft=sh
