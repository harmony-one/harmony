#!/bin/sh
unset -v progdir
case "${0}" in
*/*) progdir="${0%/*}";;
*) progdir=.;;
esac
git grep -l '^//go:generate ' | sort -t/ | (
	gogenerate() {
		case $# in 0) return;; esac
		go generate "$@"
	}
	unset -v prevdir dir file
	prevdir=
	set --
	while read -r file
	do
		case "${file}" in
		*/*) dir="${file%/*}";;
		*) dir=".";;
		esac
		case "${prevdir}" in
		"${dir}")
			set -- "${@}" "${file}"
			;;
		*) # includes ""
			gogenerate "${@}" # no-op for ""
			set -- "${file}"
			;;
		esac
		prevdir="${dir}"
	done
	gogenerate "$@"
)
