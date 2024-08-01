#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
Tests here are related to account information. Some tests require a
feedback loop with the chain.

TODO: negative test cases

As with all tests, there are 2 JSON-RPC versions/namespaces (v1 & v2) where their difference
is only suppose to be in the types of their params & returns. v1 keeps everything in hex and
v2 uses decimal when possible. However, there are some (legacy) discrepancies that some tests
enforce. These tests are noted and should NOT be broken.
"""
import pytest
from pyhmy.rpc.request import (
    base_request
)

from txs import (
    endpoints,
    initial_funding,
    get_transaction,
    send_and_confirm_transaction
)
from utils import (
    check_and_unpack_rpc_response,
    assert_valid_json_structure
)


@pytest.fixture(scope="module")
def account_test_tx():
    """
    Fixture to send (if needed) and return a transaction to be
    used for all tests in this test module.
    """
    test_tx = {
        "from": "one1v92y4v2x4q27vzydf8zq62zu9g0jl6z0lx2c8q",
        "to": "one1s92wjv7xeh962d4sfc06q0qauxak4k8hh74ep3",
        "amount": "1000",
        "from-shard": 0,
        "to-shard": 0,
        "hash": "0x3759ab736798efe609bd1eb69145616ccc57baea99a9abab4e259e935f488bc5",
        "nonce": "0x0",
        "signed-raw-tx": "0xf86f808506fc23ac008252088080948154e933c6cdcba536b04e1fa03c1de1bb6ad8f7893635c9adc5dea000008028a086f785f5893efe3ff12b3fdaa0e52ae6a7b5b6897edb15f177836df355cbf2dea0648149e5feccfff1418e8cd6ac55bb9f26423f95344e558d917b25d4b7ac88df",
    }

    in_initially_funded = False
    for tx in initial_funding:
        if tx["to"] == test_tx["from"] and tx["to-shard"] == test_tx["from-shard"]:
            in_initially_funded = True
            break
    if not in_initially_funded:
        raise AssertionError(f"Test transaction from address {test_tx['from']} "
                             f"not found in set of initially funded accounts.")

    tx_response = get_transaction(test_tx["hash"], test_tx["from-shard"])
    return send_and_confirm_transaction(test_tx) if tx_response is None else tx_response


def test_get_transactions_count(account_test_tx):
    """
    Note that v1 & v2 have the same responses.
    """
    reference_response = 0

    # Check v1, SENT
    raw_response = base_request("hmy_getTransactionsCount",
                                params=[account_test_tx["to"], "SENT"],
                                endpoint=endpoints[account_test_tx["shardID"]])
    response = check_and_unpack_rpc_response(raw_response, expect_error=False)
    assert_valid_json_structure(reference_response, response)
    assert response == 0, f"Expected account  {account_test_tx['to']} to have 0 sent transactions"

    # Check v2, SENT
    raw_response = base_request("hmyv2_getTransactionsCount",
                                params=[account_test_tx["to"], "SENT"],
                                endpoint=endpoints[account_test_tx["shardID"]])
    response = check_and_unpack_rpc_response(raw_response, expect_error=False)
    assert_valid_json_structure(reference_response, response)
    assert response == 0, f"Expected account  {account_test_tx['to']} to have 0 sent transactions"

    # Check v1, RECEIVED
    raw_response = base_request("hmy_getTransactionsCount",
                                params=[account_test_tx["to"], "RECEIVED"],
                                endpoint=endpoints[account_test_tx["shardID"]])
    response = check_and_unpack_rpc_response(raw_response, expect_error=False)
    assert_valid_json_structure(reference_response, response)
    assert response == 1, f"Expected account  {account_test_tx['to']} to have 1 received transactions"

    # Check v2, RECEIVED
    raw_response = base_request("hmyv2_getTransactionsCount",
                                params=[account_test_tx["to"], "RECEIVED"],
                                endpoint=endpoints[account_test_tx["shardID"]])
    response = check_and_unpack_rpc_response(raw_response, expect_error=False)
    assert_valid_json_structure(reference_response, response)
    assert response == 1, f"Expected account  {account_test_tx['to']} to have 1 received transactions"

    # Check v1, ALL
    raw_response = base_request("hmy_getTransactionsCount",
                                params=[account_test_tx["to"], "ALL"],
                                endpoint=endpoints[account_test_tx["shardID"]])
    response = check_and_unpack_rpc_response(raw_response, expect_error=False)
    assert_valid_json_structure(reference_response, response)
    assert response == 1, f"Expected account  {account_test_tx['to']} to have 1 received transactions"

    # Check v2, ALL
    raw_response = base_request("hmyv2_getTransactionsCount",
                                params=[account_test_tx["to"], "ALL"],
                                endpoint=endpoints[account_test_tx["shardID"]])
    response = check_and_unpack_rpc_response(raw_response, expect_error=False)
    assert_valid_json_structure(reference_response, response)
    assert response == 1, f"Expected account  {account_test_tx['to']} to have 1 received transactions"


def test_get_transactions_history_v1():
    reference_response_full = {
        "transactions": [
            {
                "blockHash": "0x28ddf57c43a3d91069d58be0e5cb8daac04261b97dd34d3c5c361f7bd941e657",
                "blockNumber": "0xf",
                "from": "one1zksj3evekayy90xt4psrz8h6j2v3hla4qwz4ur",
                "timestamp": "0x5f0d84e2",
                "gas": "0x5208",
                "gasPrice": "0x6fc23ac00",
                "hash": "0x3759ab736798efe609bd1eb69145616ccc57baea99a9abab4e259e935f488bc5",
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
        ]
    }

    reference_response_short = {
        "transactions": [
            "0x3759ab736798efe609bd1eb69145616ccc57baea99a9abab4e259e935f488bc5",
        ]
    }

    address = initial_funding[0]["from"]

    # Check short tx
    raw_response = base_request("hmy_getTransactionsHistory",
                                params=[{
                                    "address": address,
                                    "pageIndex": 0,
                                    "pageSize": 1000,
                                    "fullTx": False,
                                    "txType": "ALL",
                                    "order": "ASC"
                                }],
                                endpoint=endpoints[initial_funding[0]["from-shard"]])
    response = check_and_unpack_rpc_response(raw_response, expect_error=False)
    assert_valid_json_structure(reference_response_short, response)

    # Check long tx
    raw_response = base_request("hmy_getTransactionsHistory",
                                params=[{
                                    "address": address,
                                    "pageIndex": 0,
                                    "pageSize": 1000,
                                    "fullTx": True,
                                    "txType": "ALL",
                                    "order": "ASC"
                                }],
                                endpoint=endpoints[initial_funding[0]["from-shard"]])
    response = check_and_unpack_rpc_response(raw_response, expect_error=False)
    assert_valid_json_structure(reference_response_full, response)


def test_get_transactions_history_v2():
    reference_response_full = {
        "transactions": [
            {
                "blockHash": "0x28ddf57c43a3d91069d58be0e5cb8daac04261b97dd34d3c5c361f7bd941e657",
                "blockNumber": 15,
                "from": "one1zksj3evekayy90xt4psrz8h6j2v3hla4qwz4ur",
                "timestamp": 1594721506,
                "gas": 21000,
                "gasPrice": 30000000000,
                "hash": "0x3759ab736798efe609bd1eb69145616ccc57baea99a9abab4e259e935f488bc5",
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
        ]
    }

    reference_response_short = {
        "transactions": [
            "0x3759ab736798efe609bd1eb69145616ccc57baea99a9abab4e259e935f488bc5",
        ]
    }

    address = initial_funding[0]["from"]

    # Check short tx
    raw_response = base_request("hmyv2_getTransactionsHistory",
                                params=[{
                                    "address": address,
                                    "pageIndex": 0,
                                    "pageSize": 1000,
                                    "fullTx": False,
                                    "txType": "ALL",
                                    "order": "ASC"
                                }],
                                endpoint=endpoints[initial_funding[0]["from-shard"]])
    response = check_and_unpack_rpc_response(raw_response, expect_error=False)
    assert_valid_json_structure(reference_response_short, response)

    # Check long tx
    raw_response = base_request("hmyv2_getTransactionsHistory",
                                params=[{
                                    "address": address,
                                    "pageIndex": 0,
                                    "pageSize": 1000,
                                    "fullTx": True,
                                    "txType": "ALL",
                                    "order": "ASC"
                                }],
                                endpoint=endpoints[initial_funding[0]["from-shard"]])
    response = check_and_unpack_rpc_response(raw_response, expect_error=False)
    assert_valid_json_structure(reference_response_full, response)


def test_get_balances_by_block_number_v1(account_test_tx):
    reference_response = "0x3635c9adc5dea00000"

    raw_response = base_request("hmy_getBalanceByBlockNumber",
                                params=[account_test_tx["to"], account_test_tx["blockNumber"]],
                                endpoint=endpoints[account_test_tx["shardID"]])
    response = check_and_unpack_rpc_response(raw_response, expect_error=False)
    assert_valid_json_structure(reference_response, response)
    assert response == account_test_tx[
        "value"], f"Expected balance of {account_test_tx['to']} is {account_test_tx['value']}"


def test_get_balances_by_block_number_v2(account_test_tx):
    reference_response = 1000000000000000000000

    raw_response = base_request("hmyv2_getBalanceByBlockNumber",
                                params=[account_test_tx["to"], int(account_test_tx["blockNumber"], 16)],
                                endpoint=endpoints[account_test_tx["shardID"]])
    response = check_and_unpack_rpc_response(raw_response, expect_error=False)
    assert_valid_json_structure(reference_response, response)
    acc_tx_value = int(account_test_tx["value"], 16)
    assert response == acc_tx_value, f"Expected balance of {account_test_tx['to']} is {acc_tx_value}"
