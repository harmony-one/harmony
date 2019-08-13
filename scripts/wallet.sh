#!/usr/bin/env bash

BUCKET=pub.harmony.one
OS=$(uname -s)
REL=r3

case "$OS" in
    Darwin)
        FOLDER=release/darwin-x86_64/${REL}/
        BIN=( wallet libbls384_256.dylib libcrypto.1.0.0.dylib libgmp.10.dylib libgmpxx.4.dylib libmcl.dylib )
        ;;
    Linux)
        FOLDER=release/linux-x86_64/${REL}/
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
   -p profile  use the profile for the given network (default [main], beta, pangaea)
   -t          equivalent to -p pangaea (deprecated)
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
    8. export        - Export account key to a new file
        --account        - Specify the account to export. Empty will export every key.
    9. exportPriKey  - Export account private key
        --account        - Specify the account to export private key.
    10. blsgen        - Generate a bls key and store private key locally.
        --nopass         - The private key has no passphrase (for test only)
    11. format        - Shows different encoding formats of specific address
        --address        - The address to display the different encoding formats for
    12. blsRecovery    - Recover non-human readable file.
        --pass           - The file containg the passphrase to decrypt the bls key.
        --file           - Non-human readable bls file.
    13. importBls      - Convert raw private key into encrypted bls key.
        --key            - Raw private key.
    14. getBlsPublic   - Show Bls public key given raw private bls key.
        --key            - Raw private key.
		--file           - encrypted bls file.
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

unset network
network=default

while getopts "dp:th" opt; do
    case ${opt} in
        d)
            do_download
            exit 0
            ;;
        p)
            network="${OPTARG}"
            ;;
        t)
            network=pangaea
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
    LD_LIBRARY_PATH=$(pwd) ./wallet -p "$network" "$@"
else
    DYLD_FALLBACK_LIBRARY_PATH=$(pwd) ./wallet -p "$network" "$@"
fi
