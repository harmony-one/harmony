#!/usr/bin/env bash
# This Script is for Testing the API functionality on both local and betanet.
# -l to run localnet, -b to run betanet(mutually exclusive)
# -v to see returns from each request
# Right now only tests whether a response is recieved
# You must have properly cloned into the dapp-examples repo and have installed nodejs
VERBOSE="FALSE"
TESTS_RAN=0
TESTS_PASSED=0

red=`tput setaf 1`
green=`tput setaf 2`
blue=`tput setaf 6`
white=`tput sgr0`
yellow=`tput setaf 11`
reset=`tput sgr0`

function response_test() {
	if [ "$1" != "" ]; then
		echo "${green}RESPONSE RECIEVED${reset}"
		return 1
	else
		echo "${red}NO RESPONSE${reset}"
		return 0
	fi
}

function isHashTest() {
	if [ "$TRANSACTION" != "null" ]; then
		if [[ "$TRANSACTION_HASH" =~ ^0x[0-9a-f]{64}$ ]]; then
			echo ${green}TRANSACTION HASH VALID${reset}
			echo
			return 1
		fi
	fi
	echo ${red}TRANSACTION HASH INVALID${reset}
	return 0
}

function isHexTest() {
	if [ "$1" != "null" ]; then
		if [[ "$1" =~ ^0x[0-9a-f]+$ ]]; then
			echo ${green}VALID HEX RECIEVED${reset}
			echo
			return 1
		fi
	fi
	echo ${red}INVALID HEX RECIEVED${reset}
	return 0
}

### SETUP COMMANDLINE FLAGS ###
while getopts "lbvp" OPTION; do
	case $OPTION in
	b)
		NETWORK="betanet"
		declare -A PORT=( [POST]="http://s0.b.hmny.io:9500/" [GET]="http://e0.b.hmny.io:5000/" )
		BLOCK_0_HASH=$(curl --location --request POST "http://l0.b.hmny.io:9500" \
			  --header "Content-Type: application/json" \
			  --data "{\"jsonrpc\":\"2.0\",\"method\":\"hmy_getBlockByNumber\",\"params\":[\"0x1\", true],\"id\":1}" | jq -r '.result.hash')
		echo "BLOCK0HASH:"
		echo "$BLOCK_0_HASH"

		SIGNED_RAW_TRANSACTION=$(node ../dapp-examples/nodejs/apiTestSign.js)
		echo "RAWTX"
		echo "$SIGNED_RAW_TRANSACTION"
		TRANSACTION_HASH=$(curl  --location --request POST "http://l0.b.hmny.io:9500" \
			  --header "Content-Type: application/json" \
			    --data "{\"jsonrpc\":\"2.0\",\"method\":\"hmy_sendRawTransaction\",\"params\":[\""$SIGNED_RAW_TRANSACTION"\"],\"id\":1}" | jq -r '.result')
		echo "TRANSACTION_HASH:"
		echo $TRANSACTION_HASH
		sleep 20s
		TRANSACTION=$(curl --location --request POST "http://l0.b.hmny.io:9500" \
			  --header "Content-Type: application/json" \
			  --data "{\"jsonrpc\":\"2.0\",\"method\":\"hmy_getTransactionByHash\",\"params\":[\"$TRANSACTION_HASH\"],\"id\":1}")

		echo "TRANSACTION:"
		echo "$TRANSACTION"

		TRANSACTION_BLOCK_HASH=$(echo $TRANSACTION | jq -r '.result.blockHash')
		TRANSACTION_BLOCK_NUMBER=$(echo $TRANSACTION | jq -r '.result.blockNumber')
		TRANSACTION_INDEX=$(echo $TRANSACTION | jq -r '.result.transactionIndex')  #Needs to be get transaction Index


		TRANSACTION_BLOCK_ID=$(( $TRANSACTION_BLOCK_NUMBER ))
		echo TRANSACTION_BLOCK_ID
		echo $TRANSACTION_BLOCK_ID
		echo "TRANSACTION_BLOCK_HASH:"
		echo $TRANSACTION_BLOCK_HASH

		echo "TRANSACTION_BLOCK_NUMBER:"
		echo "$TRANSACTION_BLOCK_NUMBER"

		echo "TRANSACTION_INDEX:"
		echo $TRANSACTION_INDEX

		;;
	l)
		NETWORK="localnet"
		declare -A PORT=( [POST]="localhost:9500/" [GET]="localhost:5099/" )
		BLOCK_0_HASH=$(curl -s --location --request POST "localhost:9500" \
			  --header "Content-Type: application/json" \
			  --data "{\"jsonrpc\":\"2.0\",\"method\":\"hmy_getBlockByNumber\",\"params\":[\"0x1\", true],\"id\":1}" | jq -r '.result.hash')

		echo "BLOCK0HASH:"
		echo "$BLOCK_0_HASH"

		SIGNED_RAW_TRANSACTION=$(node ../dapp-examples/nodejs/apiTestSign.js localnet)
		echo "RAWTX"
		echo "$SIGNED_RAW_TRANSACTION"
		TRANSACTION_HASH=$(curl  --location --request POST "localhost:9500" \
			  --header "Content-Type: application/json" \
			    --data "{\"jsonrpc\":\"2.0\",\"method\":\"hmy_sendRawTransaction\",\"params\":[\""$SIGNED_RAW_TRANSACTION"\"],\"id\":1}" | jq -r '.result')
		echo "TRANSACTION_HASH:"
		echo $TRANSACTION_HASH
		sleep 20s
		TRANSACTION=$(curl --location --request POST "http://localhost:9500" \
			  --header "Content-Type: application/json" \
			  --data "{\"jsonrpc\":\"2.0\",\"method\":\"hmy_getTransactionByHash\",\"params\":[\"$TRANSACTION_HASH\"],\"id\":1}")

		echo "TRANSACTION:"
		echo "$TRANSACTION"

		TRANSACTION_BLOCK_HASH=$(echo $TRANSACTION | jq -r '.result.blockHash')
		TRANSACTION_BLOCK_NUMBER=$(echo $TRANSACTION | jq -r '.result.blockNumber')
		TRANSACTION_INDEX=$(echo $TRANSACTION | jq -r '.result.transactionIndex')

		TRANSACTION_BLOCK_ID=$(( $TRANSACTION_BLOCK_NUMBER ))
		echo TRANSACTION_BLOCK_ID
		echo $TRANSACTION_BLOCK_ID
		echo "TRANSACTION_BLOCK_HASH:"
		echo $TRANSACTION_BLOCK_HASH

		echo "TRANSACTION_BLOCK_NUMBER:"
		echo "$TRANSACTION_BLOCK_NUMBER"

		echo "TRANSACTION_INDEX:"
		echo $TRANSACTION_INDEX
		;;
	v)
		VERBOSE="TRUE"
		;;
	p)
		PRETTY="TRUE"
		;;
	esac
		done

