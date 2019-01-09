# Harmony
[![Build Status](https://travis-ci.com/harmony-one/harmony.svg?token=DnoYvYiTAk7pqTo9XsTi&branch=master)](https://travis-ci.com/harmony-one/harmony)
<a href='https://github.com/jpoles1/gopherbadger' target='_blank'>![gopherbadger-tag-do-not-edit](https://img.shields.io/badge/Go%20Coverage-45%25-brightgreen.svg?longCache=true&style=flat)</a>
<a href="https://discord.gg/dKbK6M">![Discord](https://img.shields.io/discord/532383335348043777.svg)</a>


## Coding Guidelines

* In general, we follow [effective_go](https://golang.org/doc/effective_go.html)
* Code must adhere to the official [Go formatting guidelines](https://golang.org/doc/effective_go.html#formatting) (i.e. uses [gofmt](https://golang.org/cmd/gofmt/)).
* Code must be documented adhering to the official Go [commentary](https://golang.org/doc/effective_go.html#commentary) guidelines.


## Dev Environment Setup

```
export GOPATH=$HOME/<path_of_your_choice>

mkdir -p $HOME/<path_of_your_choice>/src/github.com/harmony-one

cd $HOME/<path_of_your_choice>/src/github.com/harmony-one

git clone git@github.com:harmony-one/harmony.git

cd harmony

go get ./...
```

## Usage

### Running local test
```
./test/deploy.sh ./test/configs/local_config1.txt
```

## Testing

Make sure you the following command and make sure everything passed before submitting your code.

```
./test_before_submit.sh
```

## Linting

Make sure you the following command and make sure everything passes golint.

```
./lint_before_submit.sh
```

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
