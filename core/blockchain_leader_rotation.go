package core

type leaderRotationMeta struct {
	pub    []byte
	epoch  uint64
	count  uint64
	shifts uint64
}
