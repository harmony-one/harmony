#!/bin/bash

from='one1zksj3evekayy90xt4psrz8h6j2v3hla4qwz4ur'
to='one1y5gmmzumajkm5mx3g2qsxtza2d3haq0zxyg47r'
someRando='one1qrqcfek6sc29sachs3glhs4zny72mlad76lqcp'

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
    --website harmony.one --security-contact Edgar-VDM \
    --details none --rate 0.16798352018382678 \
    --max-rate 0.1791844697821372 \
    --max-change-rate 0.1522127615232536 \
    --min-self-delegation 1.0 \
    --max-total-delegation 13 \
    --amount 0.6 \
    --bls-pubkeys "${blsKey}" \
    --chain-id testnet --timeout 30


printf 'Wait 10 seconds, then delegation from our rando addr to our created-validator\n'
# Need to do a delegator test
sleep 10
hmy staking delegate \
    --delegator-addr "${someRando}" --validator-addr "${to}" \
    --amount 1.5 --timeout 30
