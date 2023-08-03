#!/usr/bin/env bash

unset -v ok tmpdir gomod_diff_output goimports_output golint_output progdir
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

echo "Checking go.mod..."
gomod_diff_output="${tmpdir}/gomod.diff"
if git diff --exit-code -- go.mod > "${gomod_diff_output}"
then
	echo "go.mod stayed the same as in the repository."
else
	echo "go.mod has changed from the repository version!"
	"${progdir}/print_file.sh" "${gomod_diff_output}" "go.mod diff"
	"${progdir}/print_file.sh" go.mod "go.mod changed contents"
	#ok=false
	echo "Not treating this as an error, but consider updating go.mod!"
fi

echo "Running golint..."
golint_output="${tmpdir}/golint_output.txt"
if "${progdir}/golint.sh" > "${golint_output}" 2>&1
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

echo "Running go generate..."
gogenerate_status_before="${tmpdir}/gogenerate_status_before.txt"
gogenerate_status_after="${tmpdir}/gogenerate_status_after.txt"
gogenerate_status_diff="${tmpdir}/gogenerate_status.diff"
gogenerate_output="${tmpdir}/gogenerate_output.txt"
git status --porcelain=v2 > "${gogenerate_status_before}"
if "${progdir}/gogenerate.sh" > "${gogenerate_output}" 2>&1
then
	echo "go generate succeeded."
	echo "Checking if go generate changed any files..."
	git status --porcelain=v2 > "${gogenerate_status_after}"
	if diff -u "${gogenerate_status_before}" "${gogenerate_status_after}" \
		> "${gogenerate_status_diff}"
	then
		echo "All generated files seem up to date."
	else
		echo "go generate FAILED!"
		echo "go generate changed working tree contents!"
		"${progdir}/print_file.sh" "${gogenerate_status_diff}" "git status diff"
		"${progdir}/print_file.sh" "${gogenerate_status_before}" "gogenerate before"
		"${progdir}/print_file.sh" "${gogenerate_status_after}" "gogenerate after"
		ok=false
	fi
else
	echo "go generate FAILED!"
	"${progdir}/print_file.sh" "${gogenerate_output}" "go generate"
	ok=false
fi

echo "Running go test..."
# Fix https://github.com/golang/go/issues/44129#issuecomment-788351567
go get -t ./...
if go test -count=1 -vet=all -race ./...
then
	echo "go test succeeded."
else
	echo "go test FAILED!"
	ok=false
fi

if ! ${ok}
then
	echo "Some checks failed; see output above."
	exit 1
fi

echo "All Checks Passed!!! :-)"
exit 0
