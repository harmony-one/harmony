# Harmony

[![Build Status](https://travis-ci.com/harmony-one/harmony.svg?branch=main)](https://travis-ci.com/harmony-one/harmony)
![gopherbadger-tag-do-not-edit](https://img.shields.io/badge/Go%20Coverage-45%25-brightgreen.svg?longCache=true&style=flat)
![Discord](https://img.shields.io/discord/532383335348043777.svg)
[![Github Action](https://github.com/harmony-one/harmony/actions/workflows/ci.yaml/badge.svg?event=push)](https://github.com/harmony-one/harmony/actions/workflows/ci.yaml)
[![Go Report Card](https://goreportcard.com/badge/github.com/harmony-one/harmony)](https://goreportcard.com/report/github.com/harmony-one/harmony)

## General Documentation

https://docs.harmony.one

## API Guide

http://api.hmny.io/

## Requirements

### **Go 1.19**
### **GMP and OpenSSL**

On macOS:
```bash
brew install gmp
brew install openssl
sudo ln -sf /usr/local/opt/openssl@1.1 /usr/local/opt/openssl
```
On Linux (Ubuntu)
```bash
sudo apt install libgmp-dev  libssl-dev  make gcc g++
```
On Linux (Cent OS / Amazon Linux 2)
```bash
sudo yum install glibc-static gmp-devel gmp-static openssl-libs openssl-static gcc-c++
```
### **Docker** (for testing)

On macOS:
```bash
brew install --cask docker
open /Applications/Docker.app
```
On Linux, reference official documentation [here](https://docs.docker.com/engine/install/).
### **Bash 4+**

For macOS, you can reference this [guide](http://tldrdevnotes.com/bash-upgrade-3-4-macos). For Linux, you can reference this [guide](https://fossbytes.com/installing-gnu-bash-4-4-linux-distros/).

## Dev Environment

**Most repos from [harmony-one](https://github.com/harmony-one) assumes the GOPATH convention. More information [here](https://github.com/golang/go/wiki/GOPATH).**

### First Install
Clone and set up all of the repos with the following set of commands:

1. Create the appropriate directories:
```bash
mkdir -p $(go env GOPATH)/src/github.com/harmony-one
cd $(go env GOPATH)/src/github.com/harmony-one
```
> If you get 'unknown command' or something along those lines, make sure to install [golang](https://golang.org/doc/install) first.

2. Clone this repo & dependent repos.
```bash
git clone https://github.com/harmony-one/mcl.git
git clone https://github.com/harmony-one/bls.git
git clone https://github.com/harmony-one/harmony.git
cd harmony
```

3. Build the harmony binary & dependent libs
```
go mod tidy
make
```
> Run `bash scripts/install_build_tools.sh` to ensure build tools are of correct versions.
> If you get 'missing go.sum entry for module providing package <package_name>', run `go mod tidy`.

## Dev Docker Image

Included in this repo is a Dockerfile that has a full harmony development environment and
comes with emacs, vim, ag, tig and other creature comforts. Most importantly, it already has the go environment
with our C/C++ based library dependencies (`libbls` and `mcl`) set up correctly for you.

You can build the docker image for yourself with the following commands:
```bash
cd $(go env GOPATH)/src/github.com/harmony-one/harmony
make clean
docker build -t harmony .
```
> If your build machine has an ARM-based chip, like Apple silicon (M1), the image is built for `linux/arm64` by default. To build for `x86_64`, apply the `--platform` arg like so:
> ```bash
> docker build --platform linux/amd64 -t harmony .
> ```
> Learn more about the `--platform` arg and multi-CPU architecture support, [here](https://docs.docker.com/engine/reference/builder/#automatic-platform-args-in-the-global-scope) and [here](https://docs.docker.com/desktop/multi-arch/).



Then you can start your docker container with the following command:
```bash
docker run --rm --name harmony -it -v "$(go env GOPATH)/src/github.com/harmony-one/harmony:/root/go/src/github.com/harmony-one/harmony" harmony /bin/bash
```
> Note that the harmony repo will be shared between your docker container and your host machine. However, everything else in the docker container will be ephemeral.

If you need to open another shell, just do:
```bash
docker exec -it harmony /bin/bash
```

Learn more about docker [here](https://docker-curriculum.com/).

## Build

The `make` command should automatically build the Harmony binary & all dependent libs.

However, if you wish to bypass the Makefile, first export the build flags:
```bash
export CGO_CFLAGS="-I$GOPATH/src/github.com/harmony-one/bls/include -I$GOPATH/src/github.com/harmony-one/mcl/include -I/opt/homebrew/opt/openssl@1.1/include"
export CGO_LDFLAGS="-L$GOPATH/src/github.com/harmony-one/bls/lib -L/opt/homebrew/opt/openssl@1.1/lib"
export LD_LIBRARY_PATH=$GOPATH/src/github.com/harmony-one/bls/lib:$GOPATH/src/github.com/harmony-one/mcl/lib:/opt/homebrew/opt/openssl@1.1/lib
export LIBRARY_PATH=$LD_LIBRARY_PATH
export DYLD_FALLBACK_LIBRARY_PATH=$LD_LIBRARY_PATH
export GO111MODULE=on
```

Then you can build all executables with the following command:
```bash
bash ./scripts/go_executable_build.sh -S
```
> Reference `bash ./scripts/go_executable_build.sh -h` for more build options

## Debugging

One can start a local network (a.k.a localnet) with your current code using the following command:
```bash
make debug
```
> This localnet has 2 shards, with 11 nodes on shard 0 (+1 explorer node) and 10 nodes on shard 0 (+1 explorer node).
>
> The shard 0 endpoint will be on the explorer at `http://localhost:9599`. The shard 1 endpoint will be on the explorer at `http://localhost:9598`.
>
> You can view the localnet configuration at `/test/configs/local-resharding.txt`. The fields for the config are (space-delimited & in order) `ip`, `port`, `mode`, `bls_pub_key`, and `shard` (optional).

One can force kill the local network with the following command:
```bash
make debug-kill
```
> You can view all make commands with `make help`

## Testing

To keep things consistent, we have a docker image to run all tests. **These are the same tests ran on the pull request checks**.

Note that all test Docker containers bind several ports to the host machine for your convenience. The ports are:
* `9500` - Shard 0 RPC for a validator
* `9501` - Shard 1 RPC for a validator
* `9599` - Shard 0 RPC for an explorer
* `9598` - Shard 1 RPC for an explorer
* `9799` - Shard 0 Rosetta (for an explorer)
* `9798` - Shard 1 Rosetta (for an explorer)
* `9899` - Shard 0 WS for an explorer
* `9898` - Shard 1 WS for an explorer
> This allows you to use curl, hmy CLI, postman, rosetta-cli, etc... on your host machine to play with or probe the localnet that was used for the test.

### Go tests
To run this test, do:
```bash
make test-go
```
This test runs the go tests along with go lint, go fmt, go imports, go mod, and go generate checks.

### RPC tests
To run this test, do:
```bash
make test-rpc
```
This test starts a localnet (within the Docker container), **ensures it reaches a consensus**, and runs a series of tests to ensure correct RPC behavior.
This test also acts as a preliminary integration test (more thorough tests are done on the testnets).
> The tests ran by this command can be found [here](https://github.com/harmony-one/harmony-test/tree/master/localnet).

If you wish to debug further with the localnet after the tests are done, open a new shell and run:
```bash
make test-rpc-attach
```
> This will open a shell in the docker container that is running the Node API tests.
>
> Note that the docker container has the [Harmony CLI](https://docs.harmony.one/home/wallets/harmony-cli) on path,
> therefore you can use that to debug if needed. For example, one could do `hmy blockchain latest-headers` to check
> the current block height of localnet. Reference the documentation for the CLI [here](https://docs.harmony.one/home/wallets/harmony-cli)
> for more details & commands.

### Rosetta tests
To run this test, do:
```bash
make test-rosetta
```
This test starts a localnet (within the Docker container), **ensures it reaches a consensus**, and runs the Construction & Data API checks using the [rosetta-cli](https://github.com/coinbase/rosetta-cli).
This test also acts as a preliminary integration test (more thorough tests are done on the testnets).
> The config for this test can be found [here](https://github.com/harmony-one/harmony-test/blob/master/localnet/configs/localnet_rosetta_test_s0.json) & [here](https://github.com/harmony-one/harmony-test/blob/master/localnet/configs/localnet_rosetta_test_s1.json)

Similar to the RPC tests, if you wish to debug further with the localnet after the tests are done, open a new shell and run:
```bash
make test-rosetta-attach
```

## License

Harmony is licensed under GNU Lesser General Public License v3.0. See [`LICENSE`](LICENSE) file for
the terms and conditions.

Harmony includes third-party open-source code. In general, a source subtree
with a `LICENSE` or `COPYRIGHT` file is from a third party, and our
modifications thereto are licensed under the same third-party open source
license.

Also please see [our Fiduciary License Agreement](FLA.md) if you are
contributing to the project. By your submission of your contribution to us, you
and we mutually agree to the terms and conditions of the agreement.

## Contributing To Harmony

See [`CONTRIBUTING`](CONTRIBUTING.md) for details.

## Development Status

### Finished Features

- Fully sharded network with beacon chain and shard chains
- Sharded P2P network and P2P gossiping
- FBFT (Fast Byzantine Fault Tolerance) Consensus with BLS multi-signature
- Consensus view-change protocol
- Account model and support for Solidity
- Cross-shard transaction
- VRF (Verifiable Random Function) and VDF (Verifiable Delay Function)
- Cross-links
- EPoS staking mechanism
- Kademlia routing

### Planned Features

- Resharding
- Integration with WASM
- Fast state synchronization
- Auditable privacy asset using ZK proof

