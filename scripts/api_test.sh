#!/bin/bash
# This Script is for Testing the API functionality on both local and betanet.
# -l to run localnet, -b to run betanet(mutually exclusive)
# -v to see returns from each request
# Right now only tests whether a response is recieved
# TODO(theo) add tests to check the content of messages to verify their sanity
# TODO(theo) tet interaction with the blockchain by sending transactions and vreifying balances

ALL_PASS="TRUE"
VERBOSE="FALSE"

### SETUP COMMANDLINE FLAGS ###
while getopts "lbv" OPTION; do
	case $OPTION in
	b)
		NETWORK="betanet"
		declare -A PORT=( [POST]="http://l0.b.hmny.io:9500" [GET]="107.21.71.80:5000/" )
		BLOCK_0_HASH=$(curl -s --location --request POST "http://l0.b.hmny.io:9500" \
			  --header "Content-Type: application/json" \
			  --data "{\"jsonrpc\":\"2.0\",\"method\":\"hmy_getBlockByNumber\",\"params\":[\"0x1\", true],\"id\":1}" | jq -r '.result.hash')
		;;
	l)
		NETWORK="localnet"
		declare -A PORT=( [POST]="localhost:9500/" [GET]="localhost:5099/" )
		BLOCK_0_HASH=$(curl -s --location --request POST "localhost:9500" \
			  --header "Content-Type: application/json" \
			  --data "{\"jsonrpc\":\"2.0\",\"method\":\"hmy_getBlockByNumber\",\"params\":[\"0x1\", true],\"id\":1}" | jq -r '.result.hash')
		;;
	v)
		VERBOSE="TRUE"
		;;
	esac
		done

declare -A GETDATA=( [GET_blocks]="blocks?from=0&to=0" [GET_tx]="tx?id=0" [GET_address]="address?id=0" [GET_node-count]="node-count" [GET_shard]="shard?id=0" [GET_committee]="committee?shard_id=0&epoch=0" )
echo $BLOCK_0_HASH
declare -A POSTDATA

