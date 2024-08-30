package types

type CrosslinkHeartbeat struct {
	ShardID                  uint32
	LatestContinuousBlockNum uint64
	//Deprecated
	Epoch     uint64
	PublicKey []byte
	Signature []byte
}
