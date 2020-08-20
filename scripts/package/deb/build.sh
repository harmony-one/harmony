#!/bin/bash

if [ $# != 1 ]; then
   echo "$0 version"
   exit
fi

VERSION=$1

fpm -s dir -t deb -n harmony -p harmony_VERSION_ARCH.deb -C harmony-${VERSION} \
 -v "$VERSION" \
 --license "MIT" \
 --vendor "Harmony Blockchain" \
 --category "net" \
 --no-depends \
 --no-auto-depends \
 --directories /etc/harmony \
 --directories /data/harmony \
 --architecture x86_64 \
 --maintainer "Leo Chen <leo@harmony.one>" \
 --description "Harmony is a sharded, fast finality, low fee, PoS public blockchain.\nThis package contains the validator node program for harmony blockchain." \
 --url "https://harmony.one" \
 --before-install scripts/preinst \
 --after-install scripts/postinst \
 --before-remove scripts/prerm \
 --after-remove scripts/postrm \
 --before-upgrade scripts/preup \
 --after-upgrade scripts/postup \
 --deb-changelog scripts/changelog \
 --deb-systemd-restart-after-upgrade
