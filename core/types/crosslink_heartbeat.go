package types

type CrosslinkHeartbeat struct {
	ShardID                  uint32
	LatestContinuousBlockNum uint64
	Epoch                    uint64
	PublicKey                []byte
	Signature                []byte
}
