#!/bin/bash

BUCKET=pub.harmony.one
OS=$(uname -s)
REL=cello-test

if [ "$OS" == "Darwin" ]; then
   FOLDER=release/$REL/darwin-x86_64/
   BIN=( txgen wallet.ini libbls384.dylib libcrypto.1.0.0.dylib libgmp.10.dylib libgmpxx.4.dylib libmcl.dylib )
fi
if [ "$OS" == "Linux" ]; then
   FOLDER=release/$REL/linux-x86_64/
   BIN=( txgen wallet.ini libbls384.so libcrypto.so.10 libgmp.so.10 libgmpxx.so.4 libmcl.so )
fi

function usage
{
   cat<<EOT
Usage: $0 [option] command

Option:
   -d          download all the binaries/config files (do it when updated)
   -h          print this help

TxGen Help:

txgen script downloads and runs the binary taking configuraiton details from wallet.ini
-duration  Sets the duration for which you want to run the txgen
-ip sets the ip of the machine running the txgen
-port sets the port of the node that will be listening
-log_folder the folder collecting the logs of this execution
-bootnodes supply bootnode addresses, refer to wallet.ini
-key the private key file of the txgen, if not specified, it will be created.

EOT
	exit 0
}

function do_download
{
# clean up old files
	for bin in "${BIN[@]}"; do
		rm -f ${bin}
	done

# download all the binaries
	for bin in "${BIN[@]}"; do
		curl http://${BUCKET}.s3.amazonaws.com/${FOLDER}${bin} -o ${bin}
	done

# set wallet.ini
	mkdir -p .hmy
	cp -f wallet.ini .hmy
	chmod +x txgen

	exit 0
}

download_only=false

while getopts "dh" opt; do
	case $opt in
		h) usage ;;
		d) download_only=true ;;
		*) usage ;;
	esac
done

shift $((OPTIND-1))

if [ $download_only == "true" ]; then
	do_download
fi

# Run Harmony TXGEN
if [ "$OS" == "Linux" ]; then
   LD_LIBRARY_PATH=$(pwd) ./txgen $*
else
   DYLD_FALLBACK_LIBRARY_PATH=$(pwd) ./txgen $*
fi
