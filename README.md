# Harmony

[![Build Status](https://travis-ci.com/harmony-one/harmony.svg?token=DnoYvYiTAk7pqTo9XsTi&branch=master)](https://travis-ci.com/harmony-one/harmony)
<a href='https://github.com/jpoles1/gopherbadger' target='_blank'>![gopherbadger-tag-do-not-edit](https://img.shields.io/badge/Go%20Coverage-45%25-brightgreen.svg?longCache=true&style=flat)</a>
<a href="https://discord.gg/kdf8a6T">![Discord](https://img.shields.io/discord/532383335348043777.svg)</a>
[![Coverage Status](https://coveralls.io/repos/github/harmony-one/harmony/badge.svg?branch=master)](https://coveralls.io/github/harmony-one/harmony?branch=master)
[![Go Report Card](https://goreportcard.com/badge/github.com/harmony-one/harmony)](https://goreportcard.com/report/github.com/harmony-one/harmony)

# Play with the blockchain

## Running the harmony blockchain with docker

The easiest way to run our blockchain locally is to use docker.
If you are on `OS X`, then install docker via `brew`.

```
$ brew cask install docker
$ open /Applications/Docker.app
```

then you can run

```
$ docker run -it harmonyone/main:stable /bin/bash
```

You'll be in this current repository when the shell opens up, now let's catch up on any missing
commits (all the following commands assume you are in the running docker container)

### Bleeding edge master

```
$ git fetch
$ git reset --hard origin/master
```

### Mainnet release intended branch

```
$ git fetch
$ git checkout s3
$ git reset --hard origin/s3
```

And now run the local blockchain

```
$ test/debug.sh
```

## Using our hmy command line tool

Assuming that you got `test/debug.sh` running in your docker container, we can interact with
the running blockchain using our `hmy` command line tool. Its part of our
[go-sdk](https://github.com/harmony-one/go-sdk) repo, but our docker container already has it as
well and built.

Find your running container's ID, here's an example

```
$ docker container ls
CONTAINER ID        IMAGE                    COMMAND             CREATED             STATUS              PORTS               NAMES
62572a199bac        harmonyone/main:stable   "/bin/bash"         2 minutes ago       Up 2 minutes                            awesome_cohen
```

now lets get our second shell in the running container:

```
$ docker exec -it 62572a199bac /bin/bash
```

The container already comes with a prebuilt `hmy` that you can invoke anywhere, but lets go ahead
and build the latest version.

```
$ cd ../go-sdk
$ git fetch
$ git reset --hard origin/master
$ make
$ ./hmy version
Harmony (C) 2020. hmy, version v211-698b282 (@harmony.one 2020-01-17T00:58:51+0000)
```

Then checkout `./hmy cookbook` for example usages. The majority of commands output legal `JSON`,
here is one example:

```
root@62572a199bac:~/go/src/github.com/harmony-one/go-sdk# ./hmy blockchain latest-header
{
  "id": "0",
  "jsonrpc": "2.0",
  "result": {
    "blockHash": "0x34a8b155f90b8fc22342fc8b5d1c969ed836a2f666c506e4017b570dc337e88c",
    "blockNumber": 0,
    "epoch": 0,
    "lastCommitBitmap": "",
    "lastCommitSig": "000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
    "leader": "one1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqquzw7vz",
    "shardID": 0,
    "timestamp": "2019-06-28 15:00:00 +0000 UTC",
    "unixtime": 1561734000,
    "viewID": 0
  }
}
```

## Developing in the container

The docker container is a full harmony development environment and comes with emacs, vim, ag, tig
and other creature comforts. Most importantly, it already has the go environment with our C/C++
based library dependencies (`libbls` and `mcl`) setup correctly for you. You can also change go
versions easily, an example:

```
$ eval $(gimme 1.12.6)
```

Note that changing the go version might mean that dependencies won't work out right when trying to
run `test/debug.sh` again, get the correct environment again easily by `exec bash`.

## Installation Requirements for directly on your machine

GMP and OpenSSL

```bash
brew install gmp
brew install openssl
```

## Dev Environment Setup

The required go version is: **go1.12**

```bash
export GOPATH=$HOME/<path_of_your_choice>

mkdir -p $HOME/<path_of_your_choice>/src/github.com/harmony-one

cd $HOME/<path_of_your_choice>/src/github.com/harmony-one

git clone git@github.com:harmony-one/mcl.git
git clone git@github.com:harmony-one/bls.git
git clone git@github.com:harmony-one/harmony.git

cd harmony

make

```

Note: make sure to run `scripts/install_build_tools.sh`to make sure build tools are of correct versions.

## Build

If you want to bypass the Makefile:

```bash
export CGO_CFLAGS="-I$GOPATH/src/github.com/harmony-one/bls/include -I$GOPATH/src/github.com/harmony-one/mcl/include -I/usr/local/opt/openssl/include"
export CGO_LDFLAGS="-L$GOPATH/src/github.com/harmony-one/bls/lib -L/usr/local/opt/openssl/lib"
export LD_LIBRARY_PATH=$GOPATH/src/github.com/harmony-one/bls/lib:$GOPATH/src/github.com/harmony-one/mcl/lib:/usr/local/opt/openssl/lib
export LIBRARY_PATH=$LD_LIBRARY_PATH
export DYLD_FALLBACK_LIBRARY_PATH=$LD_LIBRARY_PATH
export GO111MODULE=on
```

Note : Some of our scripts require bash 4.x support, please [install bash 4.x](http://tldrdevnotes.com/bash-upgrade-3-4-macos) on MacOS X.

### Build all executables

You can run the script `./scripts/go_executable_build.sh` to build all the executables.

### Build individual executables

Harmony server / main node:

```bash
./scripts/go_executable_build.sh harmony

```

Wallet:

```bash
./scripts/go_executable_build.sh wallet
```

Tx Generator:

```bash
./scripts/go_executable_build.sh txgen
```

## Usage

You may build the src/harmony.go locally and run local test.

### Running local test

The debug.sh script calls test/deploy.sh script to create a local environment of Harmony blockchain devnet based on the configuration file.
The configuration file configures number of nodes and their IP/Port.
The script starts 2 shards and 7 nodes in each shard.

```bash
./test/debug.sh
```

### Test local blockchain

```bash
source scripts/setup_bls_build_flags.sh
./bin/wallet list
./bin/wallet -p local balances
```

### Terminate the local blockchain

```bash
./test/kill_nodes.sh
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
- Sharded P2P network and P2P gossiping
- FBFT (Fast Byzantine Fault Tolerance) Consensus with BLS multi-signature
- Consensus view-change protocol
- Account model and support for Solidity
- Cross-shard transaction
- VRF (Verifiable Random Function) and VDF (Verifiable Delay Function)
- Cross-links
- Information disposal algorithm using erasure encoding (to be integrated)
- Transaction generator for loadtesting
- Cuckoo-rule based resharding

### Features To Be Implemented

- EPoS staking mechanism
- Leader rotation

### Features Planned after Mainnet

- Integration with WASM
- Fast state synchronization
- Kademlia routing
