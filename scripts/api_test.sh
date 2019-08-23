#!/bin/bash
# This Script is for Testing the API functionality on both local and betanet.
# -l to run localnet, -b to run betanet(mutually exclusive)
# -v to see returns from each request
# Right now only tests whether a response is recieved
# TODO(theo) make tests individual functions
# TODO(theo) add tests to check the content of messages to verify their sanity
# TODO(theo) tet interaction with the blockchain by sending transactions and vreifying balances

ALL_PASS="TRUE"

VERBOSE="FALSE"
while getopts "lbv" OPTION; do
	case $OPTION in
	b)
		NETWORK="betanet"
		;;
	l)
		NETWORK="localnet"
		;;
	v)
		VERBOSE="TRUE"
		;;
				esac
		done

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
	response_test $BLOCK0_GET
	echo
}

function Explorer_getTx_test() {
	echo "GET tx(explorer) test:"
	response_test $TX0 
	echo

}

function Explorer_getExplorerAddress_test() {
	echo "GET tx(explorer) test:"
	response_test $TX0 
	echo

}

function Explorer_getExplorerNodeAdress_test() {
	echo "GET address(explorer) test:"
	response_test $ADDR0 "\n" 
	echo
}

function Explorer_getExplorerNode_test() {
	echo "GET node-count(explorer) test:"
	response_test $NODE_COUNT
	echo
}

function Explorer_getShard_test() {
	echo "GET shard(explorer) test:"
	response_test $SHARD0
	echo
}

function Explorer_getCommitte_test() {
	echo "GET committe(explorer) test:"
	response_test $COMMITTE0
	echo
}

function API_getBlockByNumber_test() {
	echo "POST hmy_getBlockByNumber test:"
	response_test $BLOCK0_POST
	echo
}

function API_getBlockByHash_test() {
	echo "POST hmy_getBlockByHash test:"
	response_test $BLOCK_0x1_BY_HASH
	echo
}

function API_getBlockTransactionCountByHash_test() {
	echo "POST hmy_getBlockTransactionCountByHash test:"
	response_test $BLOCK_0x1_TXCOUNT
	echo
}

function API_getBlockTransactionCountByNumber_test() {
	echo "POST hmy_getBlockTransactionCountByNumber test:"
	response_test $BLOCKNUM_0x1_TXCOUNT
	echo
}

function API_getCode_test() {
	echo "POST hmy_getCode test:"
	response_test $GETCODE_OUTPUT
	echo
}

function API_getTransactionByBlockHashAndIndex_test() {
	echo "POST hmy_getTransactionByBlockHashrAndIndex test:"
	response_test $BLOCK_X_TX0
	echo
}

function API_getTransactionByBlockNumberAndIndex_test() {
	echo "POST hmy_getTransactionByBlockNumberAndIndex test:"
	response_test $BLOCK_0x4_TX0
	echo
}

function API_getTransactionByHash_test() {
	echo "POST hmy_getTransactionByHash test:"
	response_test $RAND_TX
	echo
}

function API_getTransactionReceipt_test() {
	echo "POST hmy_getTransactionReceipt test:"
	response_test $RAND_TX_RECEIPT
	echo
}

function API_syncing_test() {
	echo "POST hmy_syncing test:"
	response_test $SYNCING_OUTPUT
	echo
}

function API_netPeerCount_test() {
	echo "POST net_peerCount test:"
	response_test $NET_PEER_COUNT
	echo
}

function API_getBalance_test() {
	echo "POST hmy_getBalance test:"
	response_test $BALANCE_0xD7F
	echo
}

function API_getStorageAt_test() {
	echo "POST hmy_getStorageAt test:"
	response_test $STORAGE_AT_0xD7F
	echo
}

function API_getTransactionCount_test() {
	echo "POST hmy_getTransactionCount test:"
	response_test $TRANSACTIONCOUNT
	echo
}

function API_sendRawTransaction_test() {
	echo "POST hmy_sendRawTransaction test:"
	response_test $SENDRAWTX_OUTPUT
	echo
}

function API_getLogs_test() {
	echo "POST hmy_getLogs test:"
	response_test $LOGSBYBLOCK_0x572
	echo
}

function API_getFilterChanges_test() {
	echo "POST hmy_getFilterChanges test:"
	response_test $FILTERCHANGES
	echo
}

function API_newPendingTransactionFilter_test() {
	echo "POST hmy_sendRawTransaction test:"
	response_test $SENDRAWTX_OUTPUT
	echo
}

