#!/usr/bin/env bash

export GO111MODULE=on

declare -A SRC
SRC[harmony]=cmd/harmony/main.go
SRC[bootnode]=cmd/bootnode/main.go

BINDIR=bin
BUCKET=unique-bucket-bin
PUBBUCKET=pub.harmony.one
REL=
GOOS=linux
GOARCH=amd64
FOLDER=${WHOAMI:-$USER}
RACE=
VERBOSE=
GO_GCFLAGS="all=-c 2"
DEBUG=false
NETWORK=main
STATIC=false

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
   LIB[libbls384_256.dylib]=${BLS_DIR}/lib/libbls384_256.dylib
   LIB[libmcl.dylib]=${MCL_DIR}/lib/libmcl.dylib
   LIB[libgmp.10.dylib]=/usr/local/opt/gmp/lib/libgmp.10.dylib
   LIB[libgmpxx.4.dylib]=/usr/local/opt/gmp/lib/libgmpxx.4.dylib
   LIB[libcrypto.1.0.0.dylib]=/usr/local/opt/openssl/lib/libcrypto.1.0.0.dylib
else
   MD5=md5sum
   LIB[libbls384_256.so]=${BLS_DIR}/lib/libbls384_256.so
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
   -s             build static linux executable (default: $STATIC)


ACTION:
   build       build binaries only (default action)
   upload      upload binaries to s3
   release     upload binaries to release bucket

   harmony|bootnode|
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
   if [[ "$STATIC" == "true" && "$GOOS" == "darwin" ]]; then
      echo "static build only supported on Linux platform"
      exit 2
   fi

   VERSION=$(git rev-list --count HEAD)
   COMMIT=$(git describe --always --long --dirty)
   BUILTAT=$(date +%FT%T%z)
   BUILTBY=${USER}@
   local build=$1

   set_gcflags
   set -e

   for bin in "${!SRC[@]}"; do
      if [[ -z "$build" || "$bin" == "$build" ]]; then
         rm -f $BINDIR/$bin
         echo "building ${SRC[$bin]}"
         if [ "$DEBUG" == "true" ]; then
            env GOOS=$GOOS GOARCH=$GOARCH go build $VERBOSE -gcflags="${GO_GCFLAGS}" -ldflags="-X main.version=v${VERSION} -X main.commit=${COMMIT} -X main.builtAt=${BUILTAT} -X main.builtBy=${BUILTBY}" -o $BINDIR/$bin $RACE ${SRC[$bin]}
         else
            if [ "$STATIC" == "true" ]; then
               env GOOS=$GOOS GOARCH=$GOARCH go build $VERBOSE -gcflags="${GO_GCFLAGS}" -ldflags="-X main.version=v${VERSION} -X main.commit=${COMMIT} -X main.builtAt=${BUILTAT} -X main.builtBy=${BUILTBY}  -w -extldflags \"-static -lm\"" -o $BINDIR/$bin $RACE ${SRC[$bin]}
            else
               env GOOS=$GOOS GOARCH=$GOARCH go build $VERBOSE -gcflags="${GO_GCFLAGS}" -ldflags="-X main.version=v${VERSION} -X main.commit=${COMMIT} -X main.builtAt=${BUILTAT} -X main.builtBy=${BUILTBY}" -o $BINDIR/$bin $RACE ${SRC[$bin]}
            fi
         fi
         if [ "$(uname -s)" == "Linux" ]; then
            $BINDIR/$bin -version || $BINDIR/$bin version
         fi
         if [ "$(uname -s)" == "Darwin" -a "$GOOS" == "darwin" -a -e $BINDIR/$bin ]; then
            $BINDIR/$bin -version || $BINDIR/$bin version
         fi
      fi
   done

   pushd $BINDIR
   if [ "$STATIC" == "true" ]; then
      $MD5 "${!SRC[@]}" > md5sum.txt
   else
      for lib in "${!LIB[@]}"; do
         if [ -e ${LIB[$lib]} ]; then
            cp -pf ${LIB[$lib]} .
         fi
      done

      $MD5 "${!SRC[@]}" "${!LIB[@]}" > md5sum.txt
      # hardcode the prebuilt libcrypto to md5sum.txt
      if [ "$(uname -s)" == "Linux" ]; then
         echo '771150db04267126823190c873a96e48  libcrypto.so.10' >> md5sum.txt
      fi
   fi
   popd
}

