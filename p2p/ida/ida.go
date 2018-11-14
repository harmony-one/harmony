package ida

import (
	"time"

	"github.com/simple-rules/harmony-benchmark/p2p"
)

// IDAImp implements IDA interface.
type IDAImp struct {
	raptorQImp RaptorQ
}

// TakeRaptorQ takes RaptorQ implementation.
func (ida *IDAImp) TakeRaptorQ(raptorQImp RaptorQ) {
	ida.raptorQImp = raptorQImp
}

// Process implements very simple IDA logic.
func (ida *IDAImp) Process(msg Message, peers []p2p.Peer, done chan struct{}, timeout time.Duration) error {
	if ida.raptorQImp == nil {
		return ErrRaptorImpNotFound
	}
	chunkStream := ida.raptorQImp.Process(msg)
	id := 0
	for {
		select {
		case <-done:
			return nil
		case <-time.After(timeout):
			return ErrTimeOut
		case chunk := <-chunkStream:
			p2p.SendMessage(peers[id], chunk)
			id++
		}
	}
}
