#!/usr/bin/env bash

ME=$(basename "$0")
CONFIG=/etc/harmony/harmony.conf
VER=v1.0

function usage() {
   local MSG=${1}

   cat <<-EOT
$MSG
This script is a helper script to edit the $CONFIG.

Usage: $ME [options]

Options:
   -t validator/explorer   specify the type of the node is explorer or validator (default is: $TYPE)
   -s int                  specify the shard id, only needed if node type is explorer (default is: $SHARD)
   -h                      print this help
   -v                      print out the version of the script

Examples:
   $ME -t explorer -s 0

# TODO: interactive mode
EOT
   exit 0
}

function _setup_validator_config_file() {
   sed -i.bak 's,NodeType =.*,NodeType = "validator",; s,ShardID = .*,ShardID = -1,; s,IsArchival = .*,IsArchival = false,' $CONFIG
}

function _setup_explorer_config_file() {
   sed -i.bak "s,NodeType =.*,NodeType = \"explorer\",; s,ShardID = .*,ShardID = $SHARD,; s,IsArchival = .*,IsArchival = true," $CONFIG
}

function setup_config_file() {
   case $TYPE in
      validator) _setup_validator_config_file ;;
      explorer) _setup_explorer_config_file ;;
      *) usage "ERROR: invalid node type! '$TYPE'" ;;
   esac
}

####### default value ######
TYPE=validator
SHARD=-1

while getopts ":t:s:v" opt; do
   case ${opt} in
      t) TYPE=${OPTARG} ;;
      s) SHARD=${OPTARG} ;;
      v) echo $VER; exit ;;
      *) usage ;;
   esac
done

shift $((OPTIND-1))

# validate input parameters
case ${TYPE} in
   explorer)
      case ${SHARD} in
         0|1|2|3) ;;
         *) usage "ERROR: invalid shard number! '$SHARD'" ;;
      esac
      ;;
   validator)
      case ${SHARD} in
         -1) ;;
         *) usage "ERROR: do not specify shard number in validator type!!" ;;
      esac
      ;;
   *) usage "ERROR: invalid node type! '$TYPE'" ;;
esac

setup_config_file

echo "Please run 'sudo systemctl restart harmony' to reload the configuration"