if [ $OPTIND -eq 1 ]; then echo "No options were passed, -l for localnet, -b for betanet, -v to view logs of either"; exit;  fi

declare -A GETDATA=( [GET_blocks]="blocks?from=$TRANSACTION_BLOCK_ID&to=$TRANSACTION_BLOCK_ID" [GET_tx]="tx?id=0" [GET_address]="address?id=0" [GET_node-count]="node-count" [GET_shard]="shard?id=0" [GET_committee]="committee?shard_id=0&epoch=0" )
declare -A POSTDATA

if [ "$NETWORK" == "localnet" ]; then
	POSTDATA[hmy_getBlockByHash]="hmy_getBlockByHash\",\"params\":[\"$TRANSACTION_BLOCK_HASH\", true]"
	POSTDATA[hmy_getBlockByNumber]="hmy_getBlockByNumber\",\"params\":[\"$TRANSACTION_BLOCK_NUMBER\", true]"
	POSTDATA[hmy_getBlockTransactionCountByHash]="hmy_getBlockTransactionCountByHash\",\"params\":[\"$TRANSACTION_BLOCK_HASH\"]"
	POSTDATA[hmy_getBlockTransactionCountByNumber]="hmy_getBlockTransactionCountByNumber\",\"params\":[\"$TRANSACTION_BLOCK_NUMBER\"]"
	POSTDATA[hmy_getCode]="hmy_getCode\",\"params\":[\"0x08AE1abFE01aEA60a47663bCe0794eCCD5763c19\", \"latest\"]"
	POSTDATA[hmy_getTransactionByBlockHashAndIndex]="hmy_getTransactionByBlockHashAndIndex\",\"params\":[\"$TRANSACTION_BLOCK_HASH\", \"$TRANSACTION_INDEX\"]"
	POSTDATA[hmy_getTransactionByBlockNumberAndIndex]="hmy_getTransactionByBlockNumberAndIndex\",\"params\":[\"$TRANSACTION_BLOCK_NUMBER\", \"$TRANSACTION_INDEX\"]"
	POSTDATA[hmy_getTransactionByHash]="hmy_getTransactionByHash\",\"params\":[\"$TRANSACTION_HASH\"]"
	POSTDATA[hmy_getTransactionReceipt]="hmy_getTransactionReceipt\",\"params\":[\"$TRANSACTION_HASH\"]"
	POSTDATA[hmy_syncing]="hmy_syncing\",\"params\":[]"
	POSTDATA[net_peerCount]="net_peerCount\",\"params\":[]"
	POSTDATA[hmy_getBalance]="hmy_getBalance\",\"params\":[\"one18t4yj4fuutj83uwqckkvxp9gfa0568uc48ggj7\", \"latest\"]"
	POSTDATA[hmy_getStorageAt]="hmy_getStorageAt\",\"params\":[\"0xD7Ff41CA29306122185A07d04293DdB35F24Cf2d\", \"0\", \"latest\"]"
	POSTDATA[hmy_getAccountNonce]="hmy_getAccountNonce\",\"params\":[\"0x806171f95C5a74371a19e8a312c9e5Cb4E1D24f6\", \"latest\"]"
	POSTDATA[hmy_sendRawTransaction]="hmy_sendRawTransaction\",\"params\":[\"$SIGNED_RAW_TRANSACTION\"]"
	POSTDATA[hmy_getLogs]="hmy_getLogs\", \"params\":[{\"BlockHash\": \"$TRANSACTION_BLOCK_HASH\"}]"
	POSTDATA[hmy_getFilterChanges]="hmy_getFilterChanges\", \"params\":[\"0x58010795a282878ed0d61da72a14b8b0\"]"
	POSTDATA[hmy_newPendingTransactionFilter]="hmy_newPendingTransactionFilter\", \"params\":[]"
	POSTDATA[hmy_newBlockFilter]="hmy_newBlockFilter\", \"params\":[]"
	POSTDATA[hmy_newFilter]="hmy_newFilter\", \"params\":[{\"BlockHash\": \"0x5725b5b2ab28206e7256a78cda4f9050c2629fd85110ffa54eacd2a13ba68072\"}]"
	POSTDATA[hmy_call]="hmy_call\", \"params\":[{\"to\": \"0x08AE1abFE01aEA60a47663bCe0794eCCD5763c19\"}, \"latest\"]"
	POSTDATA[hmy_gasPrice]="hmy_gasPrice\",\"params\":[]"
	POSTDATA[hmy_blockNumber]="hmy_blockNumber\",\"params\":[]"
	POSTDATA[net_version]="net_version\",\"params\":[]"
	POSTDATA[hmy_protocolVersion]="hmy_protocolVersion\",\"params\":[]"
