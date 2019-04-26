# Harmony

[![Build Status](https://travis-ci.com/harmony-one/harmony.svg?token=DnoYvYiTAk7pqTo9XsTi&branch=master)](https://travis-ci.com/harmony-one/harmony)
<a href='https://github.com/jpoles1/gopherbadger' target='_blank'>![gopherbadger-tag-do-not-edit](https://img.shields.io/badge/Go%20Coverage-45%25-brightgreen.svg?longCache=true&style=flat)</a>
<a href="https://discord.gg/kdf8a6T">![Discord](https://img.shields.io/discord/532383335348043777.svg)</a>
[![Coverage Status](https://coveralls.io/repos/github/harmony-one/harmony/badge.svg?branch=master)](https://coveralls.io/github/harmony-one/harmony?branch=master)
[![Go Report Card](https://goreportcard.com/badge/github.com/harmony-one/harmony)](https://goreportcard.com/report/github.com/harmony-one/harmony)

## Installation Requirements

GMP and OpenSSL

```bash
brew install gmp
brew install openssl
```

## Dev Environment Setup

The required go version is: **go1.12**

```bash
export GOPATH=$HOME/<path_of_your_choice>
export CGO_CFLAGS="-I$GOPATH/src/github.com/harmony-one/bls/include -I$GOPATH/src/github.com/harmony-one/mcl/include -I/usr/local/opt/openssl/include"
export CGO_LDFLAGS="-L$GOPATH/src/github.com/harmony-one/bls/lib -L/usr/local/opt/openssl/lib"
export LD_LIBRARY_PATH=$GOPATH/src/github.com/harmony-one/bls/lib:$GOPATH/src/github.com/harmony-one/mcl/lib:/usr/local/opt/openssl/lib
export LIBRARY_PATH=$LD_LIBRARY_PATH
export DYLD_FALLBACK_LIBRARY_PATH=$LD_LIBRARY_PATH

mkdir -p $HOME/<path_of_your_choice>/src/github.com/harmony-one

cd $HOME/<path_of_your_choice>/src/github.com/harmony-one

git clone git@github.com:harmony-one/mcl.git

cd mcl && make -j4 && cd ..

git clone git@github.com:harmony-one/bls.git

cd bls && make -j4 && cd ..

git clone git@github.com:harmony-one/harmony.git

cd harmony

go get ./...

git submodule update --init --recursive

```

## Build

Note : Some of our scripts require bash 4.x support, please [install bash 4.x](http://tldrdevnotes.com/bash-upgrade-3-4-macos) on MacOS X.
Make sure you set `export GO111MODULE=on`.

### Build all executables

You can  run the script `./scripts/go_executable_build.sh` to build all the executables.

### Build individual executables

Initialize BLS 
```
source scripts/setup_bls_build_flags.sh
```

Harmony server / main node:

```
go build -o bin/harmony cmd/harmony/main.go
```

Wallet:

```
go build -o bin/wallet cmd/client/wallet/main.go
```

Tx Generator:

```
go build -o bin/txgen cmd/client/txgen/main.go
```

## Usage

You may build the src/harmony.go locally and run local test.

### Running local test

The deploy.sh script creates a local environment of Harmony blockchain devnet based on the configuration file.
The configuration file configures number of nodes and their IP/Port.
The script starts one local beacon chain node, the blockchain nodes, and run a transactional generator program which generates and sends simulated transactions to the local blockchain.

```bash
./test/deploy.sh ./test/configs/ten-oneshard.txt
```

## Testing

Make sure you use the following command and make sure everything passed before submitting your code.

```bash
./test/test_before_submit.sh
```

## License

Harmony is licensed under the MIT License. See [`LICENSE`](LICENSE) file for
the terms and conditions.

Harmony includes third-party open source code. In general, a source subtree
with a `LICENSE` or `COPYRIGHT` file is from a third party, and our
modifications thereto are licensed under the same third-party open source
license.

Also please see [our Fiduciary License Agreement](FLA.md) if you are
contributing to the project. By your submission of your contribution to us, you
and we mutually agree to the terms and conditions of the agreement.

## Contributing To Harmony

See [`CONTRIBUTING`](CONTRIBUTING.md) for details.

## Development Status

### Features Done

- Fully sharded network with beacon chain and shard chains
- Cuckoo-rule based resharding
- Staking on beacon chain
- Distributed randomness generation with VRF and VDF (Proof-of-Concept VDF)
- Sharded P2P network and P2P gossiping
- FBFT (Fast Byzantine Fault Tolerance) Consensus with BLS multi-signature
- Account model and support for Solidity
- Simple wallet program
- Information disposal algorithm using erasure encoding (to be integrated)
- Blockchain explorer with performance report and transaction lookup
- Transaction generator for loadtesting

### Features To Be Implemented

- Secure VDF
- Consensus view-change protocol
- Cross-shard transaction

### Features Planned after Mainnet

- Integration with WASM
- Fast state synchronization
- Kademlia routing