function API_sendRawTransaction_test() {
	echo "POST hmy_newPendingTransactionFilter test:"
	response_test $NEWPENDTINGTXFILTER_OUTPUT
	echo
}

function API_newBlockFilter_test() {
	echo "POST hmy_newBlockFilter test:"
	response_test $NEWBLOCKFILTER_OUTPUT
	echo
}

function API_newFilter_test() {
	echo "POST hmy_newFilter test:"
	response_test $NEWFILTER_OUTPUT
	echo
}

function API_call_test() {
	echo "POST hmy_call test:"
	response_test $CALL_OUTPUT
	echo
}

function API_gasPrice_test() {
	echo "POST hmy_gasPrice test:"
	response_test $GASPRICE
	echo
}

function API_blockNumber_test() {
	echo "POST hmy_blockNumber test:"
	response_test $BLOCKNUMBER
	echo
}

function API_net_version_test() {
	echo "POST net_version test:"
	response_test $NETVERSION
	echo
}

function API_protocolVersion_test() {
	echo "POST hmy_protocolVersion test:"
	response_test $PROTOCOL_VRESION
	echo
}


#if [ $(jq '.foo' $BLOCK_0_BY_HASH) == $(jq '.foo' $BLOCK0_POST) ]; then


function run_tests() {
	echo "### TESTING RPC CALLS ###"
	echo
	### Calls to the individual API method test ###
	Explorer_getBlock_test
	Explorer_getTx_test
	Explorer_getExplorerAddress_test
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

function log_API_responses {
	### Log info from API Explorer GET requests ###

	echo "BLOCK 0_GET"
	echo $BLOCK0_GET
	echo
	echo "Transaction 0:"
	echo $TX0
	echo
	echo "Address 0:"
	echo $ADDR0
	echo
	echo "node count:"
	echo $NODE_COUNT
	echo
	echo "Shard 0:"
	echo $SHARD0
	echo
	echo "Committe 0:"	
	echo $COMMITTE0
	echo

	### Log info from API TEST POST requests ###
	echo "BLOCK_0x1_BY_HASH"
	echo $BLOCK_0x1_BY_HASH
	echo
	echo "BLOCK0_POST"
	echo $BLOCK0_POST
	echo
	echo "BLOCK_0x1_TXCOUNT"
	echo $BLOCK_0x1_TXCOUNT
	echo
	echo "BLOCKNUM_0x1_TXCOUNT"
	echo $BLOCKNUM_0x1_TXCOUNT
	echo
	echo "GETCODE_OUTPUT"
	echo $GETCODE_OUTPUT
	echo
	echo "BLOCK_X_TX0"
	echo $BLOCK_X_TX0
	echo
	echo "BLOCK_0x4_TX0"
	echo $BLOCK_0x4_TX0
	echo
	echo "RAND_TX"
	echo $RAND_TX
	echo
	echo "RAND_TX_RECEIPT"
	echo $RAND_TX_RECEIPT
	echo
	echo "SYNCING_OUTPUT"
	echo $SYNCING_OUTPUT
	echo
	echo "NET_PEER_COUNT"
	echo $NET_PEER_COUNT
	echo
	echo "BALANCE_0xD7F"
	echo $BALANCE_0xD7F
	echo
	echo "STORAGE_AT_0xD7F"
	echo $STORAGE_AT_0xD7F
	echo
	echo "TRANSACTIONCOUNT"
	echo $TRANSACTIONCOUNT
	echo
	echo "SENDRAWTX_OUTPUT"
	echo $SENDRAWTX_OUTPUT
	echo
	echo "LOGSBYBLOCK_0x572"
	echo $LOGSBYBLOCK_0x572
	echo
	echo "FILTERCHANGES"
	echo $FILTERCHANGES
	echo
	echo "NEWPENDTINGTXFILTER_OUTPUT"
	echo $NEWPENDTINGTXFILTER_OUTPUT
	echo
	echo "NEWBLOCKFILTER_OUTPUT"
	echo $NEWBLOCKFILTER_OUTPUT
	echo
	echo "NEWFILTER_OUTPUT"
	echo $NEWFILTER_OUTPUT
	echo
	echo "CALL_OUTPUT"
	echo $CALL_OUTPUT
	echo
	echo "GASPRICE"
	echo $GASPRICE
	echo
	echo "BLOCKNUMBER"
	echo $BLOCKNUMBER
	echo
	echo "NETVERSION"
	echo $NETVERSION
	echo
	echo "PROTOCOL_VRESION"
	echo $PROTOCOL_VRESION
	echo
}

red=`tput setaf 1`
green=`tput setaf 2`
reset=`tput sgr0`
echo $NETWORK
if [[ "$NETWORK" == "localnet" ]]; then
	echo "LOCALNET TESTING "
	# Ping the LocalNet API and store responses

	### EXPLORER GET REQUESTS LOCALHOST ###

	BLOCK0_GET=$(curl --location --request GET "localhost:5099/blocks?from=1&to=1" \
		  --header "Content-Type: application/json" \
		  --data "")

	TX0=$(curl -s --location --request GET "localhost:5099/tx?id=0" \
		  --header "Content-Type: application/json" \
		  --data "")

	ADDR0=$(curl -s --location --request GET "localhost:5099/address?id=one1pdv9lrdwl0rg5vglh4xtyrv3wjk3wsqket7zxy" \
		  --header "Content-Type: application/json" \
		  --data "")

	NODE_COUNT=$(curl -s --location --request GET "localhost:5099/node-count" \
		  --header "Content-Type: application/json" \
		    --data "")

	SHARD0=$(curl -s --location --request GET "localhost:5099/shard?id=0" \
		  --header "Content-Type: application/json" \
		    --data "")

	SHARD1=$(curl -s --location --request GET "localhost:5099/shard?id=1" \
		  --header "Content-Type: application/json" \
		    --data "")

	COMMITTE0=$(curl --location --request GET "localhost:5099/committee?shard_id=0&epoch=0" \
		  --header "Content-Type: application/json" \
		    --data "")

	### API POST REQUESTS LOCALHOST###
	BLOCK_0x1_BY_HASH=$(curl -s --location --request POST "localhost:9500" \
		  --header "Content-Type: application/json" \
		  --data "{\"jsonrpc\":\"2.0\",\"method\":\"hmy_getBlockByHash\",\"params\":[\"0x38bf30b40b7c4452c0459fea394e7fc217ace0c949939df3ecf260b785d584c2\", true],\"id\":1}")

	BLOCK0_POST=$(curl -s --location --request POST "localhost:9500" \
		  --header "Content-Type: application/json" \
		  --data "{\"jsonrpc\":\"2.0\",\"method\":\"hmy_getBlockByNumber\",\"params\":[\"0x1\", true],\"id\":1}")

	BLOCK_0x1_TXCOUNT=$(curl -s --location --request POST "localhost:9500" \
		  --header "Content-Type: application/json" \
		  --data "{\"jsonrpc\":\"2.0\",\"method\":\"hmy_getBlockTransactionCountByHash\",\"params\":[\"0x660fe701f580ffebfcfb4af1836c9929c1fd0014d8d79d60749fecf52df7a90d\"],\"id\":1}")

	BLOCKNUM_0x1_TXCOUNT=$(curl -s --location --request POST "localhost:9500" \
		  --header "Content-Type: application/json" \
		  --data "{\"jsonrpc\":\"2.0\",\"method\":\"hmy_getBlockTransactionCountByNumber\",\"params\":[\"0x66\"],\"id\":1}")

	GETCODE_OUTPUT=$(curl -s --location --request POST "localhost:9500" \
		  --header "Content-Type: application/json" \
		  --data "{\"jsonrpc\":\"2.0\",\"method\":\"hmy_getCode\",\"params\":[\"0x08AE1abFE01aEA60a47663bCe0794eCCD5763c19\", \"latest\"],\"id\":1}")

	BLOCK_X_TX0=$(curl -s --location --request POST "localhost:9500" \
		  --header "Content-Type: application/json" \
		  --data "{\"jsonrpc\":\"2.0\",\"method\":\"hmy_getTransactionByBlockHashAndIndex\",\"params\":[\"0x428ead93e632d5255ea3d1fb61b02ab8493cf562a398af2159c33ecd53c62c16\", \"0x0\"],\"id\":1}")

	BLOCK_0x4_TX0=$(curl -s --location --request POST "localhost:9500" \
		  --header "Content-Type: application/json" \
		  --data "{\"jsonrpc\":\"2.0\",\"method\":\"hmy_getTransactionByBlockNumberAndIndex\",\"params\":[\"0x4\", \"0x0\"],\"id\":1}")

	RAND_TX=$(curl -s --location --request POST "localhost:9500" \
		  --header "Content-Type: application/json" \
		  --data "{\"jsonrpc\":\"2.0\",\"method\":\"hmy_getTransactionByHash\",\"params\":[\"0xa7bb2c7cbfe4f5d6cf061aacd9d0dce7660d1f82da63dd7c76d9e856c1dc0278\"],\"id\":1}")

	RAND_TX_RECEIPT=$(curl -s --location --request POST "localhost:9500" \
		  --header "Content-Type: application/json" \
		  --data "{\"jsonrpc\":\"2.0\",\"method\":\"hmy_getTransactionReceipt\",\"params\":[\"0x17452d6c153f3e42dae114f63fd0a9dab9ce9cc2a4bb4400823762f60787c3bf\"],\"id\":1}")

	SYNCING_OUTPUT=$(curl -s --location --request POST "localhost:9500" \
		  --header "Content-Type: application/json" \
		  --data "{\"jsonrpc\":\"2.0\",\"method\":\"hmy_syncing\",\"params\":[],\"id\":1}")

	NET_PEER_COUNT=$(curl -s --location --request POST "localhost:9500" \
		  --header "Content-Type: application/json" \
		  --data "{\"jsonrpc\":\"2.0\",\"method\":\"net_peerCount\",\"params\":[],\"id\":1}")

	BALANCE_0xD7F=$(curl -s --location --request POST "localhost:9500" \
		  --header "Content-Type: application/json" \
		  --data "{\"jsonrpc\":\"2.0\",\"method\":\"hmy_getBalance\",\"params\":[\"0xD7Ff41CA29306122185A07d04293DdB35F24Cf2d\", \"latest\"],\"id\":1}")

	STORAGE_AT_0xD7F=$(curl -s --location --request POST "localhost:9500" \
		  --header "Content-Type: application/json" \
		  --data "{\"jsonrpc\":\"2.0\",\"method\":\"hmy_getStorageAt\",\"params\":[\"0xD7Ff41CA29306122185A07d04293DdB35F24Cf2d\", \"0\", \"latest\"],\"id\":1}")

	TRANSACTIONCOUNT=$(curl -s --location --request POST "localhost:9500" \
		  --header "Content-Type: application/json" \
		  --data "{\"jsonrpc\":\"2.0\",\"method\":\"hmy_getTransactionCount\",\"params\":[\"0x806171f95C5a74371a19e8a312c9e5Cb4E1D24f6\", \"latest\"],\"id\":1}")

	SENDRAWTX_OUTPUT=$(curl -s --location --request POST "localhost:9500" \
		  --header "Content-Type: application/json" \
		  --data "{\"jsonrpc\":\"2.0\",\"method\":\"hmy_sendRawTransaction\",\"params\":[\"0xf869808082520880809410a02a0a6e95a676ae23e2db04bea3d1b8b7ca2e880de0b6b3a7640000801ba0c8d0c5390086999b5b5a93373953c3c94b44dc8fd06d88a421a7c2461e9e4482a0730d7859d1e3109d499bcd75f00700729b9bc17b03940da4f84b6ea784f51eb1\"],\"id\":1}")

	LOGSBYBLOCK_0x572=$(curl -s --location --request POST "localhost:9500" \
		  --header "Content-Type: application/json" \
		  --data "{\"jsonrpc\":\"2.0\", \"method\":\"hmy_getLogs\", \"params\":[{\"BlockHash\": \"0x5725b5b2ab28206e7256a78cda4f9050c2629fd85110ffa54eacd2a13ba68072\"}],\"id\":1}")

	FILTERCHANGES=$(curl -s --location --request POST "localhost:9500" \
		  --header "Content-Type: application/json" \
		  --data "{\"jsonrpc\":\"2.0\", \"method\":\"hmy_getFilterChanges\", \"params\":[\"0x58010795a282878ed0d61da72a14b8b0\"],\"id\":1}")

	NEWPENDTINGTXFILTER_OUTPUT=$(curl -s --location --request POST "localhost:9500" \
		  --header "Content-Type: application/json" \
		  --data "{\"jsonrpc\":\"2.0\", \"method\":\"hmy_newPendingTransactionFilter\", \"params\":[],\"id\":1}")

	NEWBLOCKFILTER_OUTPUT=$(curl -s --location --request POST "localhost:9500" \
		  --header "Content-Type: application/json" \
		  --data "{\"jsonrpc\":\"2.0\", \"method\":\"hmy_newBlockFilter\", \"params\":[],\"id\":1}")

	NEWFILTER_OUTPUT=$(curl -s --location --request POST "localhost:9500" \
		  --header "Content-Type: application/json" \
		  --data "{\"jsonrpc\":\"2.0\", \"method\":\"hmy_newFilter\", \"params\":[{\"BlockHash\": \"0x5725b5b2ab28206e7256a78cda4f9050c2629fd85110ffa54eacd2a13ba68072\"}],\"id\":1}")

	CALL_OUTPUT=$(curl -s --location --request POST "localhost:9500" \
		  --header "Content-Type: application/json" \
		  --data "{\"jsonrpc\":\"2.0\", \"method\":\"hmy_call\", \"params\":[{\"to\": \"0x08AE1abFE01aEA60a47663bCe0794eCCD5763c19\"}, \"latest\"],\"id\":1}")

	GASPRICE=$(curl -s --location --request POST "localhost:9500" \
		  --header "Content-Type: application/json" \
		  --data "{\"jsonrpc\":\"2.0\",\"method\":\"hmy_gasPrice\",\"params\":[],\"id\":1}")

	BLOCKNUMBER=$(curl -s --location --request POST "localhost:9500" \
		  --header "Content-Type: application/json" \
		  --data "{\"jsonrpc\":\"2.0\",\"method\":\"hmy_blockNumber\",\"params\":[],\"id\":1}")

	NETVERSION=$(curl -s --location --request POST "localhost:9500" \
		  --header "Content-Type: application/json" \
		  --data "{\"jsonrpc\":\"2.0\",\"method\":\"net_version\",\"params\":[],\"id\":1}")

	PROTOCOL_VRESION=$(curl -s --location --request POST "localhost:9500" \
		  --header "Content-Type: application/json" \
		  --data "{\"jsonrpc\":\"2.0\",\"method\":\"hmy_protocolVersion\",\"params\":[],\"id\":1}")


	if [ "$VERBOSE" == "TRUE" ]; then
		log_API_responses
	fi

	### LOCALNET TESTS ###
	run_tests
fi 

if [[ "$NETWORK" == "betanet" ]]; then
	echo "BETANET TESTING "
	# Ping the LocalNet API and store responses

	### EXPLORER GET REQUESTS LOCALHOST ###

	BLOCK0_GET=$(curl --location --request GET "107.21.71.80:5000/blocks?from=0&to=0" \
		  --header "Content-Type: application/json" \
		  --data "")

	TX0=$(curl -s --location --request GET "107.21.71.80:5000/tx?id=0" \
		  --header "Content-Type: application/json" \
		  --data "")

	ADDR0=$(curl -s --location --request GET "107.21.71.80:5000/address?id=0" \
		  --header "Content-Type: application/json" \
		  --data "")

	NODE_COUNT=$(curl -s --location --request GET "107.21.71.80:5000/node-count" \
		  --header "Content-Type: application/json" \
		    --data "")

	SHARD0=$(curl -s --location --request GET "107.21.71.80:5000/shard?id=0" \
		  --header "Content-Type: application/json" \
		    --data "")

	COMMITTE0=$(curl -s --location --request GET "107.21.71.80:5000/committee?shard_id=0&epoch=0" \
		  --header "Content-Type: application/json" \
		    --data "")

	### API POST REQUESTS BETANET###
	BLOCK_0x1_BY_HASH=$(curl -s --location --request POST "http://l0.b.hmny.io:9500" \
		  --header "Content-Type: application/json" \
		  --data "{\"jsonrpc\":\"2.0\",\"method\":\"hmy_getBlockByHash\",\"params\":[\"0x38b2d548bc24ac21a17db22e92c198ec0dc7a3646bf5245971617041ca1ebf1e\", true],\"id\":1}")

	BLOCK0_POST=$(curl -s --location --request POST "http://l0.b.hmny.io:9500" \
		  --header "Content-Type: application/json" \
		  --data "{\"jsonrpc\":\"2.0\",\"method\":\"hmy_getBlockByNumber\",\"params\":[\"0x1\", true],\"id\":1}")

	BLOCK_0x1_TXCOUNT=$(curl -s --location --request POST "http://l0.b.hmny.io:9500" \
		  --header "Content-Type: application/json" \
		  --data "{\"jsonrpc\":\"2.0\",\"method\":\"hmy_getBlockTransactionCountByHash\",\"params\":[\"0x660fe701f580ffebfcfb4af1836c9929c1fd0014d8d79d60749fecf52df7a90d\"],\"id\":1}")

	BLOCKNUM_0x1_TXCOUNT=$(curl -s --location --request POST "http://l0.b.hmny.io:9500" \
		  --header "Content-Type: application/json" \
		  --data "{\"jsonrpc\":\"2.0\",\"method\":\"hmy_getBlockTransactionCountByNumber\",\"params\":[\"0x66\"],\"id\":1}")

	GETCODE_OUTPUT=$(curl -s --location --request POST "http://l0.b.hmny.io:9500" \
		  --header "Content-Type: application/json" \
		  --data "{\"jsonrpc\":\"2.0\",\"method\":\"hmy_getCode\",\"params\":[\"0x08AE1abFE01aEA60a47663bCe0794eCCD5763c19\", \"latest\"],\"id\":1}")

	BLOCK_X_TX0=$(curl -s --location --request POST "http://l0.b.hmny.io:9500" \
		  --header "Content-Type: application/json" \
		  --data "{\"jsonrpc\":\"2.0\",\"method\":\"hmy_getTransactionByBlockHashAndIndex\",\"params\":[\"0x428ead93e632d5255ea3d1fb61b02ab8493cf562a398af2159c33ecd53c62c16\", \"0x0\"],\"id\":1}")

	BLOCK_0x4_TX0=$(curl -s --location --request POST "http://l0.b.hmny.io:9500" \
		  --header "Content-Type: application/json" \
		  --data "{\"jsonrpc\":\"2.0\",\"method\":\"hmy_getTransactionByBlockNumberAndIndex\",\"params\":[\"0x4\", \"0x0\"],\"id\":1}")

	RAND_TX=$(curl -s --location --request POST "http://l0.b.hmny.io:9500" \
		  --header "Content-Type: application/json" \
		  --data "{\"jsonrpc\":\"2.0\",\"method\":\"hmy_getTransactionByHash\",\"params\":[\"0xa7bb2c7cbfe4f5d6cf061aacd9d0dce7660d1f82da63dd7c76d9e856c1dc0278\"],\"id\":1}")

	RAND_TX_RECEIPT=$(curl -s --location --request POST "http://l0.b.hmny.io:9500" \
		  --header "Content-Type: application/json" \
		  --data "{\"jsonrpc\":\"2.0\",\"method\":\"hmy_getTransactionReceipt\",\"params\":[\"0x17452d6c153f3e42dae114f63fd0a9dab9ce9cc2a4bb4400823762f60787c3bf\"],\"id\":1}")

	SYNCING_OUTPUT=$(curl -s --location --request POST "http://l0.b.hmny.io:9500" \
		  --header "Content-Type: application/json" \
		  --data "{\"jsonrpc\":\"2.0\",\"method\":\"hmy_syncing\",\"params\":[],\"id\":1}")

	NET_PEER_COUNT=$(curl -s --location --request POST "http://l0.b.hmny.io:9500" \
		  --header "Content-Type: application/json" \
		  --data "{\"jsonrpc\":\"2.0\",\"method\":\"net_peerCount\",\"params\":[],\"id\":1}")

	BALANCE_0xD7F=$(curl -s --location --request POST "http://l0.b.hmny.io:9500" \
		  --header "Content-Type: application/json" \
		  --data "{\"jsonrpc\":\"2.0\",\"method\":\"hmy_getBalance\",\"params\":[\"0xD7Ff41CA29306122185A07d04293DdB35F24Cf2d\", \"latest\"],\"id\":1}")

	STORAGE_AT_0xD7F=$(curl -s --location --request POST "http://l0.b.hmny.io:9500" \
		  --header "Content-Type: application/json" \
		  --data "{\"jsonrpc\":\"2.0\",\"method\":\"hmy_getStorageAt\",\"params\":[\"0xD7Ff41CA29306122185A07d04293DdB35F24Cf2d\", \"0\", \"latest\"],\"id\":1}")

	TRANSACTIONCOUNT=$(curl -s --location --request POST "http://l0.b.hmny.io:9500" \
		  --header "Content-Type: application/json" \
		  --data "{\"jsonrpc\":\"2.0\",\"method\":\"hmy_getTransactionCount\",\"params\":[\"0x806171f95C5a74371a19e8a312c9e5Cb4E1D24f6\", \"latest\"],\"id\":1}")

	SENDRAWTX_OUTPUT=$(curl -s --location --request POST "http://l0.b.hmny.io:9500" \
		  --header "Content-Type: application/json" \
		  --data "{\"jsonrpc\":\"2.0\",\"method\":\"hmy_sendRawTransaction\",\"params\":[\"0xf869808082520880809410a02a0a6e95a676ae23e2db04bea3d1b8b7ca2e880de0b6b3a7640000801ba0c8d0c5390086999b5b5a93373953c3c94b44dc8fd06d88a421a7c2461e9e4482a0730d7859d1e3109d499bcd75f00700729b9bc17b03940da4f84b6ea784f51eb1\"],\"id\":1}")

	LOGSBYBLOCK_0x572=$(curl -s --location --request POST "http://l0.b.hmny.io:9500" \
		  --header "Content-Type: application/json" \
		  --data "{\"jsonrpc\":\"2.0\", \"method\":\"hmy_getLogs\", \"params\":[{\"BlockHash\": \"0x5725b5b2ab28206e7256a78cda4f9050c2629fd85110ffa54eacd2a13ba68072\"}],\"id\":1}")

	FILTERCHANGES=$(curl -s --location --request POST "http://l0.b.hmny.io:9500" \
		  --header "Content-Type: application/json" \
		  --data "{\"jsonrpc\":\"2.0\", \"method\":\"hmy_getFilterChanges\", \"params\":[\"0x58010795a282878ed0d61da72a14b8b0\"],\"id\":1}")

	NEWPENDTINGTXFILTER_OUTPUT=$(curl -s --location --request POST "http://l0.b.hmny.io:9500" \
		  --header "Content-Type: application/json" \
		  --data "{\"jsonrpc\":\"2.0\", \"method\":\"hmy_newPendingTransactionFilter\", \"params\":[],\"id\":1}")

	NEWBLOCKFILTER_OUTPUT=$(curl -s --location --request POST "http://l0.b.hmny.io:9500" \
		  --header "Content-Type: application/json" \
		  --data "{\"jsonrpc\":\"2.0\", \"method\":\"hmy_newBlockFilter\", \"params\":[],\"id\":1}")

	NEWFILTER_OUTPUT=$(curl -s --location --request POST "http://l0.b.hmny.io:9500" \
		  --header "Content-Type: application/json" \
		  --data "{\"jsonrpc\":\"2.0\", \"method\":\"hmy_newFilter\", \"params\":[{\"BlockHash\": \"0x5725b5b2ab28206e7256a78cda4f9050c2629fd85110ffa54eacd2a13ba68072\"}],\"id\":1}")

	CALL_OUTPUT=$(curl -s --location --request POST "http://l0.b.hmny.io:9500" \
		  --header "Content-Type: application/json" \
		  --data "{\"jsonrpc\":\"2.0\", \"method\":\"hmy_call\", \"params\":[{\"to\": \"0x08AE1abFE01aEA60a47663bCe0794eCCD5763c19\"}, \"latest\"],\"id\":1}")

	GASPRICE=$(curl -s --location --request POST "http://l0.b.hmny.io:9500" \
		  --header "Content-Type: application/json" \
		  --data "{\"jsonrpc\":\"2.0\",\"method\":\"hmy_gasPrice\",\"params\":[],\"id\":1}")

	BLOCKNUMBER=$(curl -s --location --request POST "http://l0.b.hmny.io:9500" \
		  --header "Content-Type: application/json" \
		  --data "{\"jsonrpc\":\"2.0\",\"method\":\"hmy_blockNumber\",\"params\":[],\"id\":1}")

	NETVERSION=$(curl -s --location --request POST "http://l0.b.hmny.io:9500" \
		  --header "Content-Type: application/json" \
		  --data "{\"jsonrpc\":\"2.0\",\"method\":\"net_version\",\"params\":[],\"id\":1}")

	PROTOCOL_VRESION=$(curl -s --location --request POST "http://l0.b.hmny.io:9500" \
		  --header "Content-Type: application/json" \
		  --data "{\"jsonrpc\":\"2.0\",\"method\":\"hmy_protocolVersion\",\"params\":[],\"id\":1}")


	if [ "$VERBOSE" == "TRUE" ]; then
		log_API_responses
	fi
	### BETANET TESTS ###

	run_tests
fi


