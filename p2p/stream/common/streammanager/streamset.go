package streammanager

import (
	"sync"

	sttypes "github.com/harmony-one/harmony/p2p/stream/types"
	"github.com/pkg/errors"
)

var (
	// ErrNoAvailableStream indicates that a request cannot be processed
	// because there are no active streams available.
	ErrNoAvailableStream = errors.New("no available stream")
)

// streamSet is the concurrency safe stream set.
type streamSet struct {
	streams    map[sttypes.StreamID]sttypes.Stream
	numByProto map[sttypes.ProtoSpec]int
	lock       sync.RWMutex
}

func newStreamSet() *streamSet {
	return &streamSet{
		streams:    make(map[sttypes.StreamID]sttypes.Stream),
		numByProto: make(map[sttypes.ProtoSpec]int),
	}
}

func (ss *streamSet) Erase() {
	ss.lock.Lock()
	defer ss.lock.Unlock()

	ss.streams = make(map[sttypes.StreamID]sttypes.Stream)
	ss.numByProto = make(map[sttypes.ProtoSpec]int)
}

func (ss *streamSet) size() int {
	ss.lock.RLock()
	defer ss.lock.RUnlock()

	return len(ss.streams)
}

func (ss *streamSet) get(id sttypes.StreamID) (sttypes.Stream, bool) {
	ss.lock.RLock()
	defer ss.lock.RUnlock()

	if id == "" {
		return nil, false
	}

	st, ok := ss.streams[id]
	return st, ok
}

func (ss *streamSet) addStream(st sttypes.Stream) {
	ss.lock.Lock()
	defer ss.lock.Unlock()

	if _, exists := ss.streams[st.ID()]; exists {
		return
	}

	if spec, err := st.ProtoSpec(); err != nil {
		return
	} else {
		ss.streams[st.ID()] = st
		ss.numByProto[spec]++
	}

}

func (ss *streamSet) deleteStream(st sttypes.Stream) {
	ss.lock.Lock()
	defer ss.lock.Unlock()

	if _, exists := ss.streams[st.ID()]; !exists {
		return
	}

	delete(ss.streams, st.ID())

	spec, _ := st.ProtoSpec()
	ss.numByProto[spec]--
	if ss.numByProto[spec] == 0 {
		delete(ss.numByProto, spec)
	}
}

func (ss *streamSet) slice() []sttypes.Stream {
	ss.lock.RLock()
	defer ss.lock.RUnlock()

	sts := make([]sttypes.Stream, 0, len(ss.streams))
	for _, st := range ss.streams {
		sts = append(sts, st)
	}
	return sts
}

func (ss *streamSet) getStreams() []sttypes.Stream {
	ss.lock.RLock()
	defer ss.lock.RUnlock()

	res := make([]sttypes.Stream, 0)
	for _, st := range ss.streams {
		res = append(res, st)
	}
	return res
}

func (ss *streamSet) popStream() (sttypes.Stream, error) {
	ss.lock.Lock()
	defer ss.lock.Unlock()

	if len(ss.streams) == 0 {
		return nil, ErrNoAvailableStream
	}
	for id, stream := range ss.streams {
		delete(ss.streams, id)
		return stream, nil
	}
	return nil, errors.New("pop stream failed")
}

func (ss *streamSet) numStreamsWithMinProtoSpec(minSpec sttypes.ProtoSpec) int {
	ss.lock.RLock()
	defer ss.lock.RUnlock()

	var res int
	for spec, num := range ss.numByProto {
		if !spec.Version.LessThan(minSpec.Version) {
			res += num
		}
	}
	return res
}
