#!/bin/bash

# this is existing account that generates funds in test/debug.sh
from='one1spshr72utf6rwxseaz339j09ed8p6f8ke370zj'
to='one1zyxauxquys60dk824p532jjdq753pnsenrgmef'

blsKey='be23bc3c93fe14a25f3533feee1cff1c60706845a4907c5df58bc19f5d1760bfff06fe7c9d1f596b18fdf529e0508e0a'

set -eu

# transfer from already imported account (one of the ones that generates tokens)
./hmy-cli transfer --from "${from}" \
	  --from-shard 0 --to-shard 0 \
	  --to "${to}" --amount 5 --wait-for-confirm 30

./hmy-cli balances "${to}"

./hmy-cli staking create-validator --validator-addr "${to}" \
	  --name _Test_key_validator0 --identity test_account \
	  --website harmony.one --security-contact Daniel-VDM \
	  --details none --rate 0.16798352018382678 \
	  --max-rate 0.1791844697821372 \
	  --max-change-rate 0.1522127615232536 \
	  --min-self-delegation 1.1 \
	  --max-total-delegation 13 \
	  --amount 3 \
	  --bls-pubkeys "${blsKey}" \
	  --chain-id testnet --wait-for-confirm 30
