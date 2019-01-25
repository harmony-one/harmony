#!/bin/bash

unset -v ok tmpdir go_files go_dirs goimports_output golint_output progdir
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

go_files="${tmpdir}/go_files.txt"
"${progdir}/list_harmony_go_files.sh" > "${go_files}"

# Print dirname of each relative pathname from stdin (one per line).
dirnames() {
	# pathname	dirname
	# ----------------------
	# a/b/c.go	a/b
	# c.go		.
	sed \
		-e 's:^:./:' \
		-e 's:/[^/]*$::' \
		-e 's:^\./::'
}

go_dirs="${tmpdir}/go_dirs.txt"
dirnames < "${go_files}" | sort -u -t/ > "${go_dirs}"

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
if xargs golint -set_exit_status < "${go_dirs}" > "${golint_output}" 2>&1
then
	echo "golint passed."
else
	echo "golint FAILED!"
	"${progdir}/print_file.sh" "${golint_output}" "golint"
	ok=false
fi

echo "Running goimports..."
goimports_output="${tmpdir}/goimports_output.txt"
xargs goimports -d -e < "${go_files}" > "${goimports_output}" 2>&1
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
