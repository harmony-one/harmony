#!/bin/bash

from='one1zksj3evekayy90xt4psrz8h6j2v3hla4qwz4ur'
to='one14438psd5vrjes7qm97jrj3t0s5l4qff5j5cn4h'
someRando='one1nqevvacj3y5ltuef05my4scwy5wuqteur72jk5'
endpt="http://34.230.39.233:9500"
amt="50"

export BLS_KEY='377c28caf0fd5ef5a914109c618245804b8c3336e131e4f9e4129fbed9a51ec6ba8ed89fffd1776ac530a2f50d888613'

export BLS_KEY_PATH='/home/edgar/go/src/github.com/harmony-one/harmony/377c28caf0fd5ef5a914109c618245804b8c3336e131e4f9e4129fbed9a51ec6ba8ed89fffd1776ac530a2f50d888613.key'

export BOOTNODE_PATH='/ip4/52.40.84.2/tcp/9876/p2p/QmbPVwrqWsTYXq1RxGWcxx9SWaTUCfoo1wA6wmdbduWe29'

set -eu

# transfer from already imported account (one of the ones that generates tokens)
printf 'Sent %s from funded account to external validator\n' $amt

hmy transfer --from "${from}" \
    --from-shard 0 --to-shard 0 \
    --to "${to}" --amount $amt \
    --timeout 30 -n "${endpt}"

printf 'Sent %s from funded account to rando delegator\n' $amt
# transfer for our rando delegator 
hmy transfer --from "${from}" \
    --from-shard 0 --to-shard 0 \
    --to "${someRando}" --amount 50 \
    --timeout 30 --node "${endpt}"

printf 'Check balance of our addr for create-validator\n'
hmy balances "${to}" -n "${endpt}"

printf 'Create the actual validator\n'
hmy staking create-validator --validator-addr "${to}" \
    --name _Test_key_validator0 --identity test_account \
    --website harmony.one --security-contact Edgar-VDM \
    --details none --rate 0.16798352018382678 \
    --max-rate 0.1791844697821372 \
    --max-change-rate 0.1522127615232536 \
    --min-self-delegation 1.0 \
    --max-total-delegation 13 \
    --amount 6.6 \
    --bls-pubkeys "${BLS_KEY}" \
    --chain-id testnet --timeout 30 \
    --node "${endpt}" || true


printf 'Wait 10 seconds, then delegation from our rando addr to our created-validator\n'
# Need to do a delegator test
sleep 10
hmy staking delegate \
    --delegator-addr "${someRando}" --validator-addr "${to}" \
    --amount 1.5 --timeout 30 -n "${endpt}" || true

cp scripts/node.sh bin

cd bin

touch blspass

sudo ./node.sh -p blspass -N slashing -z -D