fi

if [ "$NETWORK" == "betanet" ]; then
	POSTDATA[hmy_getBlockByHash]="hmy_getBlockByHash\",\"params\":[\"$TRANSACTION_BLOCK_HASH\", true]"
	POSTDATA[hmy_getBlockByNumber]="hmy_getBlockByNumber\",\"params\":[\"$TRANSACTION_BLOCK_NUMBER\", true]"
	POSTDATA[hmy_getBlockTransactionCountByHash]="hmy_getBlockTransactionCountByHash\",\"params\":[\"$TRANSACTION_BLOCK_HASH\"]"
	POSTDATA[hmy_getBlockTransactionCountByNumber]="hmy_getBlockTransactionCountByNumber\",\"params\":[\"$TRANSACTION_BLOCK_NUMBER\"]"
	POSTDATA[hmy_getCode]="hmy_getCode\",\"params\":[\"0x08AE1abFE01aEA60a47663bCe0794eCCD5763c19\", \"latest\"]"
	POSTDATA[hmy_getTransactionByBlockHashAndIndex]="hmy_getTransactionByBlockHashAndIndex\",\"params\":[\"$TRANSACTION_BLOCK_HASH\", \"$TRANSACTION_INDEX\"]"
	POSTDATA[hmy_getTransactionByBlockNumberAndIndex]="hmy_getTransactionByBlockNumberAndIndex\",\"params\":[\"$TRANSACTION_BLOCK_NUMBER\", \"$TRANSACTION_INDEX\"]"
	POSTDATA[hmy_getTransactionByHash]="hmy_getTransactionByHash\",\"params\":[\"$TRANSACTION_HASH\"]"
	POSTDATA[hmy_getTransactionReceipt]="hmy_getTransactionReceipt\",\"params\":[\"$TRANSACTION_HASH\"]"
	POSTDATA[hmy_syncing]="hmy_syncing\",\"params\":[]"
	POSTDATA[net_peerCount]="net_peerCount\",\"params\":[]"
	POSTDATA[hmy_getBalance]="hmy_getBalance\",\"params\":[\"one18t4yj4fuutj83uwqckkvxp9gfa0568uc48ggj7\", \"latest\"]"
	POSTDATA[hmy_getStorageAt]="hmy_getStorageAt\",\"params\":[\"0xD7Ff41CA29306122185A07d04293DdB35F24Cf2d\", \"0\", \"latest\"]"
	POSTDATA[hmy_getAccountNonce]="hmy_getAccountNonce\",\"params\":[\"0x806171f95C5a74371a19e8a312c9e5Cb4E1D24f6\", \"latest\"]"
	POSTDATA[hmy_sendRawTransaction]="hmy_sendRawTransaction\",\"params\":[\"$SIGNED_RAW_TRANSACTION\"]"
	POSTDATA[hmy_getLogs]="hmy_getLogs\", \"params\":[{\"BlockHash\": \"$TRANSACTION_BLOCK_HASH\"}]"
	POSTDATA[hmy_getFilterChanges]="hmy_getFilterChanges\", \"params\":[\"0x58010795a282878ed0d61da72a14b8b0\"]"
	POSTDATA[hmy_newPendingTransactionFilter]="hmy_newPendingTransactionFilter\", \"params\":[]"
	POSTDATA[hmy_newBlockFilter]="hmy_newBlockFilter\", \"params\":[]"
	POSTDATA[hmy_newFilter]="hmy_newFilter\", \"params\":[{\"BlockHash\": \"0x5725b5b2ab28206e7256a78cda4f9050c2629fd85110ffa54eacd2a13ba68072\"}]"
	POSTDATA[hmy_call]="hmy_call\", \"params\":[{\"to\": \"0x08AE1abFE01aEA60a47663bCe0794eCCD5763c19\"}, \"latest\"]"
	POSTDATA[hmy_gasPrice]="hmy_gasPrice\",\"params\":[]"
	POSTDATA[hmy_blockNumber]="hmy_blockNumber\",\"params\":[]"
	POSTDATA[net_version]="net_version\",\"params\":[]"
	POSTDATA[hmy_protocolVersion]="hmy_protocolVersion\",\"params\":[]"
