#!/usr/bin/env bash

export GO111MODULE=on

declare -A SRC
SRC[harmony]=cmd/harmony/main.go
SRC[txgen]=cmd/client/txgen/main.go
SRC[bootnode]=cmd/bootnode/main.go
SRC[wallet]=cmd/client/wallet/main.go

BINDIR=bin
BUCKET=unique-bucket-bin
PUBBUCKET=pub.harmony.one
GOOS=linux
GOARCH=amd64
FOLDER=/${WHOAMI:-$USER}
RACE=
VERBOSE=
DEBUG=false

unset -v progdir
case "${0}" in
*/*) progdir="${0%/*}";;
*) progdir=.;;
esac

. "${progdir}/setup_bls_build_flags.sh"

declare -A LIB

if [ "$(uname -s)" == "Darwin" ]; then
   MD5='md5 -r'
   GOOS=darwin
   LIB[libbls384.dylib]=${BLS_DIR}/lib/libbls384.dylib
   LIB[libmcl.dylib]=${MCL_DIR}/lib/libmcl.dylib
else
   MD5=md5sum
   LIB[libbls384.so]=${BLS_DIR}/lib/libbls384.so
   LIB[libmcl.so]=${MCL_DIR}/lib/libmcl.so
fi

function usage
{
   ME=$(basename $0)
   cat<<EOF

Usage: $ME [OPTIONS] ACTION

OPTIONS:
   -h             print this help message
   -p profile     aws profile name
   -a arch        set build arch (default: $GOARCH)
   -o os          set build OS (default: $GOOS, windows is supported)
   -b bucket      set the upload bucket name (default: $BUCKET)
   -f folder      set the upload folder name in the bucket (default: $FOLDER)
   -r             enable -race build option (default: $RACE)
   -v             verbose build process (default: $VERBOSE)

ACTION:
   build       build binaries only (default action)
   upload      upload binaries to s3
   pubwallet   upload wallet to public bucket (bucket: $PUBBUCKET)

   harmony|txgen|bootnode|wallet
               only build the specified binary

EXAMPLES:

# build linux binaries only by default
   $ME

# build windows binaries
   $ME -o windows

# upload binaries to my s3 bucket, 0908 folder
   $ME -b mybucket -f 0908 upload

EOF
   exit 1
}

function build_only
{
   VERSION=$(git rev-list --count HEAD)
   COMMIT=$(git describe --always --long --dirty)
   BUILTAT=$(date +%FT%T%z)
   BUILTBY=${USER}@
   local build=$1

   for bin in "${!SRC[@]}"; do
      if [[ -z "$build" || "$bin" == "$build" ]]; then
         rm -f $BINDIR/$bin
         echo "building ${SRC[$bin]}"
         if [ "$DEBUG" == "true" ]; then
            env GOOS=$GOOS GOARCH=$GOARCH go build $VERBOSE -gcflags="all=-N -l -c 2" -ldflags="-X main.version=v${VERSION} -X main.commit=${COMMIT} -X main.builtAt=${BUILTAT} -X main.builtBy=${BUILTBY}" -o $BINDIR/$bin $RACE ${SRC[$bin]}
         else
            env GOOS=$GOOS GOARCH=$GOARCH go build $VERBOSE -gcflags="all=-c 2" -ldflags="-X main.version=v${VERSION} -X main.commit=${COMMIT} -X main.builtAt=${BUILTAT} -X main.builtBy=${BUILTBY}" -o $BINDIR/$bin $RACE ${SRC[$bin]}
         fi
         if [ "$(uname -s)" == "Linux" ]; then
            $BINDIR/$bin -version
         fi
         if [ "$(uname -s)" == "Darwin" -a "$GOOS" == "darwin" -a -e $BINDIR/$bin ]; then
            $BINDIR/$bin -version
         fi
      fi
   done

   $MD5 $BINDIR/* > $BINDIR/md5sum.txt 2> /dev/null
}

function upload
{
   AWSCLI=aws

   if [ -n "$PROFILE" ]; then
      AWSCLI+=" --profile $PROFILE"
   fi

   for bin in "${!SRC[@]}"; do
      [ -e $BINDIR/$bin ] && $AWSCLI s3 cp $BINDIR/$bin s3://${BUCKET}$FOLDER/$bin --acl public-read
   done

   for lib in "${!LIB[@]}"; do
      if [ -e ${LIB[$lib]} ]; then
         $AWSCLI s3 cp ${LIB[$lib]} s3://${BUCKET}$FOLDER/$lib --acl public-read
      else
         echo "!! MISSING ${LIB[$lib]} !!"
      fi
   done

   [ -e $BINDIR/md5sum.txt ] && $AWSCLI s3 cp $BINDIR/md5sum.txt s3://${BUCKET}$FOLDER/md5sum.txt --acl public-read
}

function upload_wallet
{
   AWSCLI=aws

   if [ -n "$PROFILE" ]; then
      AWSCLI+=" --profile $PROFILE"
   fi


   OS=$(uname -s)

   case "$OS" in
      "Linux")
         DEST=wallet/wallet
         DESTDIR=wallet ;;
      "Darwin")
         DEST=wallet.osx/wallet
         DESTDIR=wallet.osx ;;
      *)
         echo "Unsupported OS: $OS"
         return ;;
   esac

   $AWSCLI s3 cp $BINDIR/wallet s3://$PUBBUCKET/$DEST
   $AWSCLI s3api put-object-acl --bucket $PUBBUCKET --key $DEST --acl public-read

   for lib in "${!LIB[@]}"; do
      if [ -e ${LIB[$lib]} ]; then
         $AWSCLI s3 cp ${LIB[$lib]} s3://${PUBBUCKET}/$DESTDIR/$lib --acl public-read
      else
         echo "!! MISSING ${LIB[$lib]} !!"
      fi
   done


}

################################ MAIN FUNCTION ##############################
while getopts "hp:a:o:b:f:rv" option; do
   case $option in
      h) usage ;;
      p) PROFILE=$OPTARG ;;
      a) GOARCH=$OPTARG ;;
      o) GOOS=$OPTARG ;;
      b) BUCKET=$OPTARG/ ;;
      f) FOLDER=$OPTARG ;;
      r) RACE=-race ;;
      v) VERBOSE='-v -x' ;;
      d) DEBUG=true ;;
   esac
done

mkdir -p $BINDIR

shift $(($OPTIND-1))

ACTION=${1:-build}

case "$ACTION" in
   "build") build_only ;;
   "upload") upload ;;
   "pubwallet") upload_wallet ;;
   "harmony"|"wallet"|"txgen"|"bootnode") build_only $ACTION ;;
   *) usage ;;
esac
