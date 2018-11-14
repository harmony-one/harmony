package ida

import (
	"github.com/simple-rules/harmony-benchmark/p2p"
	"time"
)

// Symbol is produced from a RaptorQ implementation.
type Symbol []byte

// Message is type of general message gopssiped
type Message []byte

// RaptorQ interface.
type RaptorQ interface {
	Init()
	Process(msg Message) chan Symbol
}

// IDA interface.
type IDA interface {
	TakeRaptorQ(raptorQImp *RaptorQ)
	Process(msg Message, peers []p2p.Peer, done chan struct{}, timeout time.Duration) error
}
