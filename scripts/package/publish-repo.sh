#!/usr/bin/env bash

ME=$(basename "$0")
DEPS=(aws createrepo aptly gpg)
PKG=rpm
PROFILE=dev
SRC=

# destination of the bucket to host the repo
declare -A TARGET
TARGET[rpm.dev]="haochen-harmony-pub/pub/yum"
TARGET[deb.dev]="haochen-harmony-pub/pub/repo"
TARGET[rpm.prod]="pub.harmony.one/release/package/yum"
TARGET[deb.prod]="pub.harmony.one/release/package/apt"

function usage() {
   cat<<-EOT
Usage: $ME [options]

Option:
   -h             print this help message
   -p dev/prod    profile of the repo (dev/prod, default: $PROFILE)
   -n rpm/deb     type of package for publish (rpm/deb, default: $PKG)
   -s directory   source of the package repo

Examples:
   $ME -p dev -n rpm -s ~/rpmbuild

   $ME -p prod -n deb -s ~/debbuild

EOT
   exit 0
}

function validation() {
   for dep in "${DEPS[@]}"; do
      if ! command -v "$dep" > /dev/null; then
         echo "missing dependency: $dep"
         exit 1
      fi
   done

   case $PROFILE in
      dev|prod) ;;
      *) usage ;;
   esac

   if [[ -z "$SRC" || ! -d "$SRC" ]]; then
      echo "missing source path or wrong path: $SRC"
      exit 1
   fi
}

function publish_rpm() {
   local target
   local tempdir
   target=${TARGET[$PKG.$PROFILE]}
   tempdir="/tmp/$(basename $target)"
   mkdir -p "$tempdir/x86_64"
   aws s3 sync "s3://$target" "$tempdir"
   cp -rv $SRC/RPMS/x86_64/* "$tempdir/x86_64"
   UPDATE=""
   if [ -e "$tempdir/x86_64/repodata/repomd.xml" ]; then
      UPDATE="--update"
   fi
   createrepo -v $UPDATE --deltas "$tempdir/x86_64/"

   aws s3 sync "$tempdir" "s3://$target" --acl public-read
}

function publish_deb() {
   if aptly repo show harmony-$PROFILE > /dev/null; then
      aptly repo add harmony-$PROFILE $SRC
      aptly publish update bionic s3:harmony-$PROFILE:
   else
      aptly repo create -distribution=bionic -component=main harmony-$PROFILE
      aptly repo add harmony-$PROFILE $SRC
      aptly publish repo harmony-$PROFILE s3:harmony-$PROFILE:
   fi
}

################## MAIN ##################
if [ $# = 0 ]; then
   usage
fi

while getopts ":hp:n:s:" opt; do
   case $opt in
      p) PROFILE=${OPTARG} ;;
      n) PKG=${OPTARG} ;;
      s) SRC=${OPTARG} ;;
      *) usage ;;
   esac
done

shift $((OPTIND - 1))

validation

case $PKG in
   rpm) publish_rpm ;;
   deb) publish_deb ;;
   *) usage ;;
esac
