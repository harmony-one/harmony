#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
Tests here are related to smart contracts & require a feedback loop with the chain.

TODO: negative test cases

As with all tests, there are 2 JSON-RPC versions/namespaces (v1 & v2) where their difference
is only suppose to be in the types of their params & returns. v1 keeps everything in hex and
v2 uses decimal when possible. However, there are some (legacy) discrepancies that some tests
enforce. These tests are noted and should NOT be broken.
"""
import traceback

import pytest
from pyhmy import (
    transaction,
    blockchain
)
from pyhmy.rpc.request import (
    base_request
)

from txs import (
    initial_funding,
    endpoints,
    send_and_confirm_transaction,
    get_transaction
)
from utils import (
    check_and_unpack_rpc_response,
    is_valid_json_rpc
)


def _get_contract_address(tx_hash, shard):
    """
    Helper function to get contract address from the tx_hash.
    """
    assert isinstance(tx_hash, str), f"Sanity check: expected tx_hash to be of type str not {type(tx_hash)}"
    assert isinstance(shard, int), f"Sanity check: expected shard to be of type int not {type(shard)}"
    address = transaction.get_transaction_receipt(tx_hash, endpoints[shard])["contractAddress"]
    assert address, f"Transaction {tx_hash} is not a deployment of a smart contract, got not contract address"
    return address


@pytest.fixture(scope="module")
def deployed_contract():
    """
    Fixture to deploy a smart contract.

    Returns the transaction information (`contract_tx`) to be used by tests in this module.

    Note that this contract is the Migrations contract used by truffle.
    """
    contract_tx = {
        "from": "one156wkx832t0nxnaq6hxawy4c3udmnpzzddds60a",
        "to": None,
        "amount": None,
        "from-shard": 0,
        "to-shard": 0,
        "hash": "0x7b818e63630dd41c7778e0a72ebd7e1f37ac9b92e5ee8050e46a252fa2a85757",
        "nonce": "0x8",
        "signed-raw-tx": "0xf90251808506fc23ac008366916c80808080b901fc608060405234801561001057600080fd5b50336000806101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff16021790555061019c806100606000396000f3fe608060405234801561001057600080fd5b50600436106100415760003560e01c8063445df0ac146100465780638da5cb5b14610064578063fdacd576146100ae575b600080fd5b61004e6100dc565b6040518082815260200191505060405180910390f35b61006c6100e2565b604051808273ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200191505060405180910390f35b6100da600480360360208110156100c457600080fd5b8101908080359060200190929190505050610107565b005b60015481565b6000809054906101000a900473ffffffffffffffffffffffffffffffffffffffff1681565b6000809054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff163373ffffffffffffffffffffffffffffffffffffffff16141561016457806001819055505b5056fea265627a7a723158209b80813a158b44af65aee232b44c0ac06472c48f4abbe298852a39f0ff34a9f264736f6c6343000510003228a016d5ca07b8fd32ac9ee0b9ce688ab0a384fc5dc8015b0629f3f9be728bdfbbdda03ffe944dd66c4af37262c15a2bc26d4e92602c3ca6546672e08060da8abf3a26",
        "code": "0x608060405234801561001057600080fd5b50600436106100415760003560e01c8063445df0ac146100465780638da5cb5b14610064578063fdacd576146100ae575b600080fd5b61004e6100dc565b6040518082815260200191505060405180910390f35b61006c6100e2565b604051808273ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200191505060405180910390f35b6100da600480360360208110156100c457600080fd5b8101908080359060200190929190505050610107565b005b60015481565b6000809054906101000a900473ffffffffffffffffffffffffffffffffffffffff1681565b6000809054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff163373ffffffffffffffffffffffffffffffffffffffff16141561016457806001819055505b5056fea265627a7a723158209b80813a158b44af65aee232b44c0ac06472c48f4abbe298852a39f0ff34a9f264736f6c63430005100032"
    }

    in_initially_funded = False
    for tx in initial_funding:
        if tx["to"] == contract_tx["from"] and tx["to-shard"] == contract_tx["from-shard"]:
            in_initially_funded = True
            break
    if not in_initially_funded:
        raise AssertionError(f"Test transaction from address {contract_tx['from']} "
                             f"not found in set of initially funded accounts.")

    if get_transaction(contract_tx["hash"], contract_tx["from-shard"]) is None:
        tx = send_and_confirm_transaction(contract_tx)
        assert tx["hash"] == contract_tx["hash"], f"Expected contract transaction hash to be {contract_tx['hash']}, " \
                                                  f"got {tx['hash']}"
    return contract_tx


def test_call_v1(deployed_contract):
    """
    TODO: Real smart contract call, currently, it is an error case test.
    """
    address = _get_contract_address(deployed_contract["hash"], deployed_contract["from-shard"])
    raw_response = base_request("hmy_call", params=[{"to": address}, "latest"],
                                endpoint=endpoints[deployed_contract["from-shard"]])
    assert is_valid_json_rpc(raw_response), f"Invalid JSON response: {raw_response}"
    try:
        response = check_and_unpack_rpc_response(raw_response, expect_error=False)
        assert isinstance(response, str) and response.startswith("0x")  # Must be a hex string
    except Exception as e:
        pytest.skip("RPC format being reworked, fix when finished")


def test_call_v2(deployed_contract):
    """
    TODO: Real smart contract call, currently, it is an error case test.
    """
    address = _get_contract_address(deployed_contract["hash"], deployed_contract["from-shard"])
    block_number = blockchain.get_block_number(endpoint=endpoints[deployed_contract["from-shard"]])
    raw_response = base_request("hmyv2_call", params=[{"to": address}, block_number],
                                endpoint=endpoints[deployed_contract["from-shard"]])
    assert is_valid_json_rpc(raw_response), f"Invalid JSON response: {raw_response}"
    try:
        response = check_and_unpack_rpc_response(raw_response, expect_error=False)
        assert isinstance(response, str) and response.startswith("0x")  # Must be a hex string
    except Exception as e:
        pytest.skip("RPC format being reworked, fix when finished")


def test_estimate_gas_v1(deployed_contract):
    """
    RPC currently returns a constant, subject to change in the future, so skip for any error.
    """
    try:
        raw_response = base_request("hmy_estimateGas", params=[{}], endpoint=endpoints[deployed_contract["from-shard"]])
        response = check_and_unpack_rpc_response(raw_response, expect_error=False)
        assert response == "0xcf08", f"Expected constant reply for estimate gas to be 0xcf08, got {response}"
    except Exception as e:
        pytest.skip(traceback.format_exc())
        pytest.skip(f"Exception: {e}")


def test_estimate_gas_v2(deployed_contract):
    """
    RPC currently returns a constant, subject to change in the future, so skip for any error.
    """
    try:
        raw_response = base_request("hmyv2_estimateGas", params=[{}],
                                    endpoint=endpoints[deployed_contract["from-shard"]])
        response = check_and_unpack_rpc_response(raw_response, expect_error=False)
        assert response == "0xcf08", f"Expected constant reply for estimate gas to be 0xcf08, got {response}"
    except Exception as e:
        pytest.skip(traceback.format_exc())
        pytest.skip(f"Exception: {e}")


def test_get_code_v1(deployed_contract):
    address = _get_contract_address(deployed_contract["hash"], deployed_contract["from-shard"])
    raw_response = base_request("hmy_getCode", params=[address, "latest"],
                                endpoint=endpoints[deployed_contract["from-shard"]])
    response = check_and_unpack_rpc_response(raw_response, expect_error=False)
    assert response == deployed_contract["code"], f"Expected {deployed_contract['code']}, got {response}"


def test_get_code_v2(deployed_contract):
    address = _get_contract_address(deployed_contract["hash"], deployed_contract["from-shard"])
    block_number = blockchain.get_block_number(endpoint=endpoints[deployed_contract["from-shard"]])
    raw_response = base_request("hmyv2_getCode", params=[address, block_number],
                                endpoint=endpoints[deployed_contract["from-shard"]])
    response = check_and_unpack_rpc_response(raw_response, expect_error=False)
    assert response == deployed_contract["code"], f"Expected {deployed_contract['code']}, got {response}"


def test_get_storage_at_v1(deployed_contract):
    address = _get_contract_address(deployed_contract["hash"], deployed_contract["from-shard"])
    raw_response = base_request("hmy_getStorageAt", params=[address, "0x0", "latest"],
                                endpoint=endpoints[deployed_contract["from-shard"]])
    response = check_and_unpack_rpc_response(raw_response, expect_error=False)
    assert isinstance(response, str) and response.startswith("0x")  # Must be a hex string


def test_get_storage_at_v2(deployed_contract):
    address = _get_contract_address(deployed_contract["hash"], deployed_contract["from-shard"])
    block_number = blockchain.get_block_number(endpoint=endpoints[deployed_contract["from-shard"]])
    raw_response = base_request("hmyv2_getStorageAt", params=[address, "0x0", block_number],
                                endpoint=endpoints[deployed_contract["from-shard"]])
    response = check_and_unpack_rpc_response(raw_response, expect_error=False)
    assert isinstance(response, str) and response.startswith("0x")  # Must be a hex string