if [ "$NETWORK" == "localnet" ]; then
	POSTDATA[hmy_getBlockByHash]="hmy_getBlockByHash\",\"params\":[\"$BLOCK_0_HASH\", true]"
	POSTDATA[hmy_getBlockByNumber]="hmy_getBlockByNumber\",\"params\":[\"0x1\", true]"
	POSTDATA[hmy_getBlockTransactionCountByHash]="hmy_getBlockByHash\",\"params\":[\"$BLOCK_0_HASH\", true]"
	POSTDATA[hmy_getBlockTransactionCountByNumber]="hmy_getBlockByNumber\",\"params\":[\"0x1\", true]"
	POSTDATA[hmy_getCode]="hmy_getCode\",\"params\":[\"0x08AE1abFE01aEA60a47663bCe0794eCCD5763c19\", \"latest\"]"
	POSTDATA[hmy_getTransactionByBlockHashAndIndex]="hmy_getTransactionByBlockHashAndIndex\",\"params\":[\"$BLOCK_0_HASH\", \"0x0\"]"
	POSTDATA[hmy_getTransactionByBlockNumberAndIndex]="hmy_getTransactionByBlockNumberAndIndex\",\"params\":[\"0x0\", \"0x0\"]"
	POSTDATA[hmy_getTransactionByHash]="hmy_getTransactionByHash\",\"params\":[\"0xa7bb2c7cbfe4f5d6cf061aacd9d0dce7660d1f82da63dd7c76d9e856c1dc0278\"] "
	POSTDATA[hmy_getTransactionReceipt]="hmy_getTransactionReceipt\",\"params\":[\"0x17452d6c153f3e42dae114f63fd0a9dab9ce9cc2a4bb4400823762f60787c3bf\"]"
	POSTDATA[hmy_syncing]="hmy_syncing\",\"params\":[]"
	POSTDATA[net_peerCount]="net_peerCount\",\"params\":[]"
	POSTDATA[hmy_getBalance]="hmy_getBalance\",\"params\":[\"0xD7Ff41CA29306122185A07d04293DdB35F24Cf2d\", \"latest\"]"
	POSTDATA[hmy_getStorageAt]="hmy_getStorageAt\",\"params\":[\"0xD7Ff41CA29306122185A07d04293DdB35F24Cf2d\", \"0\", \"latest\"]"
	POSTDATA[hmy_getTransactionCount]="hmy_getTransactionCount\",\"params\":[\"0x806171f95C5a74371a19e8a312c9e5Cb4E1D24f6\", \"latest\"]"
	POSTDATA[hmy_sendRawTransaction]="hmy_sendRawTransaction\",\"params\":[\"0xf869808082520880809410a02a0a6e95a676ae23e2db04bea3d1b8b7ca2e880de0b6b3a7640000801ba0c8d0c5390086999b5b5a93373953c3c94b44dc8fd06d88a421a7c2461e9e4482a0730d7859d1e3109d499bcd75f00700729b9bc17b03940da4f84b6ea784f51eb1\"]"
	POSTDATA[hmy_getLogs]="hmy_getLogs\", \"params\":[{\"BlockHash\": \"0x5725b5b2ab28206e7256a78cda4f9050c2629fd85110ffa54eacd2a13ba68072\"}]"
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
	POSTDATA[hmy_getBlockByHash]="hmy_getBlockByHash\",\"params\":[\"$BETANET_BLOCK_0_HASH\", true]"
	POSTDATA[hmy_getBlockByNumber]="hmy_getBlockByNumber\",\"params\":[\"0x1\", true]"
	POSTDATA[hmy_getBlockTransactionCountByHash]="hmy_getBlockByHash\",\"params\":[\"$BETANET_BLOCK_0_HASH\", true]"
	POSTDATA[hmy_getBlockTransactionCountByNumber]="hmy_getBlockByNumber\",\"params\":[\"0x1\", true]"
	POSTDATA[hmy_getCode]="hmy_getCode\",\"params\":[\"0x08AE1abFE01aEA60a47663bCe0794eCCD5763c19\", \"latest\"]"
	POSTDATA[hmy_getTransactionByBlockHashAndIndex]="hmy_getTransactionByBlockHashAndIndex\",\"params\":[\"$BETANET_BLOCK_0_HASH\", \"0x0\"]"
	POSTDATA[hmy_getTransactionByBlockNumberAndIndex]="hmy_getTransactionByBlockNumberAndIndex\",\"params\":[\"0x0\", \"0x0\"]"
	POSTDATA[hmy_getTransactionByHash]="hmy_getTransactionByHash\",\"params\":[\"0xa7bb2c7cbfe4f5d6cf061aacd9d0dce7660d1f82da63dd7c76d9e856c1dc0278\"]"
	POSTDATA[hmy_getTransactionReceipt]="hmy_getTransactionReceipt\",\"params\":[\"0x17452d6c153f3e42dae114f63fd0a9dab9ce9cc2a4bb4400823762f60787c3bf\"]"
	POSTDATA[hmy_syncing]="hmy_syncing\",\"params\":[]"
	POSTDATA[net_peerCount]="net_peerCount\",\"params\":[]"
	POSTDATA[hmy_getBalance]="hmy_getBalance\",\"params\":[\"0xD7Ff41CA29306122185A07d04293DdB35F24Cf2d\", \"latest\"]"
	POSTDATA[hmy_getStorageAt]="hmy_getStorageAt\",\"params\":[\"0xD7Ff41CA29306122185A07d04293DdB35F24Cf2d\", \"0\", \"latest\"]"
	POSTDATA[hmy_getTransactionCount]="hmy_getTransactionCount\",\"params\":[\"0x806171f95C5a74371a19e8a312c9e5Cb4E1D24f6\", \"latest\"]"
	POSTDATA[hmy_sendRawTransaction]="hmy_sendRawTransaction\",\"params\":[\"0xf869808082520880809410a02a0a6e95a676ae23e2db04bea3d1b8b7ca2e880de0b6b3a7640000801ba0c8d0c5390086999b5b5a93373953c3c94b44dc8fd06d88a421a7c2461e9e4482a0730d7859d1e3109d499bcd75f00700729b9bc17b03940da4f84b6ea784f51eb1\"]"
	POSTDATA[hmy_getLogs]="hmy_getLogs\", \"params\":[{\"BlockHash\": \"0x5725b5b2ab28206e7256a78cda4f9050c2629fd85110ffa54eacd2a13ba68072\"}]"
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
RESPONSES[hmy_getTransactionCount]=""
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
	echo "${!GETDATA[@]}"
	for K in "${!GETDATA[@]}"; 
	do
		RESPONSES[$K]=$(curl -s --location --request GET "${PORT[GET]}${GETDATA[$K]}" \
	  		--header "Content-Type: application/json" \
			--data "")

		echo "$K"
		echo ${RESPONSES[$K]}
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

GET_requests
POST_requests

function response_test() {
	if [ "$1" != "" ]; then
		echo "${green}RESPONSE RECIEVED${reset}"
	else
		echo "${red}NO RESPONSE${reset}"
		ALL_PASS="FALSE"
	fi
}

function Explorer_getBlock_test() {
	echo "GET blocks(explorer) test:"
	response_test ${RESPONSES[GET_blocks]}
	echo
}

function Explorer_getTx_test() {
	echo "GET tx(explorer) test:"
	response_test ${RESPONSES[GET_tx]}
	echo
}

function Explorer_getExplorerNodeAdress_test() {
	echo "GET address(explorer) test:"
	response_test ${RESPONSES[GET_address]}
	echo
}

function Explorer_getExplorerNode_test() {
	echo "GET node-count(explorer) test:"
	response_test ${RESPONSES[GET_node-count]}
	echo
}

function Explorer_getShard_test() {
	echo "GET shard(explorer) test:"
	response_test ${RESPONSES[GET_shard]}
	echo
}

function Explorer_getCommitte_test() {
	echo "GET committe(explorer) test:"
	response_test ${RESPONSES[GET_committee]}
	echo
}


