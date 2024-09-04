package bootnode

// ShardID defines the ID of a shard
type ShardID uint32

func getNetworkPrefix(shardID ShardID) (netPre string) {
	switch GetShardConfig(uint32(shardID)).GetNetworkType() {
	case Mainnet:
		netPre = "harmony"
	case Testnet:
		netPre = "hmy/testnet"
	case Pangaea:
		netPre = "hmy/pangaea"
	case Partner:
		netPre = "hmy/partner"
	case Stressnet:
		netPre = "hmy/stressnet"
	case Devnet:
		netPre = "hmy/devnet"
	case Localnet:
		netPre = "hmy/local"
	default:
		netPre = "hmy/misc"
	}
	return
}
