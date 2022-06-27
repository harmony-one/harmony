#!/bin/bash
set -e

Help()
{
   # Display Help
   echo "ENV variable supported in this format: HARMONY_{GROUP NAME}_{CONFIG NAME}, e.g"
   echo
   echo "HARMONY_NETWORK_NETWORKTYPE    [Network] NetworkType: network to join, <mainnet|testnet>"
   echo "HARMONY_GENERAL_NODETYPE       [General] NodeType: run node type,  <explorer|validator>"
   echo "HARMONY_HTTP_ROSETTAENABLED    [HTTP] RosettaEnabled: enable rosetta, <true|false>"
   echo
}

Help

#LOG_PATH=${HARMONY_HOME}/log
CONFIG_FILE=${HARMONY_HOME}/harmony.conf
BLSKEYS_PATH=${HARMONY_HOME}/.hmy/blskeys
BLACKLIST_FILE=${HARMONY_HOME}/.hmy/blacklist.txt

# Create directory
mkdir -p ${BLSKEYS_PATH}

# Replace harmony.config with Env available to start the client
env_vars=$(printenv | awk -v RS='\n' -F= '/^HARMONY_/{print $1}')

mainnet_dns_zone="t.hmny.io"
mainnet_boot_nodes="[\"/dnsaddr/bootstrap.t.hmny.io\"]"
mainnet_min_peers=6

bls_keys_passphrase=harmony
bls_keys_node="https://api.s0.b.hmny.io"
bls_keys_shardid=${HARMONY_GENERAL_SHARDID:-0}

# Replace mainnet related configurations
network_type=${HARMONY_NETWORK_NETWORKTYPE:-testnet}
if [[ "${network_type}" == "mainnet" ]] ; then
  sed -i "s#Zone.*#Zone = \"$mainnet_dns_zone\"#"           ${CONFIG_FILE}
  sed -i "s#BootNodes.*#BootNodes = $mainnet_boot_nodes#"   ${CONFIG_FILE}
  sed -i "s#NetworkType.*#NetworkType = \"$val\"#"          ${CONFIG_FILE}
  sed -i "s#MinPeers.*#MinPeers = $mainnet_min_peers#"      ${CONFIG_FILE}

  bls_keys_node="https://api.s0.t.hmny.io"
fi

re_numbers='^[0-9]+$'
for name in ${env_vars[@]}
do
  val="${!name}"

  group_name=$(echo $name | cut -d'_' -f2)
  key_name=$(echo $name | cut -d'_' -f3)

  if [[ "$val" =~ $re_numbers ]] || [[ "$val" == "true" ]] || [[ "$val" == "false" ]]; then
    sed -i "\#\[${group_name}\]#I,\#^\s*\$# s#\(${key_name} = \)\(.*\)#\1${val}#I"      ${CONFIG_FILE}
  else
    sed -i "\#\[${group_name}\]#I,\#^\s*\$# s#\(${key_name} = \)\(.*\)#\1\"${val}\"#I"  ${CONFIG_FILE}
  fi
done

# Generate BLS key & pass for validator if not exist
node_type=${HARMONY_GENERAL_NODETYPE:-explorer}
if [[ "${node_type}" == "validator" ]] && [[ ! "$(ls -A ${BLSKEYS_PATH})" ]] ; then
  echo -n $bls_keys_passphrase > bls.pass

  hmy keys generate-bls-keys --node=${bls_keys_node} --count 1 --shard ${bls_keys_shardid} \
    --passphrase-file bls.pass

  key_name=$(find . -name '*.key' | cut -d'/' -f2 | cut -d'.' -f1)

  mv $key_name.key ${BLSKEYS_PATH}/
  mv bls.pass ${BLSKEYS_PATH}/$key_name.pass

  echo "### BLS key for validator###"
  cat ${BLSKEYS_PATH}/$key_name.key
  echo "#############################"
else
  sed -i "s#IsArchival.*#IsArchival = true#"  ${CONFIG_FILE}
fi

# Blacklist
if [[ ! -f "$BLACKLIST_FILE" ]]; then
  echo > $BLACKLIST_FILE
fi

echo "#############################"
echo "### Harmony Configuration ###"
echo
cat ${CONFIG_FILE}
echo
echo "#############################"

exec yes no | "$@"
