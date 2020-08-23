TOP:=$(realpath ..)
export CGO_CFLAGS:=-I$(TOP)/bls/include -I$(TOP)/mcl/include -I/usr/local/opt/openssl/include
export CGO_LDFLAGS:=-L$(TOP)/bls/lib -L/usr/local/opt/openssl/lib
export LD_LIBRARY_PATH:=$(TOP)/bls/lib:$(TOP)/mcl/lib:/usr/local/opt/openssl/lib
export LIBRARY_PATH:=$(LD_LIBRARY_PATH)
export DYLD_FALLBACK_LIBRARY_PATH:=$(LD_LIBRARY_PATH)
export GO111MODULE:=on
PKGNAME=harmony
VERSION?=2.3.5
RPMBUILD=$(HOME)/rpmbuild
DEBBUILD=$(HOME)/debbuild
SHELL := bash

.PHONY: all help libs exe race trace-pointer debug debug-kill test test-go test-api test-api-attach linux_static deb rpm_init rpm_build rpm

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
	@echo "arm_static - static build the harmony binary & bootnode on ARM64 platform"
	@echo "rpm - build a harmony RPM pacakge"
	@echo "deb - build a harmony Debian pacakge (todo)"

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
	rm -rf ./latest
	rm -f ./*.rlp
	rm -rf ~/rpmbuild

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

arm_static:
	go mod edit --require=github.com/ethereum/go-ethereum@v1.8.28
	go mod edit -replace github.com/ethereum/go-ethereum=$(GOPATH)/src/github.com/ethereum/go-ethereum
	make -C $(TOP)/mcl -j8
	make -C $(TOP)/bls minimised_static BLS_SWAP_G=1 -j8
	bash ./scripts/go_executable_build.sh -a arm64 -s
	git checkout go.mod

deb_init:
	rm -rf $(DEBBUILD)
	mkdir -p $(DEBBUILD)/$(PKGNAME)-$(VERSION)/{etc/systemd/system,usr/sbin,etc/sysctl.d,etc/harmony}
	cp -f bin/harmony $(DEBBUILD)/$(PKGNAME)-$(VERSION)/usr/sbin/
	bin/harmony dumpconfig $(DEBBUILD)/$(PKGNAME)-$(VERSION)/etc/harmony/harmony.conf
	cp -f scripts/package/rclone.conf $(DEBBUILD)/$(PKGNAME)-$(VERSION)/etc/harmony/
	cp -f scripts/package/harmony.service $(DEBBUILD)/$(PKGNAME)-$(VERSION)/etc/systemd/system/
	cp -f scripts/package/harmony-setup.sh $(DEBBUILD)/$(PKGNAME)-$(VERSION)/usr/sbin/
	cp -f scripts/package/harmony-rclone.sh $(DEBBUILD)/$(PKGNAME)-$(VERSION)/usr/sbin/
	cp -f scripts/package/harmony-sysctl.conf $(DEBBUILD)/$(PKGNAME)-$(VERSION)/etc/sysctl.d/99-harmony.conf
	cp -r scripts/package/deb/DEBIAN $(DEBBUILD)/$(PKGNAME)-$(VERSION)
	VER=$(VERSION) scripts/package/templater.sh scripts/package/deb/DEBIAN/control > $(DEBBUILD)/$(PKGNAME)-$(VERSION)/DEBIAN/control

deb_build:
	(cd $(DEBBUILD); dpkg-deb --build $(PKGNAME)-$(VERSION)/)

deb: deb_init deb_build

rpm_init:
	rm -rf $(RPMBUILD)
	mkdir -p $(RPMBUILD)/{SOURCES,SPECS,BUILD,RPMS,BUILDROOT,SRPMS}
	mkdir -p $(RPMBUILD)/SOURCES/$(PKGNAME)-$(VERSION)
	cp -f bin/harmony $(RPMBUILD)/SOURCES/$(PKGNAME)-$(VERSION)
	bin/harmony dumpconfig $(RPMBUILD)/SOURCES/$(PKGNAME)-$(VERSION)/harmony.conf
	cp -f scripts/package/harmony.service $(RPMBUILD)/SOURCES/$(PKGNAME)-$(VERSION)
	cp -f scripts/package/harmony-setup.sh $(RPMBUILD)/SOURCES/$(PKGNAME)-$(VERSION)
	cp -f scripts/package/harmony-rclone.sh $(RPMBUILD)/SOURCES/$(PKGNAME)-$(VERSION)
	cp -f scripts/package/rclone.conf $(RPMBUILD)/SOURCES/$(PKGNAME)-$(VERSION)
	cp -f scripts/package/harmony-sysctl.conf $(RPMBUILD)/SOURCES/$(PKGNAME)-$(VERSION)
	VER=$(VERSION) scripts/package/templater.sh scripts/package/rpm/harmony.spec > $(RPMBUILD)/SPECS/harmony.spec
	(cd $(RPMBUILD)/SOURCES; tar cvf $(PKGNAME)-$(VERSION).tar $(PKGNAME)-$(VERSION))

rpm_build:
	rpmbuild --target x86_64 -bb $(RPMBUILD)/SPECS/harmony.spec

rpm: rpm_init rpm_build
	rpm --addsign $(RPMBUILD)/RPMS/x86_64/$(PKGNAME)-$(VERSION)-0.x86_64.rpm

rpmpub_dev: rpm
	./scripts/package/publish-repo.sh -p dev -t rpm -s $(RPMBUILD)

rpmpub_prod: rpm
	./scripts/package/publish-repo.sh -p prod -t rpm -s $(RPMBUILD)
