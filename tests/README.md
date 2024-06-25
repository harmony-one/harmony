# Localnet tests

Tools wrapped in a Dockerfile to **build**, **deploy** and **test** a localnet.

## Assumptions
* The main harmony repo follows the GO-PATH convention. So the path to said repo must be: 
```bash
$(go env GOPATH)/src/github.com/harmony-one/harmony
```
* This repo follows the GO-PATH convention. So the path to said repo must be:
```bash
$(go env GOPATH)/src/github.com/harmony-one/harmony-test
```
* No extra transactions on the localnet are done prior to running the pytest in `./tests` or execution of the rosetta integration test.
* The localnet faucet address is `one1zksj3evekayy90xt4psrz8h6j2v3hla4qwz4ur` 

## Requirements (dev testing - no docker)
* Go
* jq
* Python 3.6+
* Pip3
* Python lib requirements from `requirements.txt`
> You can install the libs with the following command: `make install`
* The [rosetta-cli](https://github.com/coinbase/rosetta-cli) installed & on path (`$PATH`)

## Create a python venv
> This step is **optional** but *recommended* to not pollute your local Python installation.
```
 python3 -m venv .venv
 source .venv/bin/activate
```

## Build & run tests
* To build the docker image locally, do:
```bash
make build
```
* If you wish to build the docker image with a custom tag, do:
```bash
docker build -t "$TAG_NAME" .
``` 
> Note that you have to be in the same directory as the `Dockerfile`

* To release the docker image to Dockerhub, do:
```bash
make upload
```

* To run the test (start localnet, test, and teardown) outside of the docker image (for dev testing), do:
```bash
make test
```
> This will test whatever is in the main repo (following GO-PATH convention). 

* To run the localnet tests with docker using your local harmony repository, do:
```bash
make test-docker
```

> This will test whatever is in the main repo (following GO-PATH convention).

## Release

To release the docker image (used by PR test & main repo), first make sure it passes the release test:
```bash
make upload-test
```
> This will run the tests, wrapped in a docker img, 10 times and expects all 10 times to be successful.

Then upload with (assuming you have proper docker-hub credentials):
```bash
make upload
```
