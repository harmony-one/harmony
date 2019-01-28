package beacon

import (
	"github.com/harmony-one/harmony/consensus"
	"github.com/harmony-one/harmony/node"
)

//References
// https://medium.com/prysmatic-labs/ethereum-2-0-development-update-20-prysmatic-labs-e42724a2ba44
//
const (
	NumShards              = 10
	BeaconChainShardNum    = 0
	DepositContractAddress = 1234567 //Currently a regular address, later to be replaced by a smart contract address.
	MinDepositAmount	= 10
)

type beaconservice struct {
	beaconconsensus *consensus.consensus //beaconchain consensus piggybacks on
	waiting_nodes *node.Node[]
	active_nodes *node.Node[]
}
