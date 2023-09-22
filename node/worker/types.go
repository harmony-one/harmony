package worker

import "github.com/harmony-one/harmony/block"

type Environment interface {
	CurrentHeader() *block.Header
}
