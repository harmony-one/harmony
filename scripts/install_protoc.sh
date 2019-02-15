#!/usr/bin/env bash
set -eu
unset -v opt OPTIND OPTARG version platform destdir
version=
platform=
destdir=/usr/local
while getopts :V:P:d: opt
do
	case "${opt}" in
	V) version="${OPTARG}";;
	P) platform="${OPTARG}";;
	d) destdir="${OPTARG}";;
	"?") echo "unrecognized option -${OPTARG}" >&2; exit 64;;
	":") echo "missing argument for -${OPTARG}" >&2; exit 64;;
	*) echo "unhandled option -${OPTARG}" >&2; exit 70;;
	esac
done
shift $((${OPTIND} - 1))
case "${version}" in
"") echo "protobuf version (-V) not specified" >&2; exit 64;;
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
exit 0
