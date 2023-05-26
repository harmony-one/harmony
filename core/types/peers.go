package types

type PeerBroadcastMessage struct {
	Endpoints []string
	ShardID   uint32
}
