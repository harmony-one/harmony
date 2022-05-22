package types

type CrosslinkHeartbeat struct {
	ShardID   uint32
	BlockID   uint64
	PublicKey []byte
	Signature []byte
}
