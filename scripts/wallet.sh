#!/usr/bin/env bash

BUCKET=pub.harmony.one
OS=$(uname -s)
REL=drum

case "$OS" in
    Darwin)
        FOLDER=release/${REL}/darwin-x86_64/
        BIN=( wallet libbls384_256.dylib libcrypto.1.0.0.dylib libgmp.10.dylib libgmpxx.4.dylib libmcl.dylib )
        ;;
    Linux)
        FOLDER=release/${REL}/linux-x86_64/
        BIN=( wallet libbls384_256.so libcrypto.so.10 libgmp.so.10 libgmpxx.so.4 libmcl.so )
        ;;
    *)
        echo "${OS} not supported."
        exit 2
        ;;
esac

usage () {
   cat << EOT
Usage: $0 [option] command

Options:
   -d          download all the binaries/config files (do it when updated)
   -h          print this help

Commands:
    1. new           - Generates a new account and store the private key locally
    2. list          - Lists all accounts in local keystore
    3. removeAll     - Removes all accounts in local keystore
    4. import        - Imports a new account by private key
        --privateKey     - the private key to import
    5. balances      - Shows the balances of all addresses or specific address
        --address        - The address to check balance for
    6. getFreeToken  - Gets free token on each shard
        --address        - The free token receiver account's address
    7. transfer      - Transfer token from one account to another
        --from           - The sender account's address or index in the local keystore
        --to             - The receiver account's address
        --amount         - The amount of token to transfer
        --shardID        - The shard Id for the transfer
        --inputData      - Base64-encoded input data to embed in the transaction
    8. blsgen        - Generates bls keys with passphrase and store the private key locally
EOT
}

do_download () {
# clean up old files
    for bin in "${BIN[@]}"; do
        rm -f ${bin}
    done

# download all the binaries
    for bin in "${BIN[@]}"; do
        curl http://${BUCKET}.s3.amazonaws.com/${FOLDER}${bin} -o ${bin}
    done

    mkdir -p .hmy/keystore
    chmod +x wallet
}

while getopts "dh" opt; do
    case ${opt} in
        d)
            do_download
            exit 0
            ;;
        h|*)
            usage
            exit 1
            ;;
    esac
done

shift $((OPTIND-1))

# Run Harmony Wallet
if [ "$OS" = "Linux" ]; then
    LD_LIBRARY_PATH=$(pwd) ./wallet "$@"
else
    DYLD_FALLBACK_LIBRARY_PATH=$(pwd) ./wallet "$@"
fi