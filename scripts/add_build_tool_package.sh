#!/bin/sh

set -eu

unset -v progname progdir
progname="${0##*/}"
case "${0}" in
*/*) progdir="${0%/*}";;
*) progdir=".";;
esac

msg() { case $# in [1-9]*) echo "${progname}: $*" >&2;; esac; }

print_usage() {
	cat <<- ENDEND
		usage: ${progname} [-h] pkg ...

		Register the given tool package(s) in tools/tools.go and go.mod.

		pkg is a tool to install, ex: github.com/golangci/golangci-lint/cmd/golangci-lint@v1.15.0
		omitting the @version suffix installs the latest.

		options:
		-h	print this help
		ENDEND
}

usage() {
	msg "$@"
	print_usage >&2
	exit 64
}

unset -v opt
OPTIND=1
while getopts :h opt
do
	case "${opt}" in
	"?") usage "unrecognized option -${OPTARG}";;
	":") usage "missing argument for -${OPTARG}";;
	h) print_usage; exit 0;;
	esac
done
shift $((${OPTIND} - 1))

cd "${progdir}/.."

if git status --short go.mod tools/tools.go | grep -q .
then
	msg "go.mod or tools/tools.go contains local change;" \
		"commit or stash them first." >&2
	exit 69
fi

go mod tidy
if git status --short go.mod | grep -q .
then
	git commit -m 'go mod tidy' go.mod
fi

doit() {
	local pkg pkg_name
	pkg="${1-}"
	shift 1 || return $?
	pkg_name="${pkg%%@*}"
	msg "adding ${pkg} to tools/tools.go"
	sed -i.orig '/^import ($/a\
	_ "'"${pkg_name}"'"
' tools/tools.go
	rm -f tools/tools.go.orig
	goimports -w tools/tools.go
	if git diff --exit-code tools/tools.go
	then
		msg "${pkg_name} already seems to be in tools/tools.go"
		return 0
	fi
	msg "adding ${pkg} to go.mod"
	go get "${pkg}"
	go mod tidy
	if git diff --exit-code go.mod
	then
		msg "${pkg} already seems to be in go.mod" \
			"(maybe it is already required as a build dependency)"
	fi
	git commit -m "Add ${pkg} as a build tool" tools/tools.go go.mod
}

unset -v pkg ok
ok=true
for pkg
do
	doit "${pkg}" || ok=false
done
exec "${ok}"
