#!/usr/bin/env bash

# fail on errors
set -e


ROOT=$(dirname $(readlink -f "$0"))
export GOPATH=${ROOT}/.build

git submodule update --init --recursive

echo "Building in .build/ subdir"
echo "GOPATH=$GOPATH"

CGO_CFLAGS="-I$GOPATH/src/github.com/harmony-one/bls/include -I$GOPATH/src/github.com/harmony-one/mcl/include -I/usr/local/opt/openssl/include"
CGO_LDFLAGS="-L$GOPATH/src/github.com/harmony-one/bls/lib -L/usr/local/opt/openssl/lib"
LD_LIBRARY_PATH=$GOPATH/src/github.com/harmony-one/bls/lib:$GOPATH/src/github.com/harmony-one/mcl/lib:/usr/local/opt/openssl/lib
LIBRARY_PATH=$LD_LIBRARY_PATH
DYLD_FALLBACK_LIBRARY_PATH=$LD_LIBRARY_PATH

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

