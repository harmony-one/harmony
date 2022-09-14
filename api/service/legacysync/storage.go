package legacysync

import (
	"sync"

	"github.com/harmony-one/harmony/p2p"
)

type peerDupID struct {
	ip   string
	port string
}

// Storage keeps successful and failed peers.
type Storage struct {
	mu   sync.Mutex
	succ map[peerDupID]p2p.Peer
	fail map[peerDupID]p2p.Peer
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
	for _, p := range s.succ {
		peers = append(peers, p)
		if len(peers) >= n {
			break
		}
	}
	for _, p := range s.fail {
		peers = append(peers, p)
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
	s.succ[d] = p
}

func (s *Storage) Fail(p p2p.Peer) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d := peerDupID{p.IP, p.Port}
	delete(s.succ, d)
	s.fail[d] = p
}

func (s *Storage) IsFail(p p2p.Peer) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.isFail(peerDupID{p.IP, p.Port})
}

func (s *Storage) isFail(d peerDupID) bool {
	_, ok := s.fail[d]
	return ok
}

func (s *Storage) Success(p p2p.Peer) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d := peerDupID{p.IP, p.Port}
	delete(s.fail, d)
	s.succ[d] = p
}

func (s *Storage) isSuccess(d peerDupID) bool {
	_, ok := s.succ[d]
	return ok
}

func (s *Storage) IsSuccess(p p2p.Peer) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.isSuccess(peerDupID{p.IP, p.Port})
}

func NewStorage() *Storage {
	return &Storage{
		succ: make(map[peerDupID]p2p.Peer),
		fail: make(map[peerDupID]p2p.Peer),
	}
}