fi

declare -A RESPONSES

RESPONSES[GET_blocks]=""
RESPONSES[GET_tx]=""
RESPONSES[GET_address]=""
RESPONSES[GET_node-count]=""
RESPONSES[GET_shard]=""
RESPONSES[GET_committee]=""
RESPONSES[hmy_getBlockByHash]=""
RESPONSES[hmy_getBlockByNumber]=""
RESPONSES[hmy_getBlockTransactionCountByHash]=""
RESPONSES[hmy_getBlockTransactionCountByNumber]=""
RESPONSES[hmy_getCode]=""
RESPONSES[hmy_getTransactionByBlockHashAndIndex]=""
RESPONSES[hmy_getTransactionByBlockNumberAndIndex]=""
RESPONSES[hmy_getTransactionByHash]=""
RESPONSES[hmy_getTransactionReceipt]=""
RESPONSES[hmy_syncing]=""
RESPONSES[net_peerCount]=""
RESPONSES[hmy_getBalance]=""
RESPONSES[hmy_getStorageAt]=""
RESPONSES[hmy_getAccountNonce]=""
RESPONSES[hmy_sendRawTransaction]=""
RESPONSES[hmy_getLogs]=""
RESPONSES[hmy_getFilterChanges]=""
RESPONSES[hmy_newPendingTransactionFilter]=""
RESPONSES[hmy_newBlockFilter]=""
RESPONSES[hmy_newFilter]=""
RESPONSES[hmy_call]=""
RESPONSES[hmy_gasPrice]=""
RESPONSES[hmy_blockNumber]=""
RESPONSES[net_version]=""
RESPONSES[hmy_protocolVersion]=""

### Processes GET requests and stores reponses in RESPONSES ###
function GET_requests() {
	for K in "${!GETDATA[@]}";
	do
		RESPONSES[$K]=$(curl -s --location --request GET "${PORT[GET]}${GETDATA[$K]}" \
	  		--header "Content-Type: application/json" \
			--data "")
	done
}

### Processes POST requests and stores reponses in RESPONSES ###
function POST_requests() {
	for K in "${!POSTDATA[@]}";
	do
		RESPONSES[$K]="$(curl -s --location --request POST "${PORT[POST]}" \
	  		--header "Content-Type: application/json" \
			--data "{\"jsonrpc\":\"2.0\",\"method\":\"${POSTDATA[$K]},\"id\":1}")"
	done
}

function log_API_responses() {
	for K in "${!GETDATA[@]}";
	do
		echo "${yellow}$K"
		echo "${blue}REQUEST:"
		echo "${white}${GETDATA[$K]}"
		echo "${blue}RESPONSE:" ${white}
		echo ${RESPONSES[$K]} #| jq .
		echo
		echo
	done
	for K in "${!POSTDATA[@]}";
	do
		echo "${yellow}$K"
		echo "${blue}REQUEST:"
		echo "${white}${POSTDATA[$K]}"
		echo "${blue}RESPONSE: $white"
		echo ${RESPONSES[$K]} #| jq .
		echo
		echo
	done
}

