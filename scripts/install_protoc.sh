#!/usr/bin/env bash
set -eu
unset -v opt OPTIND OPTARG version platform destdir
version=
platform=
destdir=/usr/local
usage() {
	case $# in
	[1-9]*) echo "$@" >&2;;
	esac
	cat <<- ENDEND
		usage: install_protoc.sh [-P platform] [-d destdir] -V version
		options:
		-V version	protobuf version
		-P platform	fetch and use given platform (default: autodetect)
		-d destdir	install into the given dir (default: /usr/local)
		-h		print this help
	ENDEND
	exit 64
}

while getopts :V:P:d:h opt
do
	case "${opt}" in
	V) version="${OPTARG}";;
	P) platform="${OPTARG}";;
	d) destdir="${OPTARG}";;
	h) usage;;
	"?") usage "unrecognized option -${OPTARG}";;
	":") usage "missing argument for -${OPTARG}";;
	*) echo "unhandled option -${OPTARG}" >&2; exit 70;;
	esac
done
shift $((${OPTIND} - 1))
case "${version}" in
"") usage "protobuf version (-V) not specified";;
esac
case "${platform}" in
"")
	platform=$(uname -s)
	case "${platform}" in
	Darwin) platform=osx;;
	Linux) platform=linux;;
	*) echo "unsupported OS name (${platform})" >&2; exit 69;;
	esac
	platform="${platform}-$(uname -m)"
	;;
esac
unset -v tmpdir
trap 'case "${tmpdir-}" in ?*) rm -rf "${tmpdir}";; esac' EXIT
tmpdir=$(mktemp -d)
cd "${tmpdir}"
unset -v filename url
filename="protoc-${version}-${platform}.zip"
url="https://github.com/protocolbuffers/protobuf/releases/download/v${version}/${filename}"
echo "Downloading protoc v${version} for ${platform}..."
curl -s -S -L -o "${filename}" "${url}"
echo "Downloaded as ${filename}; unzipping into ${destdir}..."
sudo unzip -o -d "${destdir}" "${filename}"
echo "protoc v${version} has been installed in ${destdir}."
sudo chmod +x "${destdir}/bin/protoc"
exit 0
