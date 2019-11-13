package slash

import "github.com/harmony-one/harmony/shard"

// Slasher ..
type Slasher interface {
	ShouldSlash(shard.BlsPublicKey) bool
}