GET_requests
POST_requests
### BASIC QUERY TESTS ###

function Explorer_getBlock_test() {
	TESTS_RAN=$(( TESTS_RAN + 1 ))
	echo "GET blocks(explorer) test:"
	response_test ${RESPONSES[GET_blocks]}
	if [ "$?"  == "1" ]; then
		BLOCKBYIDHASH=$(echo ${RESPONSES[GET_blocks]} | jq -r .[0].id)
		if [ "$BLOCKBYIDHASH" != "null" ]; then
			if [ "$BLOCKBYIDHASH" == "$TRANSACTION_BLOCK_HASH" ]; then
				TESTS_PASSED=$(( TESTS_PASSED + 1 ))
				echo ${green}BLOCK HASH MATCHES TX${reset}
				echo
				return
			fi
		fi
		echo ${red}BLOCK HASH DOES NOT MATCH TX OR IS NULL${reset}
	fi
	echo
}

#Needs updating - wtf does getTx do - no arguments?
function Explorer_getTx_test() {
	TESTS_RAN=$(( TESTS_RAN + 1 ))
	echo "GET tx(explorer) test:"
	response_test ${RESPONSES[GET_tx]}
	if [ "$?" == "1" ]; then
		TX_HASH=$(echo ${RESPONSES[GET_tx]} | jq -r .id) # fix agrs to jq
		if [ "$TX_HASH" != "null" ]; then
			if [ "$TX_HASH" == "$TX_HASH" ]; then
				TESTS_PASSED=$(( TESTS_PASSED + 1 ))
				echo ${green}BLOCK HASH MATCHES TX${reset}
				echo
				return
			fi
		fi
		echo ${red}BLOCK HASH DOES NOT MATCH TX OR IS NULL${reset}
	fi
	echo
}

function Explorer_getExplorerNodeAdress_test() {
	TESTS_RAN=$(( TESTS_RAN + 1 ))
	echo "GET address(explorer) test:"
	response_test ${RESPONSES[GET_address]}
	[ "$?" == "1" ] && TESTS_PASSED=$(( TESTS_PASSED + 1 ))
	echo
}

function Explorer_getExplorerNode_test() {
	TESTS_RAN=$(( TESTS_RAN + 1 ))
	echo "GET node-count(explorer) test:"
	response_test ${RESPONSES[GET_node-count]}
	if [ ${RESPONSES[GET_node-count]}="2" ]; then
		echo ${green}SANE VALUE, 2 explorer nodes reported $reset
		TESTS_PASSED=$(( TESTS_PASSED + 1 ))
	else
		echo ${red}non 2 explorer nodes reported $reset
	fi
	echo
}

function Explorer_getShard_test() {
	TESTS_RAN=$(( TESTS_RAN + 1 ))
	echo "GET shard(explorer) test:"
	response_test ${RESPONSES[GET_shard]}
	[ "$?" == "1" ] && TESTS_PASSED=$(( TESTS_PASSED + 1 ))
	echo
}

function Explorer_getCommitte_test() {
	TESTS_RAN=$(( TESTS_RAN + 1 ))
	echo "GET committe(explorer) test:"
	response_test ${RESPONSES[GET_committee]}
	[ "$?" == "1" ] && TESTS_PASSED=$(( TESTS_PASSED + 1 ))
	echo
}

### API POST REQUESTS ###

function API_getBlockByNumber_test() {
	TESTS_RAN=$(( TESTS_RAN + 1 ))
	echo "POST hmy_getBlockByNumber test:"
	response_test ${RESPONSES[hmy_getBlockByNumber]}
	BLOCKBYNUMBERHASH=$(echo ${RESPONSES[hmy_getBlockByNumber]} | jq -r '.result.hash')

	if [ "$BLOCKBLOCKBYNUMBERHASH" != "null" ]; then
		if [ "$BLOCKBYNUMBERHASH" == "$TRANSACTION_BLOCK_HASH" ]; then
			TESTS_PASSED=$(( TESTS_PASSED + 1 ))
			echo ${green}BLOCK HASH MATCHES TX${reset}
			echo
			return
		fi
	fi
	echo ${red}BLOCK HASH DOES NOT MATCH TX OR IS NULL${reset}
	echo
}

