#!/bin/sh
# run go generate on .go files under source control; group by dir (package).
unset -v progdir
case "${0}" in
*/*) progdir="${0%/*}";;
*) progdir=.;;
esac
git grep -l '^//go:generate ' -- '*.go' | \
	PROTOC_IMAGE="frozen621/harmony-proto:latest" "${progdir}/xargs_by_dir.sh" go generate -v -x