function set_gcflags
{
   if [[ ! -z "$RACE" ]]; then
      if [ "$DEBUG" == "true" ]; then
         GO_GCFLAGS="all=-N -l"
      else
         GO_GCFLAGS=""
      fi
   else
      if [ "$DEBUG" == "true" ]; then
         GO_GCFLAGS="all=-N -l -c 2"
      fi
   fi
}

function upload
{
   AWSCLI=aws

   if [ -n "$PROFILE" ]; then
      AWSCLI+=" --profile $PROFILE"
   fi

   if [ "$STATIC" != "true" ]; then
      for lib in "${!LIB[@]}"; do
         if [ -e ${LIB[$lib]} ]; then
            $AWSCLI s3 cp ${LIB[$lib]} s3://${BUCKET}/$FOLDER/$lib --acl public-read
         else
            echo "!! MISSING ${LIB[$lib]} !!"
         fi
      done
   else
      FOLDER+='/static'
   fi

   for bin in "${!SRC[@]}"; do
      [ -e $BINDIR/$bin ] && $AWSCLI s3 cp $BINDIR/$bin s3://${BUCKET}/$FOLDER/$bin --acl public-read
   done


   [ -e $BINDIR/md5sum.txt ] && $AWSCLI s3 cp $BINDIR/md5sum.txt s3://${BUCKET}/$FOLDER/md5sum.txt --acl public-read
}

function release
{
   AWSCLI=aws

   if [ -n "$PROFILE" ]; then
      AWSCLI+=" --profile $PROFILE"
   fi

   OS=$(uname -s)

   case "$OS" in
      "Linux")
         FOLDER=release/linux-x86_64/$REL ;;
      "Darwin")
         FOLDER=release/darwin-x86_64/$REL ;;
      *)
         echo "Unsupported OS: $OS"
         return ;;
   esac

   if [ "$STATIC" != "true" ]; then
      for lib in "${!LIB[@]}"; do
         if [ -e ${LIB[$lib]} ]; then
            $AWSCLI s3 cp ${LIB[$lib]} s3://${PUBBUCKET}/$FOLDER/$lib --acl public-read
         else
            echo "!! MISSING ${LIB[$lib]} !!"
         fi
      done
   else
      FOLDER+='/static'
   fi

   for bin in "${!SRC[@]}"; do
      if [ -e $BINDIR/$bin ]; then
         $AWSCLI s3 cp $BINDIR/$bin s3://${PUBBUCKET}/$FOLDER/$bin --acl public-read
      else
         echo "!! MISSGING $bin !!"
      fi
   done

   [ -e $BINDIR/md5sum.txt ] && $AWSCLI s3 cp $BINDIR/md5sum.txt s3://${PUBBUCKET}/$FOLDER/md5sum.txt --acl public-read
}


################################ MAIN FUNCTION ##############################
while getopts "hp:a:o:b:f:rvsdN:" option; do
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
      s) STATIC=true ;;
      N) NETWORK=$OPTARG ;;
   esac
done

mkdir -p $BINDIR

shift $(($OPTIND-1))

ACTION=${1:-build}

case "${NETWORK}" in
main)
  REL=mainnet
  ;;
beta)
  REL=testnet
  ;;
pangaea)
  REL=pangaea
  ;;
*)
  echo "${NETWORK}: invalid network"
  exit
  ;;
esac

case "$ACTION" in
   "build") build_only ;;
   "upload") upload ;;
   "release") release ;;
   "harmony"|"bootnode") build_only $ACTION ;;
   *) usage ;;
esac
