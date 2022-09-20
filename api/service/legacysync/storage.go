package legacysync

import (
	"sync"

	"github.com/harmony-one/harmony/internal/linkedhashmap"
	"github.com/harmony-one/harmony/p2p"
)

type peerDupID struct {
	ip   string
	port string
}

// Storage keeps successful and failed peers.
type Storage struct {
	mu   sync.Mutex
	succ *linkedhashmap.HashMap[peerDupID, p2p.Peer]
	fail *linkedhashmap.HashMap[peerDupID, p2p.Peer]
}

func (s *Storage) Contains(p p2p.Peer) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	d := peerDupID{p.IP, p.Port}
	return s.contains(d)
}

func (s *Storage) contains(d peerDupID) bool {
	return s.isSuccess(d) || s.isFail(d)
}

func (s *Storage) AddPeers(peers []p2p.Peer) {
	for _, p := range peers {
		s.AddPeer(p)
	}
}

// GetPeers returns no more than `n` peers. First `succ` then `fail`.
func (s *Storage) GetPeers(n int) []p2p.Peer {
	s.mu.Lock()
	defer s.mu.Unlock()
	peers := make([]p2p.Peer, 0, n)
	for s.succ.HasNext() {
		peers = append(peers, s.succ.Next().Value)
		if len(peers) >= n {
			break
		}
	}
	for s.fail.HasNext() {
		peers = append(peers, s.fail.Next().Value)
		if len(peers) >= n {
			break
		}
	}
	return peers
}

func (s *Storage) AddPeer(p p2p.Peer) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d := peerDupID{p.IP, p.Port}
	if s.contains(d) {
		return
	}
	s.succ.Add(d, p)
}

func (s *Storage) Fail(p p2p.Peer) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d := peerDupID{p.IP, p.Port}
	s.succ.Remove(d)
	s.fail.Add(d, p)
}

func (s *Storage) IsFail(p p2p.Peer) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.isFail(peerDupID{p.IP, p.Port})
}

func (s *Storage) isFail(d peerDupID) bool {
	return s.fail.Contains(d)
}

func (s *Storage) Success(p p2p.Peer) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d := peerDupID{p.IP, p.Port}
	s.fail.Remove(d)
	s.succ.Add(d, p)
}

func (s *Storage) isSuccess(d peerDupID) bool {
	return s.succ.Contains(d)
}

func (s *Storage) IsSuccess(p p2p.Peer) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.isSuccess(peerDupID{p.IP, p.Port})
}

func NewStorage() *Storage {
	return &Storage{
		succ: linkedhashmap.New[peerDupID, p2p.Peer](),
		fail: linkedhashmap.New[peerDupID, p2p.Peer](),
	}
}
