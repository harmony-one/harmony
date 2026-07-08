#!/bin/sh
# run go generate on .go files under source control; group by dir (package).
unset -v progdir
case "${0}" in
*/*) progdir="${0%/*}";;
*) progdir=.;;
esac
git grep -l '^//go:generate ' -- '*.go' | \
	PROTOC_IMAGE="harmonyone/harmony-proto:51f7fa3c1588" "${progdir}/xargs_by_dir.sh" go generate -v -x
