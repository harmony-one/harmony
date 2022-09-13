package legacysync

import "github.com/harmony-one/harmony/p2p"

type peerDupID struct {
	ip   string
	port string
}

type Storage struct {
	succ map[peerDupID]p2p.Peer
	fail map[peerDupID]p2p.Peer
}

func (s *Storage) Contains(p p2p.Peer) bool {
	d := peerDupID{p.IP, p.Port}
	return s.contains(d)
}

func (s *Storage) contains(d peerDupID) bool {
	_, ok := s.succ[d]
	if ok {
		return true
	}
	_, ok = s.fail[d]
	return ok
}

func (s *Storage) AddPeers(peers []p2p.Peer) {
	for _, p := range peers {
		s.AddPeer(p)
	}
}

func (s *Storage) GetPeersN(n int) []p2p.Peer {
	peers := make([]p2p.Peer, 0, n)
	for _, p := range s.succ {
		peers = append(peers, p)
		if len(peers) >= n {
			break
		}
	}
	return peers
}

func (s *Storage) AddPeer(p p2p.Peer) {
	d := peerDupID{p.IP, p.Port}
	if s.contains(d) {
		return
	}
	s.succ[d] = p
}

func (s *Storage) Fail(p p2p.Peer) {
	d := peerDupID{p.IP, p.Port}
	delete(s.succ, d)
	s.fail[d] = p
}

func (s *Storage) Success(p p2p.Peer) {
	d := peerDupID{p.IP, p.Port}
	delete(s.fail, d)
	s.succ[d] = p
}

func NewStorage() *Storage {
	return &Storage{}
}