function API_getBlockByHash_test() {
	TESTS_RAN=$(( TESTS_RAN + 1 ))
	echo "POST hmy_getBlockByHash test:"
	response_test ${RESPONSES[hmy_getBlockByHash]}
	BLOCKBYHASHHASH=$(echo ${RESPONSES[hmy_getBlockByHash]} | jq -r '.result.hash')
	if [ "$BLOCKBYHASHBYHASH" != "null" ]; then
		if [ "$BLOCKBYHASHHASH" == "$TRANSACTION_BLOCK_HASH" ]; then
			TESTS_PASSED=$(( TESTS_PASSED + 1 ))
			echo ${green}BLOCK HASH MATCHES TX${reset}
			echo
			return
		fi
	fi
	echo ${red}BLOCK HASH DOES NOT MATCH TX OR IS NULL${reset}
	echo
}

function API_getBlockTransactionCountByHash_test() {
	TESTS_RAN=$(( TESTS_RAN + 1 ))
	echo "POST hmy_getBlockTransactionCountByHash test:"
	response_test ${RESPONSES[hmy_getBlockTransactionCountByHash]}
	TRANSACTIONCOUNTBYHASH=$(echo ${RESPONSES[hmy_getBlockTransactionCountByHash]} | jq -r '.result')
	TRANSACTIONCOUNTBYHASH=$(( TRANSACTIONCOUNTBYHASH ))
	if [ "$TRANSACTIONCOUNTBYHASH" != "null" ]; then
		if [ $TRANSACTIONCOUNTBYHASH -gt 0 ]; then
			TESTS_PASSED=$(( TESTS_PASSED + 1 ))
			echo ${green}NON ZERO TRANSACTION COUNT IN BLOCK${reset}
			echo
			return
		fi
	fi
	echo ${red}INVALID TRANSACTION COUNT IN BLOCK${reset}
	echo
}

function API_getBlockTransactionCountByNumber_test() {
	TESTS_RAN=$(( TESTS_RAN + 1 ))
	echo "POST hmy_getBlockTransactionCountByNumber test:"
	response_test ${RESPONSES[hmy_getBlockTransactionCountByNumber]}
	TRANSACTIONCOUNTBYNUMBER=$(echo ${RESPONSES[hmy_getBlockTransactionCountByNumber]} | jq -r '.result')
	TRANSACTIONCOUNTBYNUMBER=$(( TRANSACTIONCOUNTBYNUMBER ))
	if [ "$BLOCKBYHASH" != "null" ]; then
		if [ $TRANSACTIONCOUNTBYNUMBER -gt 0 ]; then
			TESTS_PASSED=$(( TESTS_PASSED + 1 ))
			echo ${green}NON ZERO TRANSACTION COUNT IN BLOCK${reset}
			echo
			return
		fi
	fi
	echo ${red}NON NATURAL TRANSACTION COUNT IN BLOCK${reset}
	echo
}

function API_getCode_test() {
	TESTS_RAN=$(( TESTS_RAN + 1 ))
	echo "POST hmy_getCode test:"
	response_test ${RESPONSES[hmy_getCode]}
	[ "$?" == "1" ] && TESTS_PASSED=$(( TESTS_PASSED + 1 ))
	echo
}

function API_getTransactionByBlockHashAndIndex_test() {
	TESTS_RAN=$(( TESTS_RAN + 1 ))
	echo "POST hmy_getTransactionByBlockHashAndIndex test:"
	response_test ${RESPONSES[hmy_getTransactionByBlockHashAndIndex]}
	TRANSACTIONHASHBYHASHANDINDEX=$(echo ${RESPONSES[hmy_getTransactionByBlockHashAndIndex]} | jq -r '.result.hash')
	if [ "$TRANSACTIONHASHBYHASHANDINDEX" != "null" ]; then
		if [ "$TRANSACTIONHASHBYHASHANDINDEX" == "$TRANSACTION_HASH" ]; then
			TESTS_PASSED=$(( TESTS_PASSED + 1 ))
			echo ${green}TRANSACTION FROM BLOCKHASH AND INDEX MATCH${reset}
			echo
			return
		fi
	fi
	echo ${red} TRANSACTION FROM BLOCKHASH AND INDEX MATCH${reset}
	echo
}

function API_getTransactionByBlockNumberAndIndex_test() {
	TESTS_RAN=$(( TESTS_RAN + 1 ))
	echo "POST hmy_getTransactionByBlockNumberAndIndex test:"
	response_test ${RESPONSES[hmy_getTransactionByBlockNumberAndIndex]}
	TRANSACTIONHASHBYNUMBERANDINDEX=$(echo ${RESPONSES[hmy_getTransactionByBlockNumberAndIndex]} | jq -r '.result.hash')
	if [ "$TRANSACTIONHASHBYNUMBERANDINDEX" != "null" ]; then
		if [ "$TRANSACTIONHASHBYNUMBERANDINDEX" == "$TRANSACTION_HASH" ]; then
			TESTS_PASSED=$(( TESTS_PASSED + 1 ))
			echo ${green}TRANSACTION FROM BLOCKNUMBER AND INDEX MATCH${reset}
			echo
			return
		fi
	fi
	echo ${red} TRANSACTION FROM BLOCKNUMBER AND INDEX MISMATCH${reset}
	echo
}

