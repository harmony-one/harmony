#!/bin/bash

unset -v ok tmpdir goimports_output golint_output progdir
ok=true

case "${0}" in
*/*) progdir="${0%/*}";;
*) progdir=.;;
esac
PATH="${PATH+"${PATH}:"}${progdir}"
export PATH

tmpdir=
trap 'case "${tmpdir}" in ?*) rm -rf "${tmpdir}";; esac' EXIT
tmpdir=$(mktemp -d)

. "${progdir}/setup_bls_build_flags.sh"

echo "Running go test..."
if go test -v -count=1 ./...
then
	echo "go test succeeded."
else
	echo "go test FAILED!"
	ok=false
fi

echo "Running golint..."
golint_output="${tmpdir}/golint_output.txt"
if "${progdir}/golint.sh" -set_exit_status > "${golint_output}" 2>&1
then
	echo "golint passed."
else
	echo "golint FAILED!"
	"${progdir}/print_file.sh" "${golint_output}" "golint"
	ok=false
fi

echo "Running goimports..."
goimports_output="${tmpdir}/goimports_output.txt"
"${progdir}/goimports.sh" -d -e > "${goimports_output}" 2>&1
if [ -s "${goimports_output}" ]
then
	echo "goimports FAILED!"
	"${progdir}/print_file.sh" "${goimports_output}" "goimports"
	ok=false
else
	echo "goimports passed."
fi

if ! ${ok}
then
	echo "Some checks failed; see output above."
	exit 1
fi

echo "All checks passed. :-)"
exit 0
