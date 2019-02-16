#!/usr/bin/env bash
#
# usage: xargs_by_dir.sh cmd [args...] < pathnames
#
# Run a command with pathnames given over stdin, grouping pathnames by its
# dirname and supplying each invocation of the command with pathnames in the
# same directory.  This is useful for commands that require all pathname
# arguments to be in the same directory, such as go generate.

set -eu

dirname() {
	case "${1}" in
	*/*) echo "${1%/*}";;
	*) echo ".";;
	esac
}

unset -v cmd ok
declare -a cmd=("${@}")
ok=true
run_cmd() {
	case $# in 0) return;; esac
	"${cmd[@]}" "${@}" || ok=false
}

# sort the input list by directory; replace stdin with the sorted list
unset -v sorted_list
trap 'case "${sorted_list-}" in ?*) rm -f "${sorted_list}";; esac' EXIT
sorted_list=$(mktemp)
sort -t/ > "${sorted_list}"
exec < "${sorted_list}"

unset -v last_dir dir path
set --
while read -r path
do
	# dirname of the next item to consider
	dir=$(dirname "${path}")
	# the same dir as the last one?
	case "${last_dir-}" in
	"${dir}")
		# same dir; do not flush the list
		;;
	*)
		# dir change; run with items seen so far and flush the list
		run_cmd "${@}"
		set --
		;;
	esac
	# add the item, update the last-seen dir for the next iteration
	set -- "${@}" "${path}"
	last_dir="${dir}"
done
# seen the last file; run with items seen so far
run_cmd "${@}"
${ok} && exit 0 || exit 1
