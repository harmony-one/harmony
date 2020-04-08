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
	make -C $(TOP)/mcl -j8
	make -C $(TOP)/bls BLS_SWAP_G=1 -j8

exe:
	./scripts/go_executable_build.sh

race:
	./scripts/go_executable_build.sh -r

trace-pointer:
	./scripts/go_executable_build.sh -t

test:
	./test/debug.sh

linux_static:
	make -C $(TOP)/mcl -j8
	make -C $(TOP)/bls minimised_static BLS_SWAP_G=1 -j8
	./scripts/go_executable_build.sh -s
