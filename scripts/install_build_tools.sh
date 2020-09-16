#!/bin/sh

set -eu

unset -v progdir
case "${0}" in
/*) progdir="/";;
*/*) progdir="${0%/*}";;
*) progdir=".";
esac

sed -n 's/^	_ "\([^"]*\)"$/\1/p' "${progdir}/../tools/tools.go" | \
	xargs "${progdir}/goget.sh"

"${progdir}/install_protoc.sh" -V 3.12.3
