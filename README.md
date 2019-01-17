# Harmony
[![Build Status](https://travis-ci.com/harmony-one/harmony.svg?token=DnoYvYiTAk7pqTo9XsTi&branch=master)](https://travis-ci.com/harmony-one/harmony)
<a href='https://github.com/jpoles1/gopherbadger' target='_blank'>![gopherbadger-tag-do-not-edit](https://img.shields.io/badge/Go%20Coverage-45%25-brightgreen.svg?longCache=true&style=flat)</a>
<a href="https://discord.gg/kdf8a6T">![Discord](https://img.shields.io/discord/532383335348043777.svg)</a>


## Dev Environment Setup

```bash
export GOPATH=$HOME/<path_of_your_choice>

mkdir -p $HOME/<path_of_your_choice>/src/github.com/harmony-one

cd $HOME/<path_of_your_choice>/src/github.com/harmony-one

git clone git@github.com:harmony-one/harmony.git

cd harmony

go get ./...

git submodule update --init --recursive

```

## Build

Harmony server / main node:
```
go build -o bin/harmony cmd/harmony.go
```

Beacon node:
```
go build -o bin/beacon cmd/beaconchain/main.go
```

Wallet:
```
go build -o bin/wallet cmd/client/wallet/main.go
```

Tx Generator:
```
go build -o bin/txgen cmd/client/txgen/main.go
```

You can also run the script `./script/go_executable_build.sh` to build all the executables.

Some of our scripts require bash 4.x support, please [install bash 4.x](http://tldrdevnotes.com/bash-upgrade-3-4-macos) on MacOS X.

## Usage
You may build the src/harmony.go locally and run local test.

### Running local test
The deploy.sh script creates a local environment of Harmony blockchain devnet based on the configuration file.
The configuration file configures number of nodes and their IP/Port.
The script starts one local beacon chain node, the blockchain nodes, and run a transactional generator program which generates and sends simulated transactions to the local blockchain.

```bash
./test/deploy.sh ./test/configs/local_config1.txt
```

## Testing

Make sure you the following command and make sure everything passed before submitting your code.

```bash
./test/test_before_submit.sh
```

## License

Harmony is licensed under the MIT License.  See [`LICENSE`](LICENSE) file for
the terms and conditions.

Also please see [our Fiduciary License Agreement](FLA.md) if you are
contributing to the project.  By your submission of your contribution to us, you
and we mutually agree to the terms and conditions of the agreement.


## Contributing To Harmony

See [`CONTRIBUTING`](CONTRIBUTING.md) for details.

## Development Status

### Features Done

* Basic consensus protocol with O(n) complexity
* Basic validator server
* P2p network connection and unicast
* Account model and support for Solidity
* Simple wallet program
* Mock beacon chain with static sharding
* Information disposal algorithm using erasure encoding (to be integrated)
* Blockchain explorer with performance report and transaction lookup
* Transaction generator for loadtesting

### Features To Be Implemented

* Full beacon chain with multiple validators
* Resharding
* Staking on beacon chain
* Fast state synchronization
* Distributed randomness generation with VRF and VDF
* Kademlia routing
* P2p network and gossiping
* Full protocol of consensus with BLS multi-sig and view-change protocol
* Integration with WASM
* Cross-shard transaction
