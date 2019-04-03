#!/bin/sh

set -eu

unset -v progname progdir
progname="${0##*/}"
case "${0}" in
*/*) progdir="${0%/*}";;
*) progdir=.;;
esac

unset -v tmpdir
sigs="0 1 2 15"  # EXIT HUP INT TERM
trapfunc() {
	case "${tmpdir-}" in ?*) rm -rf "${tmpdir}";; esac
	trap -- ${sigs}
	case "${1}" in 0|EXIT) ;; *) kill "-${1}" "$$";; esac
}
unset -v sig; for sig in ${sigs}; do trap "trapfunc ${sig}" "${sig}"; done
tmpdir=$(mktemp -d)

unset -v jq
jq=$(which jq) || { echo "${progname}: jq not found" >&2; exit 69; }

cd "${progdir}"
while [ ! -f go.mod ]
do
	case "$(pwd)" in
	/) echo "${progname}: go.mod not found" >&2; exit 69;;
	esac
	cd ..
done

go mod edit -json | \
	"${jq}" -r '.Require[] | .Path + "@" + .Version' \
	> "${tmpdir}/gomod.txt"

ok=true
unset -v pkg
for pkg
do
	unset -v best_path best_version path version
	while IFS=@ read -r path version
	do
		# Is requested package at or under this path?  Skip if not.
		case "${pkg}" in
		"${path}"|"${path}"/*) ;;
		*) continue;;
		esac
		: ${best_path="${path}"}
		: ${best_version="${version}"}
		# Is this path more specific than the current best?
		case "${path}" in
		"${best_path}"/*)
			best_path="${path}"
			best_version="${version}"
		esac
	done < "${tmpdir}/gomod.txt"
	case "${best_path-}" in
	"")
		echo "${progname}: no module provides package ${pkg}" >&2
		ok=false
		continue
		;;
	esac
	echo "${progname}: ${pkg} is provided by ${best_path}@${best_version}"
	go get "${pkg}@${best_version}"
done
"${ok}" && exit 0 || exit 1
