TOP:=$(realpath ..)
export CGO_CFLAGS:=-I$(TOP)/bls/include -I$(TOP)/mcl/include -I/usr/local/opt/openssl/include
export CGO_LDFLAGS:=-L$(TOP)/bls/lib -L/usr/local/opt/openssl/lib
export LD_LIBRARY_PATH:=$(TOP)/bls/lib:$(TOP)/mcl/lib:/usr/local/opt/openssl/lib
export LIBRARY_PATH:=$(LD_LIBRARY_PATH)
export DYLD_FALLBACK_LIBRARY_PATH:=$(LD_LIBRARY_PATH)
export GO111MODULE:=on

.PHONY: all libs exe test

all: libs
	./scripts/go_executable_build.sh

libs:
	make -C $(TOP)/mcl -j4
	make -C $(TOP)/bls BLS_SWAP_G=1 -j4

exe:
	./scripts/go_executable_build.sh

test:
	./test/debug.sh
