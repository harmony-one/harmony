package values

const (
	// BeaconChainShardID is the ShardID of the BeaconChain
	BeaconChainShardID = 0
	// VotingPowerReduceBlockThreshold roughly corresponds to 3 hours
	VotingPowerReduceBlockThreshold = 1350
	// VotingPowerFullReduce roughly corresponds to 12 hours
	VotingPowerFullReduce = 4 * VotingPowerReduceBlockThreshold
)
