#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
Stores all the transaction information used in the test suite.

INVARIANT: Each account only sends 1 plain transaction (per shard) except for initial transaction(s).
"""
import functools
import json
import time
import random

from pyhmy import (
    account,
    blockchain
)
from pyhmy.rpc.request import (
    base_request
)

from utils import (
    check_and_unpack_rpc_response,
    is_valid_json_rpc
)

tx_timeout = 50  # In seconds
beacon_shard_id = 0
_is_cross_shard_era = False
_is_staking_era = False

# Endpoints sorted by shard
endpoints = [
    "http://localhost:9598/",  # shard 0
    "http://localhost:9596/",  # shard 1
]

# ORDER MATERS: tx n cannot be sent without tx n-1 being sent first due to nonce
# Only exception on invariant.
initial_funding = [
    {
        # Used by: `account_test_tx`
        "from": "one1zksj3evekayy90xt4psrz8h6j2v3hla4qwz4ur",
        "to": "one1v92y4v2x4q27vzydf8zq62zu9g0jl6z0lx2c8q",
        # scissors matter runway reduce flush illegal ancient absurd scare young copper ticket direct wise person hobby tomato chest edge cost wine crucial vendor elevator
        "amount": "100000",
        "from-shard": 0,
        "to-shard": 0,
        "hash": "0x4553da3a01770e4048862c39dd8f2996eacf990cf40932a358405239fe3650fc",
        "nonce": "0x0",
        "signed-raw-tx": "0xf870808506fc23ac0082520880809461544ab146a815e6088d49c40d285c2a1f2fe84f8a152d02c7e14af68000008028a0dd11189f8c54cf363145d1c877b49748e4c98f5a799cb712b76adac3966651f0a05a764e87c0f056bace46d1f410bea6c9267dbfe713abaa4b3bd05389456dd3c0",
    },
    {
        # Used by: `cross_shard_txs`
        "from": "one1zksj3evekayy90xt4psrz8h6j2v3hla4qwz4ur",
        "to": "one1ue25q6jk0xk3dth4pxur9e742vcqfwulhwqh45",
        # obey scissors fiscal hood chaos grit all piano armed change general attract balcony hair cat outside hour quiz unhappy tattoo awful offer toddler invest
        "amount": "100000",
        "from-shard": 0,
        "to-shard": 0,
        "hash": "0x5dbef17541f1c375c692104419821aefeba47e62b1a28044abfef7f4a467cdd4",
        "nonce": "0x1",
        "signed-raw-tx": "0xf870018506fc23ac00825208808094e655406a5679ad16aef509b832e7d5533004bb9f8a152d02c7e14af68000008027a0298a98ab57c51f5544632f283e452d1158b4565c6d3cd8ad667ec6ea6a4de0b7a046ea90929a047862deda016845bce6711e8e82f59133230fa511024283b74916",
    },
    {
        # Used by: `test_get_pending_cx_receipts`
        "from": "one1zksj3evekayy90xt4psrz8h6j2v3hla4qwz4ur",
        "to": "one19l4hghvh40fyldxfznn0a3ss7d5gk0dmytdql4",
        # judge damage safe field faculty piece salon gentle riot unfair symptom sun exclude agree fantasy fossil catalog tool bounce tomorrow churn join very number
        "amount": "100000",
        "from-shard": 0,
        "to-shard": 0,
        "hash": "0x3d93962349fcd0e57fdfd17e94550dbd15ba75e8f2c1151a41b5535936e49abb",
        "nonce": "0x2",
        "signed-raw-tx": "0xf870028506fc23ac008252088080942feb745d97abd24fb4c914e6fec610f3688b3dbb8a152d02c7e14af68000008028a0084e402becab9df1ad57a8ff508dc6b64601839d385d6993ebc92e06f364184fa01500d6dbb3add350b89ff3957869af85eafa3bf29527b31f5ebe82db601c2d7e",
    },
    {
        # Used by: `test_pending_transactions_v1`
        "from": "one1zksj3evekayy90xt4psrz8h6j2v3hla4qwz4ur",
        "to": "one1twhzfc2wr4j5ka7gs9pmllpnrdyaskcl5lq8ye",
        # science swim absent horse gas wink switch section soup pair chuckle rug paddle lottery message veteran poverty alone current prize spoil dune super crumble
        "amount": "100000",
        "from-shard": 0,
        "to-shard": 0,
        "hash": "0x6e5125487d2b41024aa8d8e9b37a85e1f8acb48aa422cc849477cbc3142db2ff",
        "nonce": "0x3",
        "signed-raw-tx": "0xf870038506fc23ac008252088080945bae24e14e1d654b77c88143bffc331b49d85b1f8a152d02c7e14af68000008028a0d665a57bb8783ac9830b870726315e49faea916aa1ca66b4725db8fef6c06592a077717c9ecad6270917f90513b4673815cc6e84774f04adac84369d7821658e3b",
    },
    {
        # Used by: `test_pending_transactions_v2`
        "from": "one1zksj3evekayy90xt4psrz8h6j2v3hla4qwz4ur",
        "to": "one1u57rlv5q82deja6ew2l9hdy7ag3dwnw57x8s9t",
        # noble must all evoke core grass goose describe latin left because awful gossip tuna broccoli tomorrow piece enable theme comic below avoid dove high
        "amount": "100000",
        "from-shard": 0,
        "to-shard": 0,
        "hash": "0x7b5f795ca3ffbc05c6b79b52158f1dfafe8821e2f30f4a1939f22d6e347aadbb",
        "nonce": "0x4",
        "signed-raw-tx": "0xf870048506fc23ac00825208808094e53c3fb2803a9b99775972be5bb49eea22d74dd48a152d02c7e14af68000008027a044e33249ea5bcce6ac423bde73b78460df1cd91a1ce23c3396a987430edaa09fa0527b2650234543300d71190516481331a7a07c31fdebf039e4c439d145671cf2",
    },
    {
        # Used by: `test_send_raw_transaction_v1`
        "from": "one1zksj3evekayy90xt4psrz8h6j2v3hla4qwz4ur",
        "to": "one1pvkjamc0q96s6z62qzz6e09k2qrqqdj34ylxvd",
        # satisfy spend chaos twice sort obvious mercy prize slow divert addict section love inflict claim turn elbow pet flock cigar social spoil turn ensure
        "amount": "100000",
        "from-shard": 0,
        "to-shard": 0,
        "hash": "0x7ed85da44ed014ea73f1196985643b92ce2577488a5910a92e209c543ead82f8",
        "nonce": "0x5",
        "signed-raw-tx": "0xf870058506fc23ac008252088080940b2d2eef0f01750d0b4a0085acbcb650060036518a152d02c7e14af68000008027a055c5b550d01e0ce9ba338f8d9cf190dd813ac7a8184e332e10eb1ffce8973243a030bf6f289ba3fa13415d0bd601abdb94630986efb59f9ce19d6c0a1e49d11502",
    },
    {
        # Used by: `test_send_raw_transaction_v2`
        "from": "one1zksj3evekayy90xt4psrz8h6j2v3hla4qwz4ur",
        "to": "one13lu674f3jkfk2qhsngfc2vhcf372wprctdjvgu",
        # organ truly miss sell visual pulse maid element slab sugar bullet absorb digital space dance long another man cherry fruit effort pluck august flag
        "amount": "100000",
        "from-shard": 0,
        "to-shard": 0,
        "hash": "0xeaaedd017b3e2efedbe7d05f38a9ee5d7950547f73c997b08e5b2e6a694b1285",
        "nonce": "0x6",
        "signed-raw-tx": "0xf870068506fc23ac008252088080948ff9af553195936502f09a138532f84c7ca704788a152d02c7e14af68000008028a068a8211bdb8ae6ea7f85f140b37cfaa4e0a76e634bee673ac6e35d91b66700bea0795136404d542a1e40d16a924caaf1faee6243736b21f1ea7b4e3d6fb350f472",
    },
    {
        # Used by: `test_get_current_transaction_error_sink`
        "from": "one1zksj3evekayy90xt4psrz8h6j2v3hla4qwz4ur",
        "to": "one1ujsjs4mhds75xnws0yx0v8l2rvyp67arwzqrvz",
        # video mind cash involve kitten mobile multiply shine foam citizen minimum busy slab keen under food swamp fortune dumb slice beyond piano forest call
        "amount": "100000",
        "from-shard": 0,
        "to-shard": 0,
        "hash": "0x242ffd4bfbd34857dc36c59b7370bd9b50d7aef07a29555bc99483ac7feb378d",
        "nonce": "0x7",
        "signed-raw-tx": "0xf870078506fc23ac00825208808094e4a12857776c3d434dd0790cf61fea1b081d7ba38a152d02c7e14af68000008028a0f09f48a6b86b0966de74f5ac242ab727332d9fad99ef0403fb22547770c3f3e5a078885614410f66af51b306090cfce7deeed59363dd40f758b5592e6a74f642a1",
    },
    {
        # Used by: `deployed_contract`
        "from": "one1zksj3evekayy90xt4psrz8h6j2v3hla4qwz4ur",
        "to": "one156wkx832t0nxnaq6hxawy4c3udmnpzzddds60a",
        # dove turkey fitness brush drip page senior lemon other climb govern fantasy entry reflect when biology hunt victory turkey volcano casino movie shed valve
        "amount": "100000",
        "from-shard": 0,
        "to-shard": 0,
        "hash": "0x078fa8486683e5c04e820dde99c8280af0848788df313ee20cc1e474bdcac821",
        "nonce": "0x8",
        "signed-raw-tx": "0xf870088506fc23ac00825208808094a69d631e2a5be669f41ab9bae25711e37730884d8a152d02c7e14af68000008027a01af975303e964b88bf2158a8fdc00fee0244322a301d33ef3117798979f9c255a01a14d5aff8b9e2f4eaff1ee3761467e35e55ab10993118ab6454a8b1235be413",
    },
    {
        # Used by: `s0_validator`
        "from": "one1zksj3evekayy90xt4psrz8h6j2v3hla4qwz4ur",
        "to": "one109r0tns7av5sjew7a7fkekg4fs3pw0h76pp45e",
        # proud guide else desk renew leave fix post fat angle throw gain field approve year umbrella era axis horn unlock trip guide replace accident
        "amount": "100000",
        "from-shard": 0,
        "to-shard": 0,
        "hash": "0x4a069bf42191da42f2955f045e4c304904073476c7bf8f7d1cf6bf7b9a2eb995",
        "nonce": "0x9",
        "signed-raw-tx": "0xf870098506fc23ac008252088080947946f5ce1eeb290965deef936cd9154c22173efe8a152d02c7e14af68000008028a0aa679ee9c7bc4741d55c9724f248f23d0a5a79c9f5bf960c6a07a3436d1234f4a063771c21dab504eb15f44efcfa97090ba072171e71b6df77a35ea84bc2b2302f",
    },
    {
        # Used by: `s1_validator`
        "from": "one1zksj3evekayy90xt4psrz8h6j2v3hla4qwz4ur",
        "to": "one1nmy8quw0924fss4r9km640pldzqegjk4wv4wts",
        # aisle aware spatial sausage vibrant tennis useful admit junior light calm wear caution snack seven spoon yellow crater giraffe mirror spare educate result album
        "amount": "100000",
        "from-shard": 0,
        "to-shard": 0,
        "hash": "0x3ddcd1524386b985ed6ec4f0e02e6fcc4ad5180d1fffaa847c7ff95fd0c82029",
        "nonce": "0xa",
        "signed-raw-tx": "0xf8700a8506fc23ac008252088080949ec87071cf2aaa9842a32db7aabc3f6881944ad58a152d02c7e14af68000008027a075ffd26368005e0140659406472e875e22f3cef76f715a2e61d8fe3fd06de5d0a0407b1e42f8c876021a686399dcb1ac10baea0c87bd341463e95e8625cab559a9",
    },
    {
        # Used by: `test_delegation` & `test_undelegation`
        "from": "one1zksj3evekayy90xt4psrz8h6j2v3hla4qwz4ur",
        "to": "one1v895jcvudcktswcmg2sldvmxvtvvdj2wuxj3hx",
        # web topple now acid repeat inspire tomato inside nominee reflect latin salmon garbage negative liberty win royal faith hammer lawsuit west toddler payment coffee
        "amount": "100000",
        "from-shard": 0,
        "to-shard": 0,
        "hash": "0x48e942756c73070587af4d0b5191ca0b75438ea11da0157f950777cc1c711e03",
        "nonce": "0xb",
        "signed-raw-tx": "0xf8700b8506fc23ac0082520880809461cb49619c6e2cb83b1b42a1f6b36662d8c6c94e8a152d02c7e14af68000008027a031b487574472555b53591ee048d6036cf0cce14ebff7d5d1376371b337024d61a04dcc0298761d73c93a655954897b59e2ce2e78c35859c4fe24abdb6304ddf83a",
    },
    {
        # Used by: `test_pending_staking_transactions_v1`
        "from": "one1zksj3evekayy90xt4psrz8h6j2v3hla4qwz4ur",
        "to": "one13v9m45m6yk9qmmcgyq603ucy0wdw9lfsxzsj9d",
        # grief comfort prefer wealth foam consider kingdom secret comfort brush kit cereal hello ripple choose follow mammal swap city pistol drip unfair glass jacket
        "amount": "100000",
        "from-shard": 0,
        "to-shard": 0,
        "hash": "0x789a25e28c73dafcc5105205a629c297730d4da2cf95a5437d349d72d036d1e5",
        "nonce": "0xc",
        "signed-raw-tx": "0xf8700c8506fc23ac008252088080948b0bbad37a258a0def082034f8f3047b9ae2fd308a152d02c7e14af68000008027a04d8d18da12f99299421a10dd5b610e6fbd1247f5f3e5f20f0778ac1fde664f02a05835c5ca12d0629553783356663617de7686b11e3614bf117d308213149c5b31",
    },
    {
        # Used by: `test_pending_staking_transactions_v2`
        "from": "one1zksj3evekayy90xt4psrz8h6j2v3hla4qwz4ur",
        "to": "one13muqj27fcd59gfrv7wzvuaupgkkwvwzlxun0ce",
        # suit gate simple ship chicken labor twenty attend knee click quit emerge minimum veteran need group verify dish baby argue guard win tip swear
        "amount": "100000",
        "from-shard": 0,
        "to-shard": 0,
        "hash": "0x9971b136f9678f456e97ce0db8d912208b7d811f3b76a5cc52c88d444f005ff6",
        "nonce": "0xd",
        "signed-raw-tx": "0xf8700d8506fc23ac008252088080948ef8092bc9c36854246cf384ce778145ace6385f8a152d02c7e14af68000008028a062b0acdaebb337d29b2fea7fdc6da31a13011c011405a975aa1220c84757635ea03e1ea73564a792ce05717ac783bbb9fc593e077b8e6bbe8e82b661817dd3e99d",
    },
]


def is_cross_shard_era():
    """
    Returns if the network is in cross shard tx era...
    """
    global _is_cross_shard_era
    if _is_cross_shard_era:
        return True
    time.sleep(random.uniform(0.5, 1.5))  # Random to stop burst spam of RPC calls.
    if all(blockchain.get_current_epoch(e) >= 1 for e in endpoints):
        _is_cross_shard_era = True
        return True
    return False


def cross_shard(fn):
    """
    Decorator for tests that requires a cross shard transaction
    """

    @functools.wraps(fn)
    def wrap(*args, **kwargs):
        while not is_cross_shard_era():
            pass
        return fn(*args, **kwargs)

    return wrap


def is_staking_era():
    """
    Returns if the network is in staking era...
    """
    global _is_staking_era
    if _is_staking_era:
        return True
    time.sleep(random.uniform(0.5, 1.5))  # Random to stop burst spam of RPC calls.
    threshold_epoch = blockchain.get_prestaking_epoch(endpoints[beacon_shard_id])
    if all(blockchain.get_current_epoch(e) >= threshold_epoch for e in endpoints):
        _is_staking_era = True
    return False


def staking(fn):
    """
    Decorator for tests that requires staking epoch
    """

    @functools.wraps(fn)
    def wrap(*args, **kwargs):
        while not is_staking_era():
            pass
        return fn(*args, **kwargs)

    return wrap


def send_transaction(tx_data, confirm_submission=False):
    """
    Send the given transaction (`tx_data`), and check that it got submitted
    to tx pool if `confirm_submission` is enabled.

    Node that tx_data follow the format of one of the entries in `initial_funding`
    """
    assert isinstance(tx_data, dict), f"Sanity check: expected tx_data to be of type dict not {type(tx_data)}"
    for el in ["from", "from-shard", "signed-raw-tx", "hash"]:
        assert el in tx_data.keys(), f"Expected {el} as a key in {json.dumps(tx_data, indent=2)}"

    # Validate tx sender
    assert_valid_test_from_address(tx_data["from"], tx_data["from-shard"], is_staking=False)

    # Send tx
    response = base_request('hmy_sendRawTransaction', params=[tx_data["signed-raw-tx"]],
                            endpoint=endpoints[tx_data["from-shard"]])
    if confirm_submission:
        tx_hash = check_and_unpack_rpc_response(response, expect_error=False)
        assert tx_hash == tx_data["hash"], f"Expected submitted transaction to get tx hash of {tx_data['hash']}, " \
                                           f"got {tx_hash}"
    else:
        assert is_valid_json_rpc(response), f"Invalid JSON response: {response}"


def send_staking_transaction(tx_data, confirm_submission=False):
    """
    Send the given staking transaction (`tx_data`), and check that it got submitted
    to tx pool if `confirm_submission` is enabled.

    Node that tx_data follow the format of one of the entries in `initial_funding`
    """
    assert isinstance(tx_data, dict), f"Sanity check: expected tx_data to be of type dict not {type(tx_data)}"
    for el in ["signed-raw-tx", "hash"]:
        assert el in tx_data.keys(), f"Expected {el} as a key in {json.dumps(tx_data, indent=2)}"

    # Send tx
    response = base_request('hmy_sendRawStakingTransaction', params=[tx_data["signed-raw-tx"]],
                            endpoint=endpoints[0])
    if confirm_submission:
        tx_hash = check_and_unpack_rpc_response(response, expect_error=False)
        assert tx_hash == tx_data["hash"], f"Expected submitted staking transaction to get tx hash " \
                                           f"of {tx_data['hash']}, got {tx_hash}"
    else:
        assert is_valid_json_rpc(response), f"Invalid JSON response: {response}"


def send_and_confirm_transaction(tx_data, timeout=tx_timeout):
    """
    Send and confirm the given transaction (`tx_data`) within the given `timeout`.

    Node that tx_data follow the format of one of the entries in `initial_funding`.

    Note that errored tx submission will not return an error early, instead, failed transactions will be
    caught by timeout. This is done because it is possible to submit the same transaction multiple times,
    thus causing the RPC to return an error, causing unwanted errors in tests that are ran in parallel.
    """
    assert isinstance(tx_data, dict), f"Sanity check: expected tx_data to be of type dict not {type(tx_data)}"
    for el in ["from-shard", "hash"]:
        assert el in tx_data.keys(), f"Expected {el} as a key in {json.dumps(tx_data, indent=2)}"

    send_transaction(tx_data, confirm_submission=False)
    # Do not check for errors since resending initial txs is fine & failed txs will be caught in confirm timeout.

    # Confirm tx within timeout window
    start_time = time.time()
    while time.time() - start_time <= timeout:
        tx_response = get_transaction(tx_data["hash"], tx_data["from-shard"])
        if tx_response is not None:
            if tx_response['blockNumber'] is not None:
                return tx_response
        time.sleep(random.uniform(0.2, 0.5))  # Random to stop burst spam of RPC calls.
    raise AssertionError("Could not confirm transactions on-chain.")


def send_and_confirm_staking_transaction(tx_data, timeout=tx_timeout * 2):
    """
    Send and confirm the given staking transaction (`tx_data`) within the given `timeout`.

    Node that tx_data follow the format of one of the entries in `initial_funding`.

    Note that errored tx submission will not return an error early, instead, failed transactions will be
    caught by timeout. This is done because it is possible to submit the same transaction multiple times,
    thus causing the RPC to return an error, causing unwanted errors in tests that are ran in parallel.
    """
    assert isinstance(tx_data, dict), f"Sanity check: expected tx_data to be of type dict not {type(tx_data)}"
    for el in ["hash"]:
        assert el in tx_data.keys(), f"Expected {el} as a key in {json.dumps(tx_data, indent=2)}"

    send_staking_transaction(tx_data, confirm_submission=False)
    # Do not check for errors since resending initial txs is fine & failed txs will be caught in confirm timeout.

    # Confirm tx within timeout window
    start_time = time.time()
    while time.time() - start_time <= timeout:
        tx_response = get_staking_transaction(tx_data["hash"])
        if tx_response is not None:
            if tx_response['blockNumber'] is not None:
                return tx_response
        time.sleep(random.uniform(0.2, 0.5))  # Random to stop burst spam of RPC calls.
    raise AssertionError("Could not confirm staking transaction on-chain.")


def get_transaction(tx_hash, shard):
    """
    Fetch the transaction for the given hash on the given shard.
    It also checks that the RPC response is valid.
    """
    assert isinstance(tx_hash, str), f"Sanity check: expect tx hash to be of type str not {type(tx_hash)}"
    assert isinstance(shard, int), f"Sanity check: expect shard to be of type int not {type(shard)}"
    raw_response = base_request('hmy_getTransactionByHash', params=[tx_hash], endpoint=endpoints[shard])
    return check_and_unpack_rpc_response(raw_response, expect_error=False)


def get_staking_transaction(tx_hash):
    """
    Fetch the staking transaction for the given hash on the given shard.
    It also checks that the RPC response is valid.
    """
    assert isinstance(tx_hash, str), f"Sanity check: expect tx hash to be of type str not {type(tx_hash)}"
    raw_response = base_request('hmy_getStakingTransactionByHash', params=[tx_hash], endpoint=endpoints[0])
    return check_and_unpack_rpc_response(raw_response, expect_error=False)


def assert_valid_test_from_address(address, shard, is_staking=False):
    """
    Asserts that the given address is a valid 'from' address for a test transaction.

    Note that this considers the invariant for transactions.
    """
    assert isinstance(address, str), f"Sanity check: Expect address {address} as a string."
    assert isinstance(shard, int), f"Sanity check: Expect shard {shard} as am int."
    assert isinstance(is_staking, bool), f"Sanity check: Expect is_staking {is_staking} as a bool."
    assert account.is_valid_address(address), f"{address} is an invalid ONE address"
    if not account.get_balance(address, endpoint=endpoints[shard]) >= 1e18:
        raise AssertionError(f"Account {address} does not have at least 1 ONE on shard {shard}")
    if not is_staking and account.get_transaction_count(address, endpoint=endpoints[shard]) != 0:
        raise AssertionError(f"Account {address} has already sent a transaction, breaking the txs invariant")
