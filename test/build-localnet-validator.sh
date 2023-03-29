#! /bin/bash

echo "Make sure the validator account are imported"
hmy keys import-ks .hmy/extkeystore/one17ughrllgnzx9sfa46p568k8rdmtz7qj85slc6t.key 2> /dev/null
hmy keys import-ks .hmy/extkeystore/one1auqndgthqu5lznsn7tuma8s5333cq0y07cwc6x.key 2> /dev/null
hmy keys import-ks .hmy/extkeystore/one19aw2wcr5y4lxeuwt0ajgt5aw3a3qkjdgg67ygj.key 2> /dev/null
hmy keys import-ks .hmy/extkeystore/one1eenp9ujcrmyaq22ef6jrpry2k97tjz4xs6ppcf.key 2> /dev/null
hmy keys import-ks .hmy/extkeystore/one19zzwsxr0uf2fe34y8qkadek2v0eh6h5mg2deg6.key 2> /dev/null
hmy keys import-ks .hmy/extkeystore/one1lctumupg2y009pjmnhnmn4nqjk0zf0dspjanf7.key 2> /dev/null

echo "Let's fund all the validator account"
hmy --node=http://127.0.0.1:9500 transfer     --from one1zksj3evekayy90xt4psrz8h6j2v3hla4qwz4ur --to one17ughrllgnzx9sfa46p568k8rdmtz7qj85slc6t     --from-shard 0 --to-shard 0 --amount 110000 
hmy --node=http://127.0.0.1:9500 transfer     --from one1zksj3evekayy90xt4psrz8h6j2v3hla4qwz4ur --to one1auqndgthqu5lznsn7tuma8s5333cq0y07cwc6x     --from-shard 0 --to-shard 0 --amount 110000 
hmy --node=http://127.0.0.1:9500 transfer     --from one1zksj3evekayy90xt4psrz8h6j2v3hla4qwz4ur --to one19aw2wcr5y4lxeuwt0ajgt5aw3a3qkjdgg67ygj     --from-shard 0 --to-shard 0 --amount 110000 
hmy --node=http://127.0.0.1:9500 transfer     --from one1zksj3evekayy90xt4psrz8h6j2v3hla4qwz4ur --to one1eenp9ujcrmyaq22ef6jrpry2k97tjz4xs6ppcf     --from-shard 0 --to-shard 0 --amount 110000 
hmy --node=http://127.0.0.1:9500 transfer     --from one1zksj3evekayy90xt4psrz8h6j2v3hla4qwz4ur --to one19zzwsxr0uf2fe34y8qkadek2v0eh6h5mg2deg6     --from-shard 0 --to-shard 0 --amount 110000 
hmy --node=http://127.0.0.1:9500 transfer     --from one1zksj3evekayy90xt4psrz8h6j2v3hla4qwz4ur --to one1lctumupg2y009pjmnhnmn4nqjk0zf0dspjanf7     --from-shard 0 --to-shard 0 --amount 110000 


#wait for epoch 2
epoch=$(hmy blockchain latest-headers --node="http://localhost:9500" | jq -r '.["result"]["beacon-chain-header"]["epoch"]')
while (( epoch < 2 )); do
	echo "Not yet on epoch 2 .. waiting 30s"
	epoch=$(hmy blockchain latest-headers --node="http://localhost:9500" | jq -r '.["result"]["beacon-chain-header"]["epoch"]')
	sleep 30
done

echo "Now in epoch 2, we'll create the external validators"

hmy --node="http://localhost:9500" staking create-validator \
    --validator-addr one17ughrllgnzx9sfa46p568k8rdmtz7qj85slc6t --amount 10000 \
    --bls-pubkeys 4f41a37a3a8d0695dd6edcc58142c6b7d98e74da5c90e79b587b3b960b6a4f5e048e6d8b8a000d77a478d44cd640270c,7dcc035a943e29e17959dabe636efad7303d2c6f273ace457ba9dcc2fd19d3f37e70ba1cd8d082cf8ff7be2f861db48c \
    --name "s0-localnet-validator1" --identity "validator1" --details "validator1" \
    --security-contact "localnet" --website "localnet.one" \
    --max-change-rate 0.1 --max-rate 0.1 --rate 0.1 \
    --max-total-delegation 100000000 --min-self-delegation 10000 --bls-pubkeys-dir .hmy/extbls/
	
hmy --node="http://localhost:9500" staking create-validator \
    --validator-addr one1auqndgthqu5lznsn7tuma8s5333cq0y07cwc6x --amount 10000 \
    --bls-pubkeys b0917378b179a519a5055259c4f8980cce37d58af300b00dd98b07076d3d9a3b16c4a55f84522f553872225a7b1efc0c \
    --name "s0-localnet-validator2" --identity "validator2" --details "validator2" \
    --security-contact "localnet" --website "localnet.one" \
    --max-change-rate 0.1 --max-rate 0.1 --rate 0.1 \
    --max-total-delegation 100000000 --min-self-delegation 10000 --bls-pubkeys-dir .hmy/extbls/
	
hmy --node="http://localhost:9500" staking create-validator \
    --validator-addr one19aw2wcr5y4lxeuwt0ajgt5aw3a3qkjdgg67ygj --amount 10000 \
    --bls-pubkeys 5a18d4aa3e6aff4835f07588ae66be19684476d38799f63e54c6b5732fad1e86dce7458b1c295404fb54a0d61e50bb97,81296eedba05047594385e3086e1dab52c9eb9e56f46d86f58447cccc20535d646120171961d74968d27a2ec0f8af285 \
    --name "s1-localnet-validator3" --identity "validator3" --details "validator3" \
    --security-contact "localnet" --website "localnet.one" \
    --max-change-rate 0.1 --max-rate 0.1 --rate 0.1 \
    --max-total-delegation 100000000 --min-self-delegation 10000 --bls-pubkeys-dir .hmy/extbls/
	
hmy --node="http://localhost:9500" staking create-validator \
    --validator-addr one1eenp9ujcrmyaq22ef6jrpry2k97tjz4xs6ppcf --amount 10000 \
    --bls-pubkeys 89eab762e7364d6cf89f7a6c54da794f74eba2e29147992ac66adcef0f0654ef8a727710ee55ad8b532da0dd87811915 \
    --name "s1-localnet-validator4" --identity "validator4" --details "validator4" \
    --security-contact "localnet" --website "localnet.one" \
    --max-change-rate 0.1 --max-rate 0.1 --rate 0.1 \
    --max-total-delegation 100000000 --min-self-delegation 10000 --bls-pubkeys-dir .hmy/extbls/


echo "validator created"
echo '''check their information
hmy blockchain validator information one17ughrllgnzx9sfa46p568k8rdmtz7qj85slc6t --node="http://localhost:9500"
hmy blockchain validator information one1auqndgthqu5lznsn7tuma8s5333cq0y07cwc6x --node="http://localhost:9500"
hmy blockchain validator information one19aw2wcr5y4lxeuwt0ajgt5aw3a3qkjdgg67ygj --node="http://localhost:9500"
hmy blockchain validator information one1eenp9ujcrmyaq22ef6jrpry2k97tjz4xs6ppcf --node="http://localhost:9500"
'''

