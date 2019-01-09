# Harmony
[![Build Status](https://travis-ci.com/harmony-one/harmony.svg?token=DnoYvYiTAk7pqTo9XsTi&branch=master)](https://travis-ci.com/harmony-one/harmony)
<a href='https://github.com/jpoles1/gopherbadger' target='_blank'>![gopherbadger-tag-do-not-edit](https://img.shields.io/badge/Go%20Coverage-45%25-brightgreen.svg?longCache=true&style=flat)</a>


## Coding Guidelines

* In general, we follow [effective_go](https://golang.org/doc/effective_go.html)
* Code must adhere to the official [Go formatting guidelines](https://golang.org/doc/effective_go.html#formatting) (i.e. uses [gofmt](https://golang.org/cmd/gofmt/)).
* Code must be documented adhering to the official Go [commentary](https://golang.org/doc/effective_go.html#commentary) guidelines.


## Dev Environment Setup

```bash
export GOPATH=$HOME/<path_of_your_choice>

mkdir -p $HOME/<path_of_your_choice>/src/github.com/harmony-one

cd $HOME/<path_of_your_choice>/src/github.com/harmony-one

git clone git@github.com:harmony-one/harmony.git

cd harmony

go get ./...
```

## Usage
You may build the benchmark.go locally and run local test.

Some of our scripts require bash 4.x support, please [install bash 4.x](http://tldrdevnotes.com/bash-upgrade-3-4-macos) on MacOS X.

### Running local test
The deploy.sh script creates a local environment of Harmony blockchain devnet based on the configuration file.
The configuration file configures number of nodes and their IP/Port.
The script starts one local beacon chain node, the blockchain nodes, and run a transactional generator program which generates and sends simulated trnsactions to the local blockchain.

```bash
./test/deploy.sh ./test/configs/local_config1.txt
```

## Testing

Make sure you the following command and make sure everything passed before submitting your code.

```bash
./test/test_before_submit.sh
```

## Pull Request (PR)

This [github document](https://help.github.com/articles/creating-a-pull-request/) provides some guidance on how to create a pull request in github.

### PR requirement
To pursue engineering excellence, we have insisted on the highest stardard on the quality of each PR.

* For each PR, please run [golint](https://github.com/golang/lint), [gofmt](https://golang.org/cmd/gofmt/), to fix the basic issues/warnings.
* Make sure you understand [How to Write a Git Commit Message](https://chris.beams.io/posts/git-commit/).
* Add a [Test] section in every PR detailing on your test process and results. If the test log is too long, please include a link to [gist](https://gist.github.com/) and add the link to the PR.

### Typical workflow example
The best practice is to reorder and squash your local commits before the PR submission to create an atomic and self-contained PR.
This [book chapter](https://git-scm.com/book/en/v2/Git-Tools-Rewriting-History) provides detailed explanation and guidance on how to rewrite the local git history.

For exampple, a typical workflow is like the following.
```bash
# assuming you are working on a fix of bug1, and use a local branch called "fixes_of_bug1".

git clone https://github.com/harmony-one/harmony
cd harmony

# create a local branch to keep track of the origin/master
git branch fixes_of_bug1 origin/master
git checkout fixes_of_bug_1

# make changes, build, test locally, commit changes locally
# don't forget to squash or rearrange your commits using "git rebase -i"
git rebase -i origin/master

# rebase your change on the top of the tree
git pull --rebase

# push your branch and create a PR
git push origin fixes_of_bug_1:pr_fixes_of_bug_1
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