function API_getTransactionByHash_test() {
	TESTS_RAN=$(( TESTS_RAN + 1 ))
	echo "POST hmy_getTransactionByHash test:"
	TX_HASH=$(echo ${RESPONSES[hmy_getTransactionByHash]} | jq -r '.result.hash')
	response_test ${RESPONSES[hmy_getTransactionByHash]}
	if [ "$TX_HASH" != "null" ]; then
		if [ "$TX_HASH" == "$TRANSACTION_HASH" ]; then
			TESTS_PASSED=$(( TESTS_PASSED + 1 ))
			echo ${green}TRANSACTION HASH MATCH${reset}
			echo
			return
		fi
	fi
	echo ${red} TRANSACTION HASH MISMATCH${reset}
	echo
}

function API_getTransactionReceipt_test() {
	TESTS_RAN=$(( TESTS_RAN + 1 ))
	echo "POST hmy_getTransactionReceipt test:"
	TX_HASH=$(echo ${RESPONSES[hmy_getTransactionReceipt]} | jq -r '.result.transactionHash')
	response_test ${RESPONSES[hmy_getTransactionReceipt]}
	if [ "$TX_HASH" != "null" ]; then
		if [ "$TX_HASH" == "$TRANSACTION_HASH" ]; then
			TESTS_PASSED=$(( TESTS_PASSED + 1 ))
			echo ${green}TRANSACTION HASH MATCH${reset}
			echo
			return
		fi
	fi
	echo ${red} TRANSACTION HASH MISMATCH${reset}
	echo
}

function API_syncing_test() {
	TESTS_RAN=$(( TESTS_RAN + 1 ))
	echo "POST hmy_syncing test:"
	response_test ${RESPONSES[hmy_syncing]}
	[ "$?" == "1" ] && TESTS_PASSED=$(( TESTS_PASSED + 1 ))
	echo
}

function API_netPeerCount_test() {
	TESTS_RAN=$(( TESTS_RAN + 1 ))
	echo "POST net_peerCount test:"
	response_test ${RESPONSES[net_peerCount]}
	[ "$?" == "1" ] && TESTS_PASSED=$(( TESTS_PASSED + 1 ))
	echo
}

function API_getBalance_test() {
	TESTS_RAN=$(( TESTS_RAN + 1 ))
	echo "POST hmy_getBalance test:"
	response_test ${RESPONSES[hmy_getBalance]}
	[ "$?" == "1" ] && TESTS_PASSED=$(( TESTS_PASSED + 1 ))
	echo
}

function API_getStorageAt_test() {
	TESTS_RAN=$(( TESTS_RAN + 1 ))
	echo "POST hmy_getStorageAt test:"
	response_test ${RESPONSES[hmy_getStorageAt]}
	[ "$?" == "1" ] && TESTS_PASSED=$(( TESTS_PASSED + 1 ))
	echo
}

function API_getAccountNonce_test() {
	TESTS_RAN=$(( TESTS_RAN + 1 ))
	echo "POST hmy_getAccountNonce test:"
	response_test ${RESPONSES[hmy_getAccountNonce]}
	[ "$?" == "1" ] && TESTS_PASSED=$(( TESTS_PASSED + 1 ))
	echo
}

function API_sendRawTransaction_test() {
	TESTS_RAN=$(( TESTS_RAN + 1 ))
	echo "POST hmy_sendRawTransaction test:"
	response_test ${RESPONSES[hmy_sendRawTransaction]}
	[ "$?" == "1" ] && TESTS_PASSED=$(( TESTS_PASSED + 1 ))
	echo
}

function API_getLogs_test() {
	TESTS_RAN=$(( TESTS_RAN + 1 ))
	echo "POST hmy_getLogs test:"
	response_test ${RESPONSES[hmy_getLogs]}
	[ "$?" == "1" ] && TESTS_PASSED=$(( TESTS_PASSED + 1 ))
	echo
}

function API_getFilterChanges_test() {
	TESTS_RAN=$(( TESTS_RAN + 1 ))
	echo "POST hmy_getFilterChanges test:"
	response_test ${RESPONSES[hmy_getFilterChanges]}
	[ "$?" == "1" ] && TESTS_PASSED=$(( TESTS_PASSED + 1 ))
	echo
}

