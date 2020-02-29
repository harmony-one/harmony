#!/bin/bash

faucet='one1zksj3evekayy90xt4psrz8h6j2v3hla4qwz4ur'
someRando='one1nqevvacj3y5ltuef05my4scwy5wuqteur72jk5'

endpt="http://54.158.65.105:9500"
amt="50"

to0='one1zyxauxquys60dk824p532jjdq753pnsenrgmef'
to2='one14438psd5vrjes7qm97jrj3t0s5l4qff5j5cn4h'

key0='790f87868d56594bff73320b50a2b9b9818ed30780a2aeacea6ec5e6c098e6ad073d61c73946d3855a9498cee8eca200'
key2='67336532c04545afc5c1c979f58b5c301af406eaa0f4c900dcd3004189936c7213ee126d9591026f65248e5f25278f02'

set -eu

# transfer from already imported account (one of the ones that generates tokens)
printf 'Sent %s from funded account to external validators\n' $amt

hmy transfer --from $faucet \
    --from-shard 0 --to-shard 0 \
    --to $to0 --amount $amt \
    --timeout 30 -n "${endpt}"

hmy transfer --from $faucet \
    --from-shard 0 --to-shard 0 \
    --to $to2 --amount $amt \
    --timeout 30 -n "${endpt}"

printf 'Sent %s from funded account to rando delegator\n' $amt
# transfer for our rando delegator 
hmy transfer --from $faucet \
    --from-shard 0 --to-shard 0 \
    --to "${someRando}" --amount 50 \
    --timeout 30 --node "${endpt}"

printf 'Check balance of our addr for create-validator\n'
hmy balances $to0 -n "${endpt}"

hmy balances $to2 -n "${endpt}"

printf 'Create the actual validator\n'
hmy staking create-validator --validator-addr $to0 \
    --name _Test_key_validator1 --identity test_account1 \
    --website harmony.one --security-contact Edgar-VDM \
    --details none --rate 0.16798352018382678 \
    --max-rate 0.1791844697821372 \
    --max-change-rate 0.1522127615232536 \
    --min-self-delegation 1.0 \
    --max-total-delegation 13 \
    --amount 6.6 \
    --bls-pubkeys $key0 \
    --chain-id testnet --timeout 30 \
    --node "${endpt}" || true

hmy staking create-validator --validator-addr $to2 \
    --name _Test_key_validator2 --identity test_account2 \
    --website harmony.one --security-contact Edgar-VDM \
    --details none --rate 0.12798352018382678 \
    --max-rate 0.1791844697821372 \
    --max-change-rate 0.0522127615232536 \
    --min-self-delegation 3.2 \
    --max-total-delegation 80 \
    --amount 34.6 \
    --bls-pubkeys $key2 \
    --chain-id testnet --timeout 30 \
    --node "${endpt}" || true

printf 'Wait 10 seconds, then delegation from our rando addr to our created-validators\n'
# Need to do a delegator test
sleep 10

hmy staking delegate \
    --delegator-addr "${someRando}" --validator-addr $to0 \
    --amount 1.5 --timeout 30 -n "${endpt}" || true

hmy staking delegate \
    --delegator-addr "${someRando}" --validator-addr $to2 \
    --amount 1.5 --timeout 30 -n "${endpt}" || true

printf 'created-validators %s %s\n' $to0 $to2
