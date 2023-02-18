#!/usr/bin/env bash

set -e

declare -a DBHashList
DBHashList=(
    [22816574]=f5894991e83cff54b215cda93f18bc813bb9bf9a45643c8751f09891aad1f091
)

DB_FILES="CURRENT MANIFEST* *.log *.ldb"

function dbFilesHash() {
    sha256sum $DB_FILES
}

function dbRootHash() {
    echo -ne "$1" | tr -s [:space:] : | sha256sum | cut -d " " -f 1
}

function checkRootHash() {
    rootHash=$1
    for height in "${!DBHashList[@]}"
    do
        dbhash=${DBHashList[$height]}
        if [ $dbhash == $rootHash ];then
            echo "Success! The hash matches the height $height."
            exit 0
        fi
    done
    echo "check failed!" >&2
    exit 1
}

print_usage() {
    progname="${0##*/}"
    cat <<- ENDEND

usage: ${progname} COMMAND DBDIR

The COMMAND are:
    hash    calculate root hash for snapdb
    check   verify root hash of snpadb
DBDIR is path of db

Ex:
   ${progname} hash ./harmony_db_0
   ${progname} check ./harmony_db_0
ENDEND
}

CMD=$1
DBDIR=$2
if [ -z $DBDIR ];then
    print_usage
    exit 1
fi
cd $DBDIR

case "$CMD" in
   check)
    rootHash=`dbRootHash "$(dbFilesHash)"`
    checkRootHash $rootHash
    ;;
   hash)
    dbRootHash "$(dbFilesHash)"
    ;;
   *)
    print_usage
    ;;
esac