function API_newPendingTransactionFilter_test() {
	TESTS_RAN=$(( TESTS_RAN + 1 ))
	echo "POST hmy_sendRawTransaction test:"
	response_test ${RESPONSES[hmy_newPendingTransactionFilter]}
	[ "$?" == "1" ] && TESTS_PASSED=$(( TESTS_PASSED + 1 ))
	echo
}

function API_newBlockFilter_test() {
	TESTS_RAN=$(( TESTS_RAN + 1 ))
	echo "POST hmy_newBlockFilter test:"
	response_test ${RESPONSES[hmy_newBlockFilter]}
	[ "$?" == "1" ] && TESTS_PASSED=$(( TESTS_PASSED + 1 ))
	echo
}

function API_newFilter_test() {
	TESTS_RAN=$(( TESTS_RAN + 1 ))
	echo "POST hmy_newFilter test:"
	response_test ${RESPONSES[hmy_newFilter]}
	[ "$?" == "1" ] && TESTS_PASSED=$(( TESTS_PASSED + 1 ))
	echo
}

function API_call_test() {
	TESTS_RAN=$(( TESTS_RAN + 1 ))
	echo "POST hmy_call test:"
	response_test ${RESPONSES[hmy_call]}
	[ "$?" == "1" ] && TESTS_PASSED=$(( TESTS_PASSED + 1 ))
	echo
}

function API_gasPrice_test() {
	TESTS_RAN=$(( TESTS_RAN + 1 ))
	echo "POST hmy_gasPrice test:"
	response_test ${RESPONSES[hmy_gasPrice]}
	if [ "$?" == "1" ]; then
		RESULT=$(echo ${RESPONSES[hmy_gasPrice]} | jq -r '.result')
		isHexTest $RESULT
		[ "$?" == "1" ] && TESTS_PASSED=$(( TESTS_PASSED + 1 ))
	fi
}

function API_blockNumber_test() {
	TESTS_RAN=$(( TESTS_RAN + 1 ))
	echo "POST hmy_blockNumber test:"
	response_test ${RESPONSES[hmy_blockNumber]}
	if [ "$?" == "1" ]; then
		RESULT=$(echo ${RESPONSES[hmy_blockNumber]} | jq -r '.result')
		isHexTest $RESULT
		[ "$?" == "1" ] && TESTS_PASSED=$(( TESTS_PASSED + 1 ))
	fi
}

function API_net_version_test() {
	TESTS_RAN=$(( TESTS_RAN + 1 ))
	echo "POST net_version test:"
	response_test ${RESPONSES[net_version]}
	[ "$?" == "1" ] && TESTS_PASSED=$(( TESTS_PASSED + 1 ))
	echo
}

function API_protocolVersion_test() {
	TESTS_RAN=$(( TESTS_RAN + 1 ))
	echo "POST hmy_protocolVersion test:"
	response_test ${RESPONSES[hmy_protocolVersion]}
	[ "$?" == "1" ] && TESTS_PASSED=$(( TESTS_PASSED + 1 ))
	echo
}

function run_tests() {
	echo "### TESTING RPC CALLS ###"
	echo
	### Calls to the individual API method test ###
	Explorer_getBlock_test
	Explorer_getTx_test
	Explorer_getExplorerNodeAdress_test
	Explorer_getExplorerNode_test
	Explorer_getShard_test
	Explorer_getCommitte_test
	API_getBlockByNumber_test
	API_getBlockByHash_test
	API_getBlockTransactionCountByHash_test
	API_getBlockTransactionCountByNumber_test
	API_getCode_test
	API_getTransactionByBlockHashAndIndex_test
	API_getTransactionByBlockNumberAndIndex_test
	API_getTransactionByHash_test
	API_getTransactionReceipt_test
	API_syncing_test
	API_netPeerCount_test
	API_getBalance_test
	API_getStorageAt_test
	API_getAccountNonce_test
	API_sendRawTransaction_test
	API_getLogs_test
	API_getFilterChanges_test
	API_newPendingTransactionFilter_test
	API_sendRawTransaction_test
	API_newBlockFilter_test
	API_newFilter_test
	API_call_test
	API_gasPrice_test
	API_blockNumber_test
	API_net_version_test
	API_protocolVersion_test

	TESTS_FAILED=$(( $TESTS_RAN - $TESTS_PASSED ))
	echo -n ${red}
	[ $TESTS_FAILED -eq 0 ] && echo -n ${green}
	echo "PASSED $TESTS_PASSED/$TESTS_RAN: $TESTS_FAILED TESTS FAILED"${reset}
}



if [ "$VERBOSE" == "TRUE" ]; then
	log_API_responses
fi
### BETANET TESTS ###

run_tests
