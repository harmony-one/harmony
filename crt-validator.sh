#!/bin/bash

from='one1zksj3evekayy90xt4psrz8h6j2v3hla4qwz4ur'
to='one1d6wp3ddfjx7ea2mga3822xjz2rzurlxz2y68hv'
someRando='one1xqcyfa4vg9khlg6vv85p84g4wa0e0lcjtsv80h'

blsKey="$BLS_KEY"

set -eu

# transfer from already imported account (one of the ones that generates tokens)
printf 'Sent from funded account to external validator\n'

hmy transfer --from "${from}" \
    --from-shard 0 --to-shard 0 \
    --to "${to}" --amount 5 --timeout 30

printf 'Sent from funded account to rando delegator\n'
# transfer for our rando delegator 
hmy transfer --from "${from}" \
    --from-shard 0 --to-shard 0 \
    --to "${someRando}" --amount 5 --timeout 30

printf 'Check balance of our addr for create-validator\n'
hmy balances "${to}"

printf 'Create the actual validator\n'
hmy staking create-validator --validator-addr "${to}" \
    --name _Test_key_validator0 --identity test_account \
    --website harmony.one --security-contact Daniel-VDM \
    --details none --rate 0.16798352018382678 \
    --max-rate 0.1791844697821372 \
    --max-change-rate 0.1522127615232536 \
    --min-self-delegation 1.0 \
    --max-total-delegation 13 \
    --amount 1.1 \
    --bls-pubkeys "${blsKey}" \
    --chain-id testnet --timeout 30

# ./hmy-cli staking undelegate --amount 0.6 \
    # 	  --delegator-addr="${to}" \
    # 	  --validator-addr="${to}" 

printf 'Wait 10 seconds, then delegation from our rando addr to our created-validator\n'
# Need to do a delegator test
sleep 10
hmy staking delegate \
    --delegator-addr "${someRando}" --validator-addr "${to}" \
    --amount 1.5 --timeout 30
