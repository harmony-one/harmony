#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
Tests here are related to sending a plain transaction & require a
feedback loop with the chain.

TODO: negative test cases

As with all tests, there are 2 JSON-RPC versions/namespaces (v1 & v2) where their difference
is only suppose to be in the types of their params & returns. v1 keeps everything in hex and
v2 uses decimal when possible. However, there are some (legacy) discrepancies that some tests
enforce. These tests are noted and should NOT be broken.
"""
import json
import time

import pytest
from flaky import flaky
from pyhmy import (
    account,
    blockchain,
)
from pyhmy.rpc.request import (
    base_request
)

import txs
from txs import (
    tx_timeout,
    endpoints,
    initial_funding,
    get_transaction,
    send_and_confirm_transaction,
    send_transaction
)
from utils import (
    check_and_unpack_rpc_response,
    assert_valid_json_structure,
    mutually_exclusive_test,
    rerun_delay_filter
)


_mutex_scope = "transaction"


@pytest.fixture(scope="module")
@txs.cross_shard
def cross_shard_txs():
    """
    Fixture for 2 cross shard transaction.

    Returned tuple has cx from s0 -> s1 as element 0, cx from s1 -> s0 as element 1.
    """
    s0_test_tx = {
        "from": "one1ue25q6jk0xk3dth4pxur9e742vcqfwulhwqh45",
        "to": "one1t40su52axu207vgc6ymcmwe0xmml4njrskk2vf",
        # erupt concert hat tree anger discover disease town gasp lemon gesture fiber spread season mixture host awake tennis issue orbit member film winter glass
        "amount": "1000",
        "from-shard": 0,
        "to-shard": 1,
        "hash": "0x9dae9c8e2c8abc8eebab1f7731f5c28cacf2ced913f4e4d49d000f74df0edca2",
        "nonce": "0x0",
        "signed-raw-tx": "0xf86f808506fc23ac008252088001945d5f0e515d3714ff3118d1378dbb2f36f7face43893635c9adc5dea000008027a0a45a914e5072d383158f790c34019c2ea2e60a1826cf2c95fa758b8a7168e52fa04ee2ac08ae98d0c9644feffdbc9b036d2be617cb7f92104ca7274eb9f102c9a7",
    }
    s1_test_tx = {
        "from": "one1t40su52axu207vgc6ymcmwe0xmml4njrskk2vf",
        "to": "one1qljfd3pnfjwr86ll6d0s6khcqhw8969p9l7fw3",
        # faculty pave mad mind siren unfold invite avocado teach engine mimic mouse frown topple match thunder syrup fame material feed occur kit install clog
        "amount": "500",
        "from-shard": 1,
        "to-shard": 0,
        "hash": "0x30e6cb1f8889d115512b1b5963aaefa66e180f350ed78c504d24b7ea224ac123",
        "nonce": "0x0",
        "signed-raw-tx": "0xf86f808506fc23ac0082520801809407e496c4334c9c33ebffd35f0d5af805dc72e8a1891b1ae4d6e2ef5000008027a093992f9dc99fa49f48afca645e41affa23efbdc2035da784524eda018fe3ead6a017471c901f17b9d467bf0a4ec1871870a6592d6e573148dd5091a8c979656ad3",
    }
    txs = [None, None]  # s0 -> s1 is element 0, s1 -> s0 is element 1

    in_initially_funded = False
    for tx in initial_funding:
        if tx["to"] == s0_test_tx["from"] and tx["to-shard"] == s0_test_tx["from-shard"]:
            in_initially_funded = True
            break
    if not in_initially_funded:
        raise AssertionError(f"Test transaction from address {s0_test_tx['from']} "
                             f"not found in set of initially funded accounts.")

    tx_response = get_transaction(s0_test_tx["hash"], s0_test_tx["from-shard"])
    txs[0] = send_and_confirm_transaction(s0_test_tx) if tx_response is None else tx_response
    start_time = time.time()
    while time.time() - start_time < tx_timeout:
        tx_response = get_transaction(s1_test_tx["hash"], s1_test_tx["from-shard"])
        if tx_response is not None:
            txs[1] = tx_response
            return tuple(txs)
        elif account.get_balance(s1_test_tx["from"], endpoint=endpoints[s1_test_tx["from-shard"]]) >= 1e18:
            txs[1] = send_and_confirm_transaction(s1_test_tx)
            return tuple(txs)
    raise AssertionError(f"Could not confirm cross shard transaction on 'to-shard' "
                         f"(balance not updated) for tx: {json.dumps(s0_test_tx, indent=2)}")


def test_get_pool_stats():
    """
    Note that v1 & v2 have the same responses.
    """
    reference_response = {
        "executable-count": 0,
        "non-executable-count": 0
    }

    raw_response = base_request("hmy_getPoolStats", params=[], endpoint=endpoints[0])
    response = check_and_unpack_rpc_response(raw_response, expect_error=False)
    assert_valid_json_structure(reference_response, response)

    raw_response = base_request("hmyv2_getPoolStats", params=[], endpoint=endpoints[0])
    response = check_and_unpack_rpc_response(raw_response, expect_error=False)
    assert_valid_json_structure(reference_response, response)


def test_get_current_transaction_error_sink():
    """
    Note that v1 & v2 have the same responses.
    """
    error_tx = {
        "from": "one1ujsjs4mhds75xnws0yx0v8l2rvyp67arwzqrvz",
        "to": "one1wfn43ynxhhdrrjnddqcr74u38frqc7hqjhhdkx",
        # odor middle lake course smooth drive tone oven stone canyon chapter special recall page tomorrow north moon impose original under shaft guess popular debate
        "amount": "1000000",
        "from-shard": 0,
        "to-shard": 0,
        "hash": "0x2f3be62d06027ec35369680329b2796cd811a902f2fe5cd67d761e962a3639a4",
        "nonce": "0x0",
        "signed-raw-tx": "0xf870808506fc23ac008252088080947267589266bdda31ca6d68303f57913a460c7ae08ad3c21bcecceda10000008028a06fb1e5a03bf27f68d48b79cb12dfefcdefcaa72113f80bf3a9ccc7f4614bcfa3a018ba6c8c1e7bba73352ba966b28e5327ba253f70bc6032cab2d5a0ce12f3bd6f",
    }
    reference_response = [
        {
            "tx-hash-id": "0x371a399f7f62a5f372d3388a07250e16ee56ac763bd3a0c8c5f628f1e1975679",
            "time-at-rejection": 1594797464,
            "error-message": "transaction gas-price is 0.000000000000000000 ONE: transaction underpriced"
        }
    ]

    response = base_request('hmy_sendRawTransaction', params=[error_tx["signed-raw-tx"]],
                            endpoint=endpoints[error_tx["from-shard"]])
    check_and_unpack_rpc_response(response, expect_error=True)  # Send invalid transaction directly...

    # Check v1
    raw_response = base_request("hmy_getCurrentTransactionErrorSink", params=[],
                                endpoint=endpoints[error_tx["from-shard"]])
    response = check_and_unpack_rpc_response(raw_response, expect_error=False)
    assert_valid_json_structure(reference_response, response)

    found_errored_tx = False
    for err in response:
        if err["tx-hash-id"] == error_tx["hash"]:
            found_errored_tx = True
            break
    assert found_errored_tx, f"Could not find errored transaction (hash {error_tx['hash']}) in {json.dumps(response, indent=2)}"

    # Check v2
    raw_response = base_request("hmyv2_getCurrentTransactionErrorSink", params=[],
                                endpoint=endpoints[error_tx["from-shard"]])
    response = check_and_unpack_rpc_response(raw_response, expect_error=False)
    assert_valid_json_structure(reference_response, response)

    found_errored_tx = False
    for err in response:
        if err["tx-hash-id"] == error_tx["hash"]:
            found_errored_tx = True
            break
    assert found_errored_tx, f"Could not find errored transaction (hash {error_tx['hash']}) in {json.dumps(response, indent=2)}"


@flaky(max_runs=6, rerun_filter=rerun_delay_filter(delay=8))
@mutually_exclusive_test(scope=_mutex_scope)
@txs.cross_shard
def test_resend_cx(cross_shard_txs):
    """
    Note that v1 & v2 have the same responses.
    """
    reference_response = True

    for tx in cross_shard_txs:
        raw_response = base_request("hmy_resendCx", params=[tx["hash"]],
                                    endpoint=endpoints[tx["shardID"]])
        response = check_and_unpack_rpc_response(raw_response, expect_error=False)
        assert_valid_json_structure(reference_response, response)

        raw_response = base_request("hmyv2_resendCx", params=[tx["hash"]],
                                    endpoint=endpoints[tx["shardID"]])
        response = check_and_unpack_rpc_response(raw_response, expect_error=False)
        assert_valid_json_structure(reference_response, response)


@flaky(max_runs=6, rerun_filter=rerun_delay_filter(delay=8))
@txs.cross_shard
def test_get_pending_cx_receipts():
    """
    Note that v1 & v2 have the same responses.
    """
    cx = {
        "from": "one19l4hghvh40fyldxfznn0a3ss7d5gk0dmytdql4",
        "to": "one1ds3fayprfl6j7yd6mpwfncj9c0ajmhvmvhnmpm",
        # erupt concert hat tree anger discover disease town gasp lemon gesture fiber spread season mixture host awake tennis issue orbit member film winter glass
        "amount": "1000",
        "from-shard": 0,
        "to-shard": 1,
        "hash": "0x33e4c0a582f87731a474332ca712b20f322afd0221aa3f4a287c985afab52476",
        "nonce": "0x0",
        "signed-raw-tx": "0xf86f808506fc23ac008252088001946c229e90234ff52f11bad85c99e245c3fb2ddd9b893635c9adc5dea000008027a0978e0924045ba31ea891da261895f3d2840e687f0fd5df35208df2358bac69e7a05d588e2d823b65773537337d434affd0374c1331cd96841ff1513ee55504a8d1",
    }
    reference_response = [
        {
            "receipts": [
                {
                    "txHash": "0x819b0d7902134dadd07851edba0e8694e60c1aee057a96d2ceb4a9118cee0298",
                    "from": "one1t40su52axu207vgc6ymcmwe0xmml4njrskk2vf",
                    "to": "one1qljfd3pnfjwr86ll6d0s6khcqhw8969p9l7fw3",
                    "shardID": 1,
                    "toShardID": 0,
                    "amount": 500000000000000000000
                }
            ],
            "merkleProof": {
                "blockNum": 35,
                "blockHash": "0xe07abb23824f658f452012f22e2d557a270c320058a39d6c6d5d2d53d1d7e427",
                "shardID": 1,
                "receiptHash": "0xb7f422b693a5cffd3d98b2fd4f9f833e10421bcd6d488e5cd8c2fcbcf1ecd13c",
                "shardIDs": [
                    0
                ],
                "shardHashes": [
                    "0x31db710789deaa5a1721f7bf66d3eabddfbb9e712b5ba6cdc7b183f5d9dc9b51"
                ]
            },
            "header": {
                "shardID": 1,
                "hash": "0x2e0295f760bc69cdf840576636f61602f8b13ea5172562837c10a9b6f5fa711e",
                "number": "0x23",
                "viewID": 35,
                "epoch": 5
            },
            "commitSig": "G7oQCfiRJjl8s1i7B2xxPWZefCW5muiqyNY0PwcNOFt2QQkRC95ongKIGuIKCLMAVkDpkZRdC7B0cUoe3tKceT6/9++sxcwPRQ2NBWA/u6Gkl6UneKs4Xzhpuez2MoOG",
            "commitBitmap": "Pw=="
        }
    ]

    if get_transaction(cx["hash"], cx["from-shard"]) is not None:
        pytest.skip(f"Test cross shard transaction (hash {cx['hash']}) already present on chain...")

    send_transaction(cx, confirm_submission=True)

    start_time = time.time()
    v1_success, v2_success = False, False
    while time.time() - start_time <= tx_timeout * 2:  # Cross shards are generally slower...
        if not v1_success:
            raw_response = base_request("hmy_getPendingCXReceipts", endpoint=endpoints[cx["to-shard"]])
            response = check_and_unpack_rpc_response(raw_response, expect_error=False)
            assert_valid_json_structure(reference_response, response)
            for cx_receipt in response:
                for r in cx_receipt["receipts"]:
                    if r["txHash"] == cx["hash"]:
                        v1_success = True

        if not v2_success:
            raw_response = base_request("hmyv2_getPendingCXReceipts", endpoint=endpoints[cx["to-shard"]])
            response = check_and_unpack_rpc_response(raw_response, expect_error=False)
            assert_valid_json_structure(reference_response, response)
            for cx_receipt in response:
                for r in cx_receipt["receipts"]:
                    if r["txHash"] == cx["hash"]:
                        v2_success = True

        time.sleep(0.5)
        if v1_success and v2_success:
            return

    raise AssertionError(f"Timeout! Pending transaction not found for {json.dumps(cx)}")


@flaky(max_runs=6, rerun_filter=rerun_delay_filter(delay=8))
@mutually_exclusive_test(scope=_mutex_scope)
@txs.cross_shard
def test_get_cx_receipt_by_hash_v1(cross_shard_txs):
    reference_response = {
        "blockHash": "0xf12f3aefd7f189286b6da30871a47946c11f9c1673b3b693f9d37d659f69e018",
        "blockNumber": "0x21",
        "hash": "0xc0a84ec15fc3391089f20fa6b9cc90c654eb8dd2f6815297de89eef38ce4fe2b",
        "from": "one1ue25q6jk0xk3dth4pxur9e742vcqfwulhwqh45",
        "to": "one1t40su52axu207vgc6ymcmwe0xmml4njrskk2vf",
        "shardID": 0,
        "toShardID": 1,
        "value": "0x3635c9adc5dea00000"
    }

    raw_response = base_request("hmy_getCXReceiptByHash", params=[cross_shard_txs[0]["hash"]],
                                endpoint=endpoints[cross_shard_txs[0]["toShardID"]])
    response = check_and_unpack_rpc_response(raw_response, expect_error=False)
    assert_valid_json_structure(reference_response, response)


@flaky(max_runs=6, rerun_filter=rerun_delay_filter(delay=8))
@mutually_exclusive_test(scope=_mutex_scope)
@txs.cross_shard
def test_get_cx_receipt_by_hash_v2(cross_shard_txs):
    reference_response = {
        "blockHash": "0xf12f3aefd7f189286b6da30871a47946c11f9c1673b3b693f9d37d659f69e018",
        "blockNumber": 33,
        "hash": "0xc0a84ec15fc3391089f20fa6b9cc90c654eb8dd2f6815297de89eef38ce4fe2b",
        "from": "one1ue25q6jk0xk3dth4pxur9e742vcqfwulhwqh45",
        "to": "one1t40su52axu207vgc6ymcmwe0xmml4njrskk2vf",
        "shardID": 0,
        "toShardID": 1,
        "value": 1000000000000000000000
    }

    raw_response = base_request("hmyv2_getCXReceiptByHash", params=[cross_shard_txs[0]["hash"]],
                                endpoint=endpoints[cross_shard_txs[0]["toShardID"]])
    response = check_and_unpack_rpc_response(raw_response, expect_error=False)
    assert_valid_json_structure(reference_response, response)


@pytest.mark.run(order=0)
def test_send_raw_transaction_v1():
    tx = {
        "from": "one1pvkjamc0q96s6z62qzz6e09k2qrqqdj34ylxvd",
        "to": "one1mjunf85vnhc4drv57ugsyg2fxjnsq920qzkpwq",
        # identify energy glimpse train script text town amused major slot armed fiction park alter dance live snow path picture desk metal voice distance good
        "amount": "1000",
        "from-shard": 0,
        "to-shard": 0,
        "hash": "0x5e1ecefd00a7ce07ad1b307ac6da86544c68815eed4b5c605f151e538d2b416c",
        "nonce": "0x0",
        "signed-raw-tx": "0xf86f808506fc23ac00825208808094dcb9349e8c9df1568d94f71102214934a700154f893635c9adc5dea000008028a0c4214018d34fe9d2021c9bdc9a05247de4d55a304ef4d5a7c5fefea629008697a03edf9fc15552602015dbd1091dd71db17efb0a52d664aeb639cb373dbe20e81f",
    }
    reference_response = {
        "code": -32000,
        "message": "transaction already finalized"
    }

    if get_transaction(tx["hash"], tx["from-shard"]) is not None:
        pytest.skip(f"Test transaction (hash {tx['hash']}) already present on chain...")

    # Submit new transaction...
    response = base_request('hmy_sendRawTransaction', params=[tx["signed-raw-tx"]],
                            endpoint=endpoints[tx["from-shard"]])
    tx_hash = check_and_unpack_rpc_response(response, expect_error=False)
    assert tx_hash == tx["hash"], f"Expect submitted transaction to get tx hash of {tx['hash']}, got {tx_hash}"

    # Test finalized transaction error...
    start_time = time.time()
    while time.time() - start_time <= tx_timeout:
        if get_transaction(tx["hash"], tx["from-shard"]) is not None:
            raw_response = base_request('hmy_sendRawTransaction', params=[tx["signed-raw-tx"]],
                                        endpoint=endpoints[tx["from-shard"]])
            response = check_and_unpack_rpc_response(raw_response, expect_error=True)
            assert_valid_json_structure(reference_response, response)
            assert reference_response["code"] == response["code"], f"Expected error code {reference_response['code']}, " \
                                                                   f"got {response['code']}"
            return
        time.sleep(0.25)
    raise AssertionError(f"Timeout! Finalized transaction not found for {json.dumps(tx, indent=2)}")


@pytest.mark.run(order=0)
def test_send_raw_transaction_v2():
    tx = {
        "from": "one13lu674f3jkfk2qhsngfc2vhcf372wprctdjvgu",
        "to": "one14jeshxg75gdr5dz8sg7fm2sjvw7snnsdw98f0y",
        # humor brain crouch walk focus slush material sort used refuse exist prefer obscure above grow maze scheme myself liquid lab fresh awful easily debris
        "amount": "1000",
        "from-shard": 0,
        "to-shard": 0,
        "hash": "0x73e2aa48841b6d87eb6d7a4f6fc0e4253e1c806fdacd420b4d100f718e9dcd62",
        "nonce": "0x0",
        "signed-raw-tx": "0xf86f808506fc23ac00825208808094acb30b991ea21a3a3447823c9daa1263bd09ce0d893635c9adc5dea000008028a02607a24abff19534b98b0a6e365ff56f7db2449e793ce6e241263154b7527d81a02db39163c82772ab5b5276f096a39ab0ed5608f187e0b3be0273024a98c5a205",
    }
    reference_response = {
        "code": -32000,
        "message": "transaction already finalized"
    }

    if get_transaction(tx["hash"], tx["from-shard"]) is not None:
        pytest.skip(f"Test transaction (hash {tx['hash']}) already present on chain...")

    # Submit new transaction...
    response = base_request('hmyv2_sendRawTransaction', params=[tx["signed-raw-tx"]],
                            endpoint=endpoints[tx["from-shard"]])
    tx_hash = check_and_unpack_rpc_response(response, expect_error=False)
    assert tx_hash == tx["hash"], f"Expect submitted transaction to get tx hash of {tx['hash']}, got {tx_hash}"

    # Test finalized transaction error...
    start_time = time.time()
    while time.time() - start_time <= tx_timeout:
        if get_transaction(tx["hash"], tx["from-shard"]) is not None:
            raw_response = base_request('hmyv2_sendRawTransaction', params=[tx["signed-raw-tx"]],
                                        endpoint=endpoints[tx["from-shard"]])
            response = check_and_unpack_rpc_response(raw_response, expect_error=True)
            assert_valid_json_structure(reference_response, response)
            assert reference_response["code"] == response["code"], f"Expected error code {reference_response['code']}, " \
                                                                   f"got {response['code']}"
            return
        time.sleep(0.25)
    raise AssertionError(f"Timeout! Finalized transaction not found for {json.dumps(tx, indent=2)}")


@pytest.mark.run(order=0)
def test_get_transaction_by_hash_v1():
    reference_response = {
        "blockHash": "0x08ef4c7b1d24f27be157bdf9f053d3fd2fabc81037cf87f83b000804bc2e1c9f",
        "blockNumber": "0x4",
        "from": "one1zksj3evekayy90xt4psrz8h6j2v3hla4qwz4ur",
        "timestamp": "0x5f0ec12e",
        "gas": "0x5208",
        "gasPrice": "0x6fc23ac00",
        "hash": "0x4553da3a01770e4048862c39dd8f2996eacf990cf40932a358405239fe3650fc",
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
    }
    init_tx_record = initial_funding[0]

    raw_response = base_request("hmy_getTransactionByHash",
                                params=[init_tx_record["hash"]],
                                endpoint=endpoints[init_tx_record["from-shard"]])
    response = check_and_unpack_rpc_response(raw_response, expect_error=False)
    assert_valid_json_structure(reference_response, response)
    assert response["hash"] == init_tx_record["hash"], f"Expected transaction {init_tx_record['hash']}, " \
                                                       f"got {response['hash']}"


@pytest.mark.run(order=0)
def test_get_transaction_by_hash_v2():
    reference_response = {
        "blockHash": "0x08ef4c7b1d24f27be157bdf9f053d3fd2fabc81037cf87f83b000804bc2e1c9f",
        "blockNumber": 4,
        "from": "one1zksj3evekayy90xt4psrz8h6j2v3hla4qwz4ur",
        "timestamp": 1594802478,
        "gas": 21000,
        "gasPrice": 30000000000,
        "hash": "0x4553da3a01770e4048862c39dd8f2996eacf990cf40932a358405239fe3650fc",
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
    }
    init_tx_record = initial_funding[0]

    raw_response = base_request("hmyv2_getTransactionByHash",
                                params=[init_tx_record["hash"]],
                                endpoint=endpoints[init_tx_record["from-shard"]])
    response = check_and_unpack_rpc_response(raw_response, expect_error=False)
    assert_valid_json_structure(reference_response, response)
    assert response["hash"] == init_tx_record["hash"], f"Expected transaction {init_tx_record['hash']}, " \
                                                       f"got {response['hash']}"


@pytest.mark.run(order=0)
def test_get_transaction_receipt_v1():
    reference_response = {
        "blockHash": "0x08ef4c7b1d24f27be157bdf9f053d3fd2fabc81037cf87f83b000804bc2e1c9f",
        "blockNumber": "0x4",
        "contractAddress": None,
        "cumulativeGasUsed": "0x5208",
        "from": "one1zksj3evekayy90xt4psrz8h6j2v3hla4qwz4ur",
        "gasUsed": "0x5208",
        "logs": [],
        "logsBloom": "0x00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
        "shardID": 0,
        "status": "0x1",
        "to": "one1v92y4v2x4q27vzydf8zq62zu9g0jl6z0lx2c8q",
        "transactionHash": "0x4553da3a01770e4048862c39dd8f2996eacf990cf40932a358405239fe3650fc",
        "transactionIndex": "0x0"
    }
    init_tx_record = initial_funding[0]

    raw_response = base_request("hmy_getTransactionReceipt",
                                params=[init_tx_record["hash"]],
                                endpoint=endpoints[init_tx_record["from-shard"]])
    response = check_and_unpack_rpc_response(raw_response, expect_error=False)
    assert_valid_json_structure(reference_response, response)
    assert response["transactionHash"] == init_tx_record["hash"], f"Expected transaction {init_tx_record['hash']}, " \
                                                                  f"got {response['transactionHash']}"


@pytest.mark.run(order=0)
def test_get_transaction_receipt_v2():
    reference_response = {
        "blockHash": "0x08ef4c7b1d24f27be157bdf9f053d3fd2fabc81037cf87f83b000804bc2e1c9f",
        "blockNumber": 4,
        "contractAddress": None,
        "cumulativeGasUsed": 21000,
        "from": "one1zksj3evekayy90xt4psrz8h6j2v3hla4qwz4ur",
        "gasUsed": 21000,
        "logs": [],
        "logsBloom": "0x00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
        "shardID": 0,
        "status": 1,
        "to": "one1v92y4v2x4q27vzydf8zq62zu9g0jl6z0lx2c8q",
        "transactionHash": "0x4553da3a01770e4048862c39dd8f2996eacf990cf40932a358405239fe3650fc",
        "transactionIndex": 0
    }
    init_tx_record = initial_funding[0]

    raw_response = base_request("hmyv2_getTransactionReceipt",
                                params=[init_tx_record["hash"]],
                                endpoint=endpoints[init_tx_record["from-shard"]])
    response = check_and_unpack_rpc_response(raw_response, expect_error=False)
    assert_valid_json_structure(reference_response, response)
    assert response["transactionHash"] == init_tx_record["hash"], f"Expected transaction {init_tx_record['hash']}, " \
                                                                  f"got {response['transactionHash']}"


def test_pending_transactions_v1():
    tx = {
        "from": "one1twhzfc2wr4j5ka7gs9pmllpnrdyaskcl5lq8ye",
        "to": "one13awvzpjt7n3hcrmxax3elps7a6vw46u63kc28p",
        # month liar edit pull vague intact entire slab satoshi angle core unlock useless wrestle kite merry sure quiz day frame update recycle fault lecture
        "amount": "1000",
        "from-shard": 0,
        "to-shard": 0,
        "hash": "0x59272596c3633c486828755f8c56c379392d51eb5dd96f7c5b6a0805ff39c704",
        "nonce": "0x0",
        "signed-raw-tx": "0xf86f808506fc23ac008252088080948f5cc1064bf4e37c0f66e9a39f861eee98eaeb9a893635c9adc5dea000008028a044385f2ccbc98d0ff8dfb4e5109334aed1e1d2e6aca327e07afc18d33e9cc236a060aa69ecddc755ed3ecf1cb2d59819b1d546241b448b71f64c4e32236d36c1cd",
    }
    reference_response = [
        {
            "blockHash": "0x0000000000000000000000000000000000000000000000000000000000000000",
            "blockNumber": None,
            "from": "one1twhzfc2wr4j5ka7gs9pmllpnrdyaskcl5lq8ye",
            "timestamp": "0x0",
            "gas": "0x5208",
            "gasPrice": "0x6fc23ac00",
            "hash": "0x59272596c3633c486828755f8c56c379392d51eb5dd96f7c5b6a0805ff39c704",
            "input": "0x",
            "nonce": "0x0",
            "to": "one13awvzpjt7n3hcrmxax3elps7a6vw46u63kc28p",
            "transactionIndex": "0x0",
            "value": "0x3635c9adc5dea00000",
            "shardID": 0,
            "toShardID": 0,
            "v": "0x28",
            "r": "0xe876d901525a8799a8eb3ea03e2c1a43129c2ff3136ec10f6345f2899bab5026",
            "s": "0x5c4f1e659b9d371c2e9994aee240b966e36b6dd609747d42c9d9c9f23371d808"
        }
    ]

    in_initially_funded = False
    for init_tx in initial_funding:
        if init_tx["to"] == tx["from"] and init_tx["to-shard"] == tx["from-shard"]:
            in_initially_funded = True
            break
    if not in_initially_funded:
        raise AssertionError(f"Test transaction from address {tx['from']} "
                             f"not found in set of initially funded accounts.")

    if get_transaction(tx["hash"], tx["from-shard"]) is not None:
        pytest.skip(f"Test transaction (hash {tx['hash']}) already present on chain...")

    send_transaction(tx, confirm_submission=True)

    start_time = time.time()
    while time.time() - start_time <= tx_timeout:
        raw_response = base_request("hmy_pendingTransactions", endpoint=endpoints[tx["from-shard"]])
        response = check_and_unpack_rpc_response(raw_response, expect_error=False)
        assert_valid_json_structure(reference_response, response)
        for pending_tx in response:
            if pending_tx["hash"] == tx["hash"]:
                assert pending_tx["shardID"] == tx["from-shard"], f"Pending tx has from shard {pending_tx['shardID']}, " \
                                                                  f"expected shard {tx['from-shard']}"
                assert pending_tx["toShardID"] == tx["to-shard"], f"Pending tx has to shard {pending_tx['toShardID']}, " \
                                                                  f"expected shard {tx['to-shard']}"
                return

    raise AssertionError(f"Timeout! Pending transaction not found for {json.dumps(tx, indent=2)}")


def test_pending_transactions_v2():
    tx = {
        "from": "one1u57rlv5q82deja6ew2l9hdy7ag3dwnw57x8s9t",
        "to": "one1zchjhmsxksamlxuv7h3k9h5aeury670n2jck2u",
        # kit attack eternal net bronze grace apple evil market spin evil tragic kid capital noble future shrimp gossip flee wonder album ahead catalog crawl
        "amount": "1000",
        "from-shard": 0,
        "to-shard": 0,
        "hash": "0x82fd4678de1df49eb8dc4ca361300c950712bd7f328990e10ff6c252a86050b6",
        "nonce": "0x0",
        "signed-raw-tx": "0xf86f808506fc23ac00825208808094162f2bee06b43bbf9b8cf5e362de9dcf064d79f3893635c9adc5dea000008028a07a3dd13017fe6468b087fd3aec0cb2b6896fb4f78e3aa985172c39b3cd9db27aa068e6a91e4918342ee725d9e1cc29f6849b690df04bfde13145c8dd65f475b1f0",
    }
    reference_response = [
        {
            "blockHash": "0x0000000000000000000000000000000000000000000000000000000000000000",
            "blockNumber": None,
            "from": "one1twhzfc2wr4j5ka7gs9pmllpnrdyaskcl5lq8ye",
            "timestamp": 0,
            "gas": 21000,
            "gasPrice": 30000000000,
            "hash": "0x82fd4678de1df49eb8dc4ca361300c950712bd7f328990e10ff6c252a86050b6",
            "input": "0x",
            "nonce": 0,
            "to": "one13awvzpjt7n3hcrmxax3elps7a6vw46u63kc28p",
            "transactionIndex": 0,
            "value": 1000000000000000000000,
            "shardID": 0,
            "toShardID": 0,
            "v": "0x28",
            "r": "0xe876d901525a8799a8eb3ea03e2c1a43129c2ff3136ec10f6345f2899bab5026",
            "s": "0x5c4f1e659b9d371c2e9994aee240b966e36b6dd609747d42c9d9c9f23371d808"
        }
    ]

    if get_transaction(tx["hash"], tx["from-shard"]) is not None:
        pytest.skip(f"Test transaction (hash {tx['hash']}) already present on chain...")

    send_transaction(tx, confirm_submission=True)

    start_time = time.time()
    while time.time() - start_time <= tx_timeout:
        raw_response = base_request("hmyv2_pendingTransactions", endpoint=endpoints[tx["from-shard"]])
        response = check_and_unpack_rpc_response(raw_response, expect_error=False)
        assert_valid_json_structure(reference_response, response)
        for pending_tx in response:
            if pending_tx["hash"] == tx["hash"]:
                assert pending_tx["shardID"] == tx["from-shard"], f"Pending tx has from shard {pending_tx['shardID']}, " \
                                                                  f"expected shard {tx['from-shard']}"
                assert pending_tx["toShardID"] == tx["to-shard"], f"Pending tx has to shard {pending_tx['toShardID']}, " \
                                                                  f"expected shard {tx['to-shard']}"
                return

    raise AssertionError(f"Timeout! Pending transaction not found for {json.dumps(tx, indent=2)}")


@pytest.mark.run(order=1)
def test_get_transaction_by_block_hash_and_index_v1():
    reference_response = {
        "blockHash": "0x08ef4c7b1d24f27be157bdf9f053d3fd2fabc81037cf87f83b000804bc2e1c9f",
        "blockNumber": "0x4",
        "from": "one1zksj3evekayy90xt4psrz8h6j2v3hla4qwz4ur",
        "timestamp": "0x5f0ec12e",
        "gas": "0x5208",
        "gasPrice": "0x6fc23ac00",
        "hash": "0x4553da3a01770e4048862c39dd8f2996eacf990cf40932a358405239fe3650fc",
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
    }

    init_tx_record = initial_funding[0]
    tx = get_transaction(init_tx_record["hash"], init_tx_record["from-shard"])
    blk = blockchain.get_block_by_hash(tx["blockHash"], endpoint=endpoints[tx["shardID"]], include_full_tx=False)
    index, blk_hash = blk["transactions"].index(init_tx_record["hash"]), tx["blockHash"]

    raw_response = base_request("hmy_getTransactionByBlockHashAndIndex",
                                params=[blk_hash, hex(index)],
                                endpoint=endpoints[init_tx_record["from-shard"]])
    response = check_and_unpack_rpc_response(raw_response, expect_error=False)
    assert_valid_json_structure(reference_response, response)
    assert response["hash"] == init_tx_record["hash"], f"Expected transaction {init_tx_record['hash']}, " \
                                                       f"got {response['hash']}"


@pytest.mark.run(order=1)
def test_get_transaction_by_block_hash_and_index_v2():
    reference_response = {
        "blockHash": "0x08ef4c7b1d24f27be157bdf9f053d3fd2fabc81037cf87f83b000804bc2e1c9f",
        "blockNumber": 4,
        "from": "one1zksj3evekayy90xt4psrz8h6j2v3hla4qwz4ur",
        "timestamp": 1594802478,
        "gas": 21000,
        "gasPrice": 30000000000,
        "hash": "0x4553da3a01770e4048862c39dd8f2996eacf990cf40932a358405239fe3650fc",
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
    }

    init_tx_record = initial_funding[0]
    tx = get_transaction(init_tx_record["hash"], init_tx_record["from-shard"])
    blk = blockchain.get_block_by_hash(tx["blockHash"], endpoint=endpoints[tx["shardID"]], include_full_tx=False)
    index, blk_hash = blk["transactions"].index(init_tx_record["hash"]), tx["blockHash"]

    raw_response = base_request("hmyv2_getTransactionByBlockHashAndIndex",
                                params=[blk_hash, index],
                                endpoint=endpoints[init_tx_record["from-shard"]])
    response = check_and_unpack_rpc_response(raw_response, expect_error=False)
    assert_valid_json_structure(reference_response, response)
    assert response["hash"] == init_tx_record["hash"], f"Expected transaction {init_tx_record['hash']}, " \
                                                       f"got {response['hash']}"


@pytest.mark.run(order=1)
def test_get_transaction_by_block_number_and_index_v1():
    reference_response = {
        "blockHash": "0x08ef4c7b1d24f27be157bdf9f053d3fd2fabc81037cf87f83b000804bc2e1c9f",
        "blockNumber": "0x4",
        "from": "one1zksj3evekayy90xt4psrz8h6j2v3hla4qwz4ur",
        "timestamp": "0x5f0ec12e",
        "gas": "0x5208",
        "gasPrice": "0x6fc23ac00",
        "hash": "0x4553da3a01770e4048862c39dd8f2996eacf990cf40932a358405239fe3650fc",
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
    }

    init_tx_record = initial_funding[0]
    tx = get_transaction(init_tx_record["hash"], init_tx_record["from-shard"])
    blk = blockchain.get_block_by_hash(tx["blockHash"], endpoint=endpoints[tx["shardID"]], include_full_tx=False)
    index, blk_num = blk["transactions"].index(init_tx_record["hash"]), tx["blockNumber"]

    raw_response = base_request("hmy_getTransactionByBlockNumberAndIndex",
                                params=[blk_num, hex(index)],
                                endpoint=endpoints[init_tx_record["from-shard"]])
    response = check_and_unpack_rpc_response(raw_response, expect_error=False)
    assert_valid_json_structure(reference_response, response)
    assert response["hash"] == init_tx_record["hash"], f"Expected transaction {init_tx_record['hash']}, " \
                                                       f"got {response['hash']}"


@pytest.mark.run(order=1)
def test_get_transaction_by_block_number_and_index_v2():
    reference_response = {
        "blockHash": "0x08ef4c7b1d24f27be157bdf9f053d3fd2fabc81037cf87f83b000804bc2e1c9f",
        "blockNumber": 4,
        "from": "one1zksj3evekayy90xt4psrz8h6j2v3hla4qwz4ur",
        "timestamp": 1594802478,
        "gas": 21000,
        "gasPrice": 30000000000,
        "hash": "0x4553da3a01770e4048862c39dd8f2996eacf990cf40932a358405239fe3650fc",
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
    }

    init_tx_record = initial_funding[0]
    tx = get_transaction(init_tx_record["hash"], init_tx_record["from-shard"])
    blk = blockchain.get_block_by_hash(tx["blockHash"], endpoint=endpoints[tx["shardID"]], include_full_tx=False)
    index, blk_num = blk["transactions"].index(init_tx_record["hash"]), tx["blockNumber"]

    raw_response = base_request("hmyv2_getTransactionByBlockNumberAndIndex",
                                params=[int(blk_num, 16), index],
                                endpoint=endpoints[init_tx_record["from-shard"]])
    response = check_and_unpack_rpc_response(raw_response, expect_error=False)
    assert_valid_json_structure(reference_response, response)
    assert response["hash"] == init_tx_record["hash"], f"Expected transaction {init_tx_record['hash']}, " \
                                                       f"got {response['hash']}"
