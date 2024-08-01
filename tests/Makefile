.PHONY: install build build-cached upload upload-test test-docker help run

help:
	@echo "install - install dependencies for running tests without docker"
	@echo "build - build the docker image"
	@echo "build-cached - build the docker image with cache"
	@echo "upload-test - test the docker image before uploading to dockerhub"
	@echo "upload - upload the docker image to dockerhub (require credentials)"
	@echo "test - run all localnet tests as it would run inside the docker container"
	@echo "test-docker - run all localnet tests inside the docker container"
	@echo "rosetta-test-run - run the rosetta tests as it would run inside the docker container"
	@echo "rpc-test-run - run the localnet rpc tests as it would run inside the docker container"
	@echo "rpc-test-debug - run the rpc test suite on a running localnet"

install:
	python3 -m pip install -r requirements.txt

build:
	docker build --pull -t harmonyone/localnet-test . --no-cache

build-cached:
	docker build -t harmonyone/localnet-test .

upload-test: build
	bash ./scripts/release_test.sh

upload: 
	docker push harmonyone/localnet-test

test:
	bash ./scripts/run.sh -k

test-docker:
	docker run --name harmony-tests --rm -it -v "$(shell go env GOPATH)"/src/github.com/harmony-one/harmony:/go/src/github.com/harmony-one/harmony harmonyone/localnet-test

rosetta-test-run:
	bash ./scripts/run.sh -r -k

rpc-test-run:
	bash ./scripts/run.sh -n -k

rpc-test-debug:
	python3 -u -m py.test -v -r s -s rpc_tests -x -n 8

