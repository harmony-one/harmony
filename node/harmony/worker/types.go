package worker

import (
	"time"

	"github.com/harmony-one/harmony/block"
)

// CommitSigReceiverTimeout is the timeout for the receiving side of the commit sig
// if timeout, the receiver should instead ready directly from db for the commit sig
const CommitSigReceiverTimeout = 8 * time.Second

type Environment interface {
	CurrentHeader() *block.Header
}