function API_getBlockByNumber_test() {
	echo "POST hmy_getBlockByNumber test:"
	response_test ${RESPONSES[hmy_getBlockByNumber]}
	echo
}

function API_getBlockByHash_test() {
	echo "POST hmy_getBlockByHash test:"
	response_test ${RESPONSES[hmy_getBlockByHash]}
	echo
}

function API_getBlockTransactionCountByHash_test() {
	echo "POST hmy_getBlockTransactionCountByHash test:"
	response_test ${RESPONSES[hmy_getBlockTransactionCountByHash]}
	echo
}

function API_getBlockTransactionCountByNumber_test() {
	echo "POST hmy_getBlockTransactionCountByNumber test:"
	response_test ${RESPONSES[hmy_getBlockTransactionCountByNumber]}
	echo
}

function API_getCode_test() {
	echo "POST hmy_getCode test:"
	response_test ${RESPONSES[hmy_getCode]}
	echo
}
##
function API_getTransactionByBlockHashAndIndex_test() {
	echo "POST hmy_getTransactionByBlockHashrAndIndex test:"
	response_test ${RESPONSES[hmy_getTransactionByBlockHashAndIndex]}
	echo
}

function API_getTransactionByBlockNumberAndIndex_test() {
	echo "POST hmy_getTransactionByBlockNumberAndIndex test:"
	response_test ${RESPONSES[hmy_getTransactionByBlockNumberAndIndex]}
	echo
}

function API_getTransactionByHash_test() {
	echo "POST hmy_getTransactionByHash test:"
	response_test ${RESPONSES[hmy_getTransactionByHash]}
	echo
}

function API_getTransactionReceipt_test() {
	echo "POST hmy_getTransactionReceipt test:"
	response_test ${RESPONSES[hmy_getTransactionReceipt]}
	echo
}

function API_syncing_test() {
	echo "POST hmy_syncing test:"
	response_test ${RESPONSES[hmy_syncing]}
	echo
}

function API_netPeerCount_test() {
	echo "POST net_peerCount test:"
	response_test ${RESPONSES[net_peerCount]}
	echo
}

function API_getBalance_test() {
	echo "POST hmy_getBalance test:"
	response_test ${RESPONSES[hmy_getBalance]}
	echo
}

function API_getStorageAt_test() {
	echo "POST hmy_getStorageAt test:"
	response_test ${RESPONSES[hmy_getStorageAt]}
	echo
}

function API_getTransactionCount_test() {
	echo "POST hmy_getTransactionCount test:"
	response_test ${RESPONSES[hmy_getTransactionCount]}
	echo
}

function API_sendRawTransaction_test() {
	echo "POST hmy_sendRawTransaction test:"
	response_test ${RESPONSES[hmy_sendRawTransaction]}
	echo
}

function API_getLogs_test() {
	echo "POST hmy_getLogs test:"
	response_test ${RESPONSES[hmy_getLogs]}
	echo
}

function API_getFilterChanges_test() {
	echo "POST hmy_getFilterChanges test:"
	response_test ${RESPONSES[hmy_getFilterChanges]}
	echo
}

function API_newPendingTransactionFilter_test() {
	echo "POST hmy_sendRawTransaction test:"
	response_test ${RESPONSES[hmy_newPendingTransactionFilter]}
	echo
}

function API_newBlockFilter_test() {
	echo "POST hmy_newBlockFilter test:"
	response_test ${RESPONSES[hmy_newBlockFilter]}
	echo
}

function API_newFilter_test() {
	echo "POST hmy_newFilter test:"
	response_test ${RESPONSES[hmy_newFilter]}
	echo
}

function API_call_test() {
	echo "POST hmy_call test:"
	response_test ${RESPONSES[hmy_call]}
	echo
}

function API_gasPrice_test() {
	echo "POST hmy_gasPrice test:"
	response_test ${RESPONSES[hmy_gasPrice]}
	echo
}

function API_blockNumber_test() {
	echo "POST hmy_blockNumber test:"
	response_test ${RESPONSES[hmy_blockNumber]}
	echo
}

function API_net_version_test() {
	echo "POST net_version test:"
	response_test ${RESPONSES[net_version]}
	echo
}

function API_protocolVersion_test() {
	echo "POST hmy_protocolVersion test:"
	response_test ${RESPONSES[hmy_protocolVersion]}
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
	API_getTransactionCount_test
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

	if [ $ALL_PASS == "TRUE" ]; then 
		echo ${green}"-------------"
		echo ${green}" TEST PASSED "
		echo ${green}"-------------"
		echo ${reset}
	else
		echo ${red}"-------------"
		echo ${red}" TEST FAILED "
		echo ${red}"-------------"
		echo ${reset}
	fi
}


red=`tput setaf 1`
green=`tput setaf 2`
reset=`tput sgr0`


if [ "$VERBOSE" == "TRUE" ]; then
	log_API_responses
fi
### BETANET TESTS ###

run_tests
