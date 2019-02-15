#!/bin/sh
unset -v progdir
case "${0}" in
*/*) progdir="${0%/*}";;
*) progdir=.;;
esac
"${progdir}/list_harmony_go_files.sh" | "${progdir}/dirnames.sh" | \
	sort -u -t/ | xargs golint "$@"
