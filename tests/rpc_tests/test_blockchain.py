#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
Tests here do not require transaction or a feedback loop with the chain
(other than the initial transactions) to validate functionality.

TODO: negative test cases

As with all tests, there are 2 JSON-RPC versions/namespaces (v1 & v2) where their difference
is only suppose to be in the types of their params & returns. v1 keeps everything in hex and
v2 uses decimal when possible. However, there are some (legacy) discrepancies that some tests
enforce. These tests are noted and should NOT be broken.
"""

from pyhmy import (
    account,
    transaction
)
from pyhmy.rpc.request import (
    base_request
)

import txs
from flaky import flaky
from txs import (
    endpoints,
    initial_funding
)
from utils import (
    assert_valid_json_structure,
    check_and_unpack_rpc_response,
    mutually_exclusive_test,
    rerun_delay_filter
)

_mutex_scope = "blockchain"


def test_invalid_method():
    reference_response = {
        "code": -32601,
        "message": "The method test_ does not exist/is not available"
    }

    raw_response = base_request(f"invalid_{hash('method')}", endpoint=endpoints[0])
    response = check_and_unpack_rpc_response(raw_response, expect_error=True)
    assert_valid_json_structure(reference_response, response)
    assert response["code"] == reference_response["code"]


def test_net_peer_count_v1():
    """
    Note that this is NOT a `hmy` RPC, however, there are 2 versions of it.
    """
    raw_response = base_request("net_peerCount", endpoint=endpoints[0])
    response = check_and_unpack_rpc_response(raw_response, expect_error=False)
    assert isinstance(response, str) and response.startswith("0x")  # Must be a hex string


def test_net_peer_count_v2():
    """
    Note that this is NOT a `hmy` RPC, however, there are 2 versions of it.
    """
    raw_response = base_request("netv2_peerCount", endpoint=endpoints[0])
    response = check_and_unpack_rpc_response(raw_response, expect_error=False)
    assert isinstance(response, int)  # Must be an integer in base 10


def test_get_node_metadata():
    """
    Note that v1 & v2 have the same responses.
    """
    reference_response = {
        "blskey": [
            "65f55eb3052f9e9f632b2923be594ba77c55543f5c58ee1454b9cfd658d25e06373b0f7d42a19c84768139ea294f6204"
        ],
        "version": "Harmony (C) 2020. harmony, version v6110-v2.1.9-34-g24ec31c1 (danielvdm@ 2020-07-11T05:03:50-0700)",
        "network": "localnet",
        "chain-config": {
            "chain-id": 2,
            "cross-tx-epoch": 0,
            "cross-link-epoch": 2,
            "staking-epoch": 2,
            "prestaking-epoch": 0,
            "quick-unlock-epoch": 0,
            "eip155-epoch": 0,
            "s3-epoch": 0,
            "receipt-log-epoch": 0
        },
        "is-leader": True,
        "shard-id": 0,
        "current-epoch": 0,
        "blocks-per-epoch": 5,
        "role": "Validator",
        "dns-zone": "",
        "is-archival": False,
        "node-unix-start-time": 1594469045,
        "p2p-connectivity": {
            "total-known-peers": 24,
            "connected": 23,
            "not-connected": 1
        }
    }

    # Check v1
    raw_response = base_request("hmy_getNodeMetadata", endpoint=endpoints[0])
    response = check_and_unpack_rpc_response(raw_response, expect_error=False)
    assert_valid_json_structure(reference_response, response)
    assert response["shard-id"] == 0
    assert response["network"] == "localnet"
    assert response["chain-config"]["chain-id"] == 2

    # Check v2
    raw_response = base_request("hmyv2_getNodeMetadata", endpoint=endpoints[0])
    response = check_and_unpack_rpc_response(raw_response, expect_error=False)
    assert_valid_json_structure(reference_response, response)
    assert response["shard-id"] == 0
    assert response["network"] == "localnet"
    assert response["chain-config"]["chain-id"] == 2


def test_get_sharding_structure():
    """
    Note that v1 & v2 have the same responses.
    """
    reference_response = {
        "current": True,
        "http": "http://127.0.0.1:9500",
        "shardID": 0,
        "ws": "ws://127.0.0.1:9800"
    }

    # Check v1
    raw_response = base_request("hmy_getShardingStructure", endpoint=endpoints[0])
    response = check_and_unpack_rpc_response(raw_response, expect_error=False)
    assert isinstance(response, list), f"Expected response to be of type list, not {type(response)}"
    for d in response:
        assert isinstance(d, dict), f"Expected type dict in response elements, not {type(d)}"
        assert_valid_json_structure(reference_response, d)
    assert response[0]["current"]

    # Check v2
    raw_response = base_request("hmyv2_getShardingStructure", endpoint=endpoints[0])
    response = check_and_unpack_rpc_response(raw_response, expect_error=False)
    assert isinstance(response, list), f"Expected response to be of type list, not {type(response)}"
    for d in response:
        assert isinstance(d, dict), f"Expected type dict in response elements, not {type(d)}"
        assert_valid_json_structure(reference_response, d)
    assert response[0]["current"]


@txs.staking
def test_get_latest_header():
    """
    Note that v1 & v2 have the same responses.
    """
    reference_response = {
        "blockHash": "0x4e9faaf05bd7ed0ed392b3b5b19f2d2df993e60436c94b61b8afae6998b854b5",
        "blockNumber": 83,
        "shardID": 0,
        "leader": "one1pdv9lrdwl0rg5vglh4xtyrv3wjk3wsqket7zxy",
        "viewID": 83,
        "epoch": 15,
        "timestamp": "2020-07-12 14:25:05 +0000 UTC",
        "unixtime": 1594563905,
        "lastCommitSig": "76e8365fdbd947f74d86f15072546f594f8aaf3f6bf0b085df1d81079b760e17da1666d8d07f5c744e200f81a5fa750901d0dc871a4dbe5461efa779553db3f95785d168701c774b23d2326f0d906e47d534a34c87f4ace5e4ed2242860bfc0e",
        "lastCommitBitmap": "3f",
        "crossLinks": [
            {
                "hash": "0x9dda6ad7fdec1e0f76b87bbea432e12c8668dbb2de9100e87442adfb1c7d1f70",
                "block-number": 80,
                "view-id": 80,
                "signature": "73f8e045c0cee4accfd259a78ea440ef2cf8c95d1c2e0e069725b35034c63e27f4db8ec5e1983d1c21831561967d0101ba556977e047a032e2c0f70bc3dc658ae245cb3392c837aed46119fa95ab39aad4ac926a206ab174304be15bb17df68a",
                "signature-bitmap": "3f",
                "shard-id": 1,
                "epoch-number": 14
            }
        ]
    }

    # Check v1
    raw_response = base_request("hmy_latestHeader", endpoint=endpoints[0])
    response = check_and_unpack_rpc_response(raw_response, expect_error=False)
    assert_valid_json_structure(reference_response, response)
    assert response["shardID"] == 0

    # Check v2
    raw_response = base_request("hmyv2_latestHeader", endpoint=endpoints[0])
    response = check_and_unpack_rpc_response(raw_response, expect_error=False)
    assert_valid_json_structure(reference_response, response)
    assert response["shardID"] == 0


def test_get_latest_chain_headers():
    """
    Note that v1 & v2 have the same responses.
    """
    reference_response = {
        "beacon-chain-header": {
            "epoch": 18,
            "extraData": "0x",
            "gasLimit": "0x4c4b400",
            "gasUsed": "0x0",
            "hash": "0xdb6099006bb637578a94fa3003b2b68947ae8fcdc7e9f89ea0f83edd5ad65c9e",
            "logsBloom": "0x00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
            "difficulty": "0",
            "miner": "0x6911b75b2560be9a8f71164a33086be4511fc99a",
            "mixHash": "0x0000000000000000000000000000000000000000000000000000000000000000",
            "number": "0xab",
            "sha3Uncles":
            '0x0000000000000000000000000000000000000000000000000000000000000000',
            "nonce": '0x0000000000000000',
            "parentHash": "0x9afbaa3db3c2f5393b7b765aa48091c7a6cc3c3fdf19fb88f8ee2c88e257988d",
            "receiptsRoot": "0x56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421",
            "shardID": 0,
            "stateRoot": "0x4d0b5b168bd3214e32f95718ef591e04c54f060f89e4304c8d4ff2745ebad3a9",
            "timestamp": "0x60751990",
            "transactionsRoot": "0x56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421",
            "viewID": 171
        },
        "shard-chain-header": {
            "epoch": 17,
            "extraData": "0x",
            "gasLimit": "0x4c4b400",
            "gasUsed": "0x0",
            "hash": "0xdf4c2be8e676f728a419415bdd393a5c174e0b1431e016f870d0777d86668708",
            "logsBloom": "0x00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
            "difficulty": "0",
            "miner": "0xd06193871db8d5bc92dead4780e3624038174e88",
            "mixHash": "0x0000000000000000000000000000000000000000000000000000000000000000",
            "number": "0xa7",
            "sha3Uncles":
            '0x0000000000000000000000000000000000000000000000000000000000000000',
            "nonce": '0x0000000000000000',
            "parentHash": "0x336990a12d8697c5f9247790261aa1bdea33aed37031d09664680b86779f08a6",
            "receiptsRoot": "0x56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421",
            "shardID": 1,
            "stateRoot": "0xddd8a842df3ffd77555d64ab782abf6effe95bc2eab48610ac05a5445b55df92",
            "timestamp": "0x60751991",
            "transactionsRoot": "0x56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421",
            "viewID": 167
        }
    }

    # Check v1
    raw_response = base_request(
        "hmy_getLatestChainHeaders", endpoint=endpoints[1])
    response = check_and_unpack_rpc_response(raw_response, expect_error=False)
    assert_valid_json_structure(reference_response, response)
    assert response["beacon-chain-header"]["shardID"] == 0
    assert response["shard-chain-header"]["shardID"] == 1

    # Check v2
    raw_response = base_request("hmyv2_getLatestChainHeaders", endpoint=endpoints[1])
    response = check_and_unpack_rpc_response(raw_response, expect_error=False)
    assert_valid_json_structure(reference_response, response)
    assert response["beacon-chain-header"]["shardID"] == 0
    assert response["shard-chain-header"]["shardID"] == 1


def test_get_leader_address():
    """
    Note that v1 & v2 have the same responses.
    """
    reference_response = "0x6911b75b2560be9a8f71164a33086be4511fc99a"

    # Check v1
    raw_response = base_request("hmy_getLeader", endpoint=endpoints[0])
    response = check_and_unpack_rpc_response(raw_response, expect_error=False)
    assert type(reference_response) == type(response)
    if response.startswith("one1"):
        assert account.is_valid_address(response), f"Leader address is not a valid ONE address"
    else:
        ref_len = len(reference_response.replace("0x", ""))
        assert ref_len == len(response.replace("0x", "")), f"Leader address hash is not of length {ref_len}"

    # Check v2
    raw_response = base_request("hmyv2_getLeader", endpoint=endpoints[0])
    response = check_and_unpack_rpc_response(raw_response, expect_error=False)
    if response.startswith("one1"):
        assert account.is_valid_address(response), f"Leader address is not a valid ONE address"
    else:
        ref_len = len(reference_response.replace("0x", ""))
        assert ref_len == len(response.replace("0x", "")), f"Leader address hash is not of length {ref_len}"


@flaky(max_runs=6, rerun_filter=rerun_delay_filter(delay=8))
@txs.cross_shard
def test_get_block_signer_keys():
    """
    Note that v1 & v2 have the same responses.
    """
    reference_response = [
        "65f55eb3052f9e9f632b2923be594ba77c55543f5c58ee1454b9cfd658d25e06373b0f7d42a19c84768139ea294f6204",
    ]

    # Check v1
    raw_response = base_request("hmy_getBlockSignerKeys", params=["0x2"], endpoint=endpoints[0])
    response = check_and_unpack_rpc_response(raw_response, expect_error=False)
    assert_valid_json_structure(reference_response, response)

    # Check v2
    raw_response = base_request("hmyv2_getBlockSignerKeys", params=[2], endpoint=endpoints[0])
    response = check_and_unpack_rpc_response(raw_response, expect_error=False)
    assert_valid_json_structure(reference_response, response)


def test_get_header_by_number():
    """
    Note that v1 & v2 have the same responses.
    """
    reference_response = {
        "blockHash": "0xb718a66ef2b7764fa75b40bfe7047d015197a65ae4a9c4f2007501825025564c",
        "blockNumber": 1,
        "shardID": 0,
        "leader": "one1pdv9lrdwl0rg5vglh4xtyrv3wjk3wsqket7zxy",
        "viewID": 1,
        "epoch": 0,
        "timestamp": "2020-07-12 14:14:17 +0000 UTC",
        "unixtime": 1594563257,
        "lastCommitSig": "000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
        "lastCommitBitmap": "",
        "crossLinks": []
    }

    # Check v1
    raw_response = base_request("hmy_getHeaderByNumber", params=["0x0"], endpoint=endpoints[0])
    response = check_and_unpack_rpc_response(raw_response, expect_error=False)
    assert_valid_json_structure(reference_response, response)
    assert response["shardID"] == 0

    # Check v2
    raw_response = base_request("hmyv2_getHeaderByNumber", params=[0], endpoint=endpoints[0])
    response = check_and_unpack_rpc_response(raw_response, expect_error=False)
    assert_valid_json_structure(reference_response, response)
    assert response["shardID"] == 0


@flaky(max_runs=6, rerun_filter=rerun_delay_filter(delay=8))
@txs.cross_shard
def test_get_block_signers():
    """
    Note that v1 & v2 have the same responses.
    """
    reference_response = [
        "one1gh043zc95e6mtutwy5a2zhvsxv7lnlklkj42ux"
    ]

    # Check v1
    raw_response = base_request("hmy_getBlockSigners", params=["0x2"], endpoint=endpoints[0])
    response = check_and_unpack_rpc_response(raw_response, expect_error=False)
    assert_valid_json_structure(reference_response, response)
    assert len(response) > 0, "expect at least 1 signer"

    # Check v2
    raw_response = base_request("hmyv2_getBlockSigners", params=[2], endpoint=endpoints[0])
    response = check_and_unpack_rpc_response(raw_response, expect_error=False)
    assert_valid_json_structure(reference_response, response)
    assert len(response) > 0, "expect at least 1 signer"


def test_get_block_number_v1():
    raw_response = base_request("hmy_blockNumber", endpoint=endpoints[0])
    response = check_and_unpack_rpc_response(raw_response, expect_error=False)
    assert isinstance(response, str) and response.startswith("0x")  # Must be a hex string


def test_get_block_number_v2():
    raw_response = base_request("hmyv2_blockNumber", endpoint=endpoints[0])
    response = check_and_unpack_rpc_response(raw_response, expect_error=False)
    assert isinstance(response, int)  # Must be an integer in base 10


def test_get_epoch_v1():
    raw_response = base_request("hmy_getEpoch", endpoint=endpoints[0])
    response = check_and_unpack_rpc_response(raw_response, expect_error=False)
    assert isinstance(response, str) and response.startswith("0x")  # Must be a hex string


def test_get_epoch_v2():
    raw_response = base_request("hmyv2_getEpoch", endpoint=endpoints[0])
    response = check_and_unpack_rpc_response(raw_response, expect_error=False)
    assert isinstance(response, int)  # Must be an integer


def test_get_gas_price_v1():
    raw_response = base_request("hmy_gasPrice", endpoint=endpoints[0])
    response = check_and_unpack_rpc_response(raw_response, expect_error=False)
    assert isinstance(response, str) and response.startswith("0x")  # Must be a hex string


def test_get_gas_price_v2():
    raw_response = base_request("hmyv2_gasPrice", endpoint=endpoints[0])
    response = check_and_unpack_rpc_response(raw_response, expect_error=False)
    assert isinstance(response, int)  # Must be an integer in base 10


def test_get_protocol_version_v1():
    raw_response = base_request("hmy_protocolVersion", endpoint=endpoints[0])
    response = check_and_unpack_rpc_response(raw_response, expect_error=False)
    assert isinstance(response, str) and response.startswith("0x")  # Must be a hex string


def test_get_protocol_version_v2():
    raw_response = base_request("hmyv2_protocolVersion", endpoint=endpoints[0])
    response = check_and_unpack_rpc_response(raw_response, expect_error=False)
    assert isinstance(response, int)  # Must be an integer in base 10


def test_get_block_transaction_count_by_number_v1():
    init_tx_record = initial_funding[0]
    init_tx = transaction.get_transaction_by_hash(init_tx_record["hash"], endpoints[init_tx_record["from-shard"]])

    raw_response = base_request("hmy_getBlockTransactionCountByNumber", params=[init_tx["blockNumber"]],
                                endpoint=endpoints[0])
    response = check_and_unpack_rpc_response(raw_response, expect_error=False)
    assert isinstance(response, str) and response.startswith("0x")  # Must be a hex string
    assert int(response, 16) > 0, "Expected transaction count > 0 due to initial transactions"


def test_get_block_transaction_count_by_number_v2():
    init_tx_record = initial_funding[0]
    init_tx = transaction.get_transaction_by_hash(init_tx_record["hash"], endpoints[init_tx_record["from-shard"]])

    raw_response = base_request("hmyv2_getBlockTransactionCountByNumber", params=[int(init_tx["blockNumber"], 16)],
                                endpoint=endpoints[0])
    response = check_and_unpack_rpc_response(raw_response, expect_error=False)
    assert isinstance(response, int)
    assert response > 0, "Expected transaction count > 0 due to initial transactions"


def test_get_block_transaction_count_by_hash_v1():
    init_tx_record = initial_funding[0]
    init_tx = transaction.get_transaction_by_hash(init_tx_record["hash"], endpoints[init_tx_record["from-shard"]])

    raw_response = base_request("hmy_getBlockTransactionCountByHash", params=[init_tx["blockHash"]],
                                endpoint=endpoints[0])
    response = check_and_unpack_rpc_response(raw_response, expect_error=False)
    assert isinstance(response, str) and response.startswith("0x")  # Must be a hex string
    assert int(response, 16) > 0, "Expected transaction count > 0 due to initial transactions"


def test_get_block_transaction_count_by_hash_v2():
    init_tx_record = initial_funding[0]
    init_tx = transaction.get_transaction_by_hash(init_tx_record["hash"], endpoints[init_tx_record["from-shard"]])

    raw_response = base_request("hmyv2_getBlockTransactionCountByHash", params=[init_tx["blockHash"]],
                                endpoint=endpoints[0])
    response = check_and_unpack_rpc_response(raw_response, expect_error=False)
    assert isinstance(response, int)
    assert response > 0, "Expected transaction count > 0 due to initial transactions"


def test_get_block_by_number_v1():
    reference_response = {
        "difficulty": 0,
        "epoch": "0x0",
        "extraData": "0x",
        "gasLimit": "0x4c4b400",
        "gasUsed": "0x0",
        "hash": "0x0994da932016ba93937ad46b9a1207ecd6d4fbd689d7f8ddf1f926cd3ebc6016",
        "logsBloom": "0x00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
        "miner": "one1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqquzw7vz",
        "mixHash": "0x0000000000000000000000000000000000000000000000000000000000000000",
        "nonce": 0,
        "number": "0x1",
        "parentHash": "0x61610810993c42bacd55a124e3b9009b9ae225a2f727750db4d2171504be59fb",
        "receiptsRoot": "0x56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421",
        "size": "0x31e",
        "stakingTransactions": [],
        "stateRoot": "0x9e470e803db498e6ba3c9108d3f951060e7121289c2354b8b185349ddef4fc0a",
        "timestamp": "0x5f09ad95",
        "transactions": [],
        "transactionsRoot": "0x56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421",
        "uncles": [],
        "viewID": "0x1"
    }

    raw_response = base_request("hmy_getBlockByNumber", params=["0x0", True], endpoint=endpoints[0])
    response = check_and_unpack_rpc_response(raw_response, expect_error=False)
    assert_valid_json_structure(reference_response, response)


def test_get_block_by_number_v2():
    """
    Note the use of a JSON object in the param. This is different from v1.
    """
    reference_response = {
        "difficulty": 0,
        "epoch": 0,
        "extraData": "0x",
        "gasLimit": 80000000,
        "gasUsed": 0,
        "hash": "0x0994da932016ba93937ad46b9a1207ecd6d4fbd689d7f8ddf1f926cd3ebc6016",
        "logsBloom": "0x00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
        "miner": "one1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqquzw7vz",
        "mixHash": "0x0000000000000000000000000000000000000000000000000000000000000000",
        "nonce": 0,
        "number": 1,
        "parentHash": "0x61610810993c42bacd55a124e3b9009b9ae225a2f727750db4d2171504be59fb",
        "receiptsRoot": "0x56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421",
        "size": 798,
        "stakingTransactions": [],
        "stateRoot": "0x9e470e803db498e6ba3c9108d3f951060e7121289c2354b8b185349ddef4fc0a",
        "timestamp": 1594469781,
        "transactions": [],
        "transactionsRoot": "0x56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421",
        "uncles": [],
        "viewID": 1
    }

    raw_response = base_request("hmyv2_getBlockByNumber", params=[0, {"InclStaking": True}], endpoint=endpoints[0])
    response = check_and_unpack_rpc_response(raw_response, expect_error=False)
    assert_valid_json_structure(reference_response, response)


@mutually_exclusive_test(scope=_mutex_scope)
def test_get_blocks_v1():
    """
    Note: param options for 'withSigners' will NOT return any sensical data
    in staking epoch (since it returns ONE addresses) and is subject to removal, thus is not tested here.
    """
    reference_response_blk = {
        "difficulty": 0,
        "epoch": "0x0",
        "extraData": "0x",
        "gasLimit": "0x4c4b400",
        "gasUsed": "0x19a28",
        "hash": "0xf14c18213b1845ee09c41c5ecd321be5b745ef42e80d8e8a6bfd116452781465",
        "logsBloom": "0x00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
        "miner": "one1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqquzw7vz",
        "mixHash": "0x0000000000000000000000000000000000000000000000000000000000000000",
        "nonce": 0,
        "number": "0x3",
        "parentHash": "0xe4fe8484b94d3498acc183a8bcf9e0bef6d30b1a81d5628fbc67f9a7328651e8",
        "receiptsRoot": "0x100f3336862d22706bbe26d67e5abf90f8f25ec5a22c4446835b6beaa6b59536",
        "size": "0x4d7",
        "stakingTransactions": [],
        "stateRoot": "0x1dc641cf516efe9ff2250f9c8db069ba4b9954160bf70cbe376ed11904b4edf5",
        "timestamp": "0x5f0c8592",
        "transactions": [
            {
                "blockHash": "0xf14c18213b1845ee09c41c5ecd321be5b745ef42e80d8e8a6bfd116452781465",
                "blockNumber": "0x3",
                "from": "one1zksj3evekayy90xt4psrz8h6j2v3hla4qwz4ur",
                "timestamp": "0x5f0c8592",
                "gas": "0x5208",
                "gasPrice": "0x3b9aca00",
                "hash": "0x5718a2fda967f051611ccfaf2230dc544c9bdd388f5759a42b2fb0847fc8d759",
                "input": "0x",
                "nonce": "0x0",
                "to": "one1v92y4v2x4q27vzydf8zq62zu9g0jl6z0lx2c8q",
                "transactionIndex": "0x0",
                "value": "0x152d02c7e14af6800000",
                "shardID": 0,
                "toShardID": 0,
                "v": "0x28",
                "r": "0x76b6130bc018cedb9f8891343fd8982e0d7f923d57ea5250b8bfec9129d4ae22",
                "s": "0xfbc01c988d72235b4c71b21ce033d4fc5f82c96710b84685de0578cff075a0a"
            },
        ],
        "transactionsRoot": "0x8954e7b3ec6ef4b04dcaf2829d9ce8ed764f636445fe77d2f4e5ef157e69dbbd",
        "uncles": [],
        "viewID": "0x3"
    }

    init_tx_record = initial_funding[0]
    init_tx = transaction.get_transaction_by_hash(init_tx_record["hash"], endpoints[init_tx_record["from-shard"]])
    start_blk, end_blk = hex(max(0, int(init_tx["blockNumber"], 16) - 2)), init_tx["blockNumber"]
    raw_response = base_request("hmy_getBlocks",
                                params=[start_blk, end_blk, {
                                    "fullTx": True,
                                    "inclStaking": True
                                }],
                                endpoint=endpoints[init_tx_record["from-shard"]])
    response = check_and_unpack_rpc_response(raw_response, expect_error=False)
    for blk in response:
        assert_valid_json_structure(reference_response_blk, blk)
    assert len(response[-1]["transactions"]) > 0, "Expected transaction on last block due to initial transactions"
    start_num, end_num = int(start_blk, 16), int(end_blk, 16)
    for blk in response:
        blk_num = int(blk["number"], 16)
        assert start_num <= blk_num <= end_num, f"Got block number {blk_num}, which is not in range [{start_num},{end_num}]"


@mutually_exclusive_test(scope=_mutex_scope)
def test_get_blocks_v2():
    """
    Only difference in param of RPC is hex string in v1 and decimal in v2.

    Note: param options for 'withSigners' will NOT return any sensical data
    in staking epoch (since it returns ONE addresses) and is subject to removal, thus is not tested here.
    """
    reference_response_blk = {
        "difficulty": 0,
        "epoch": 0,
        "extraData": "0x",
        "gasLimit": 80000000,
        "gasUsed": 105000,
        "hash": "0xf14c18213b1845ee09c41c5ecd321be5b745ef42e80d8e8a6bfd116452781465",
        "logsBloom": "0x00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
        "miner": "one1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqquzw7vz",
        "mixHash": "0x0000000000000000000000000000000000000000000000000000000000000000",
        "nonce": 0,
        "number": 3,
        "parentHash": "0xe4fe8484b94d3498acc183a8bcf9e0bef6d30b1a81d5628fbc67f9a7328651e8",
        "receiptsRoot": "0x100f3336862d22706bbe26d67e5abf90f8f25ec5a22c4446835b6beaa6b59536",
        "size": 1239,
        "stakingTransactions": [],
        "stateRoot": "0x1dc641cf516efe9ff2250f9c8db069ba4b9954160bf70cbe376ed11904b4edf5",
        "timestamp": 1594656146,
        "transactions": [
            {
                "blockHash": "0xf14c18213b1845ee09c41c5ecd321be5b745ef42e80d8e8a6bfd116452781465",
                "blockNumber": 3,
                "from": "one1zksj3evekayy90xt4psrz8h6j2v3hla4qwz4ur",
                "timestamp": 1594656146,
                "gas": 21000,
                "gasPrice": 1000000000,
                "hash": "0x5718a2fda967f051611ccfaf2230dc544c9bdd388f5759a42b2fb0847fc8d759",
                "input": "0x",
                "nonce": 0,
                "to": "one1v92y4v2x4q27vzydf8zq62zu9g0jl6z0lx2c8q",
                "transactionIndex": 0,
                "value": 100000000000000000000000,
                "shardID": 0,
                "toShardID": 0,
                "v": "0x28",
                "r": "0x76b6130bc018cedb9f8891343fd8982e0d7f923d57ea5250b8bfec9129d4ae22",
                "s": "0xfbc01c988d72235b4c71b21ce033d4fc5f82c96710b84685de0578cff075a0a"
            },
        ],
        "transactionsRoot": "0x8954e7b3ec6ef4b04dcaf2829d9ce8ed764f636445fe77d2f4e5ef157e69dbbd",
        "uncles": [],
        "viewID": 3
    }

    init_tx_record = initial_funding[0]
    init_tx = transaction.get_transaction_by_hash(init_tx_record["hash"], endpoints[init_tx_record["from-shard"]])
    start_blk, end_blk = max(0, int(init_tx["blockNumber"], 16) - 2), int(init_tx["blockNumber"], 16)
    raw_response = base_request("hmyv2_getBlocks",
                                params=[start_blk, end_blk, {
                                    "fullTx": True,
                                    "inclStaking": True
                                }],
                                endpoint=endpoints[init_tx_record["from-shard"]])
    response = check_and_unpack_rpc_response(raw_response, expect_error=False)
    for blk in response:
        assert_valid_json_structure(reference_response_blk, blk)
    assert len(response[-1]["transactions"]) > 0, "Expected transaction on last block due to initial transactions"
    for blk in response:
        assert start_blk <= blk[
            "number"] <= end_blk, f"Got block number {blk['number']}, which is not in range [{start_blk},{end_blk}]"


@mutually_exclusive_test(scope=_mutex_scope)
def test_get_block_by_hash_v1():
    reference_response = {
        "difficulty": 0,
        "epoch": "0x0",
        "extraData": "0x",
        "gasLimit": "0x4c4b400",
        "gasUsed": "0x19a28",
        "hash": "0x8e0ca00640ea70afe078d02fd571085f14d5953dca7be8ef4efc6a02db090156",
        "logsBloom": "0x00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
        "miner": "one1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqquzw7vz",
        "mixHash": "0x0000000000000000000000000000000000000000000000000000000000000000",
        "nonce": 0,
        "number": "0x9",
        "parentHash": "0xd5d0e786f1a0c6d8d2ea038ef5f8272bb4e4def1838d61b7e1c395f4b763c09d",
        "receiptsRoot": "0x100f3336862d22706bbe26d67e5abf90f8f25ec5a22c4446835b6beaa6b59536",
        "size": "0x96a",
        "stakingTransactions": [],
        "stateRoot": "0x8a39cac10f8d917564a4e6033dc303d6a47d9fc8768c220640de5bb89765e0cf",
        "timestamp": "0x5f0c976e",
        "transactions": [
            {
                "blockHash": "0x8e0ca00640ea70afe078d02fd571085f14d5953dca7be8ef4efc6a02db090156",
                "blockNumber": "0x9",
                "from": "one1zksj3evekayy90xt4psrz8h6j2v3hla4qwz4ur",
                "timestamp": "0x5f0c976e",
                "gas": "0x5208",
                "gasPrice": "0x3b9aca00",
                "hash": "0x5718a2fda967f051611ccfaf2230dc544c9bdd388f5759a42b2fb0847fc8d759",
                "input": "0x",
                "nonce": "0x0",
                "to": "one1v92y4v2x4q27vzydf8zq62zu9g0jl6z0lx2c8q",
                "transactionIndex": "0x0",
                "value": "0x152d02c7e14af6800000",
                "shardID": 0,
                "toShardID": 0,
                "v": "0x28",
                "r": "0x76b6130bc018cedb9f8891343fd8982e0d7f923d57ea5250b8bfec9129d4ae22",
                "s": "0xfbc01c988d72235b4c71b21ce033d4fc5f82c96710b84685de0578cff075a0a"
            },
        ],
        "transactionsRoot": "0x8954e7b3ec6ef4b04dcaf2829d9ce8ed764f636445fe77d2f4e5ef157e69dbbd",
        "uncles": [],
        "viewID": "0x9"
    }

    init_tx_record = initial_funding[0]
    init_tx = transaction.get_transaction_by_hash(init_tx_record["hash"], endpoints[init_tx_record["from-shard"]])
    raw_response = base_request("hmy_getBlockByHash",
                                params=[init_tx["blockHash"], True],
                                endpoint=endpoints[init_tx_record["from-shard"]])
    response = check_and_unpack_rpc_response(raw_response, expect_error=False)
    assert_valid_json_structure(reference_response, response)
    assert len(response["transactions"]) > 0, "Expected transaction on block due to initial transactions"
    for tx in response["transactions"]:
        assert tx["blockHash"] == init_tx[
            "blockHash"], f"Transaction in block {init_tx['blockHash']} does not have same block hash"
        assert tx["shardID"] == init_tx_record[
            "from-shard"], f"Transaction in block from shard {init_tx_record['from-shard']} does not have same from shard ({tx['shardID']})"


@mutually_exclusive_test(scope=_mutex_scope)
def test_get_block_by_hash_v2():
    """
    Note the use of a JSON object in the param. This is different from v1.

    Note: param options for 'withSigners' will NOT return any sensical data
    in staking epoch (since it returns ONE addresses) and is subject to removal, thus is not tested here.
    """
    reference_response = {
        "difficulty": 0,
        "epoch": 0,
        "extraData": "0x",
        "gasLimit": 80000000,
        "gasUsed": 105000,
        "hash": "0x8e0ca00640ea70afe078d02fd571085f14d5953dca7be8ef4efc6a02db090156",
        "logsBloom": "0x00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
        "miner": "one1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqquzw7vz",
        "mixHash": "0x0000000000000000000000000000000000000000000000000000000000000000",
        "nonce": 0,
        "number": 9,
        "parentHash": "0xd5d0e786f1a0c6d8d2ea038ef5f8272bb4e4def1838d61b7e1c395f4b763c09d",
        "receiptsRoot": "0x100f3336862d22706bbe26d67e5abf90f8f25ec5a22c4446835b6beaa6b59536",
        "size": 2410,
        "stateRoot": "0x8a39cac10f8d917564a4e6033dc303d6a47d9fc8768c220640de5bb89765e0cf",
        "timestamp": 1594660718,
        "transactions": [
            {
                "blockHash": "0x8e0ca00640ea70afe078d02fd571085f14d5953dca7be8ef4efc6a02db090156",
                "blockNumber": 9,
                "from": "one1zksj3evekayy90xt4psrz8h6j2v3hla4qwz4ur",
                "timestamp": 1594660718,
                "gas": 21000,
                "gasPrice": 1000000000,
                "hash": "0x5718a2fda967f051611ccfaf2230dc544c9bdd388f5759a42b2fb0847fc8d759",
                "input": "0x",
                "nonce": 0,
                "to": "one1v92y4v2x4q27vzydf8zq62zu9g0jl6z0lx2c8q",
                "transactionIndex": 0,
                "value": 100000000000000000000000,
                "shardID": 0,
                "toShardID": 0,
                "v": "0x28",
                "r": "0x76b6130bc018cedb9f8891343fd8982e0d7f923d57ea5250b8bfec9129d4ae22",
                "s": "0xfbc01c988d72235b4c71b21ce033d4fc5f82c96710b84685de0578cff075a0a"
            },
        ],
        "transactionsRoot": "0x8954e7b3ec6ef4b04dcaf2829d9ce8ed764f636445fe77d2f4e5ef157e69dbbd",
        "uncles": [],
        "viewID": 9
    }

    init_tx_record = initial_funding[0]
    init_tx = transaction.get_transaction_by_hash(init_tx_record["hash"], endpoints[init_tx_record["from-shard"]])
    raw_response = base_request("hmyv2_getBlockByHash",
                                params=[init_tx["blockHash"], {
                                    "fullTx": True,
                                    "inclTx": True
                                }],
                                endpoint=endpoints[init_tx_record["from-shard"]])
    response = check_and_unpack_rpc_response(raw_response, expect_error=False)
    assert_valid_json_structure(reference_response, response)
    assert len(response["transactions"]) > 0, "Expected transaction on block due to initial transactions"
    for tx in response["transactions"]:
        assert tx["blockHash"] == init_tx[
            "blockHash"], f"Transaction in block {init_tx['blockHash']} does not have same block hash"
        assert tx["shardID"] == init_tx_record[
            "from-shard"], f"Transaction in block from shard {init_tx_record['from-shard']} does not have same from shard ({tx['shardID']})"
