#!/usr/bin/env bash

# fail on errors
set -e


ROOT=$(dirname $(readlink -f "$0"))
export GOPATH=${ROOT}/.build

git submodule update --init --recursive

echo "Building in .build/ subdir"
echo "GOPATH=$GOPATH"

source ./scripts/setup_bls_build_flags.sh

MCL=$GOPATH/src/github.com/harmony-one/mcl
echo "... building mcl"
if [ ! -d "$MCL" ]; then
    git clone https://github.com/harmony-one/mcl.git $MCL
    (cd $MCL && make -j4)
fi

BLS=$GOPATH/src/github.com/harmony-one/bls
echo "... building BLS"
if [ ! -d "$BLS" ]; then
    git clone https://github.com/harmony-one/bls.git $BLS
    (cd $BLS && make -j4)
fi


echo "... building Harmony"
go get -v ./...

