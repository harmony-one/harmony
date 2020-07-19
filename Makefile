TOP:=$(realpath ..)
export CGO_CFLAGS:=-I$(TOP)/bls/include -I$(TOP)/mcl/include -I/usr/local/opt/openssl/include
export CGO_LDFLAGS:=-L$(TOP)/bls/lib -L/usr/local/opt/openssl/lib
export LD_LIBRARY_PATH:=$(TOP)/bls/lib:$(TOP)/mcl/lib:/usr/local/opt/openssl/lib
export LIBRARY_PATH:=$(LD_LIBRARY_PATH)
export DYLD_FALLBACK_LIBRARY_PATH:=$(LD_LIBRARY_PATH)
export GO111MODULE:=on

.PHONY: all help libs exe race trace-pointer debug debug-kill test test-go test-api test-api-attach linux_static

all: libs
	bash ./scripts/go_executable_build.sh -S

help:
	@echo "all - build the harmony binary & bootnode along with the MCL & BLS libs (if necessary)"
	@echo "libs - build only the MCL & BLS libs (if necessary) "
	@echo "exe - build the harmony binary & bootnode"
	@echo "race - build the harmony binary & bootnode with race condition checks"
	@echo "trace-pointer - build the harmony binary & bootnode with pointer analysis"
	@echo "debug - start a localnet with 2 shards (s0 rpc endpoint = localhost:9599; s1 rpc endpoint = localhost:9598)"
	@echo "debug-kill - force kill the localnet"
	@echo "clean - remove node files & logs created by localnet"
	@echo "test - run the entire test suite (go test & Node API test)"
	@echo "test-go - run the go test (with go lint, fmt, imports, mod, and generate checks)"
	@echo "test-api - run the Node API test"
	@echo "test-api-attach - attach onto the Node API testing docker container for inspection"
	@echo "linux_static - static build the harmony binary & bootnode along with the MCL & BLS libs (for linux)"

libs:
	make -C $(TOP)/mcl -j8
	make -C $(TOP)/bls BLS_SWAP_G=1 -j8

exe:
	bash ./scripts/go_executable_build.sh -S

race:
	bash ./scripts/go_executable_build.sh -r

trace-pointer:
	bash ./scripts/go_executable_build.sh -t

debug:
	bash ./test/debug.sh

debug-kill:
	bash ./test/kill_node.sh

clean:
	rm -rf ./tmp_log*
	rm -rf ./.dht*
	rm -rf ./db-*
	rm -f ./*.rlp

test:
	bash ./test/all.sh

test-go:
	bash ./test/go.sh

test-api:
	bash ./test/api.sh run

test-api-attach:
	bash ./test/api.sh attach

linux_static:
	make -C $(TOP)/mcl -j8
	make -C $(TOP)/bls minimised_static BLS_SWAP_G=1 -j8
	bash ./scripts/go_executable_build.sh -s
