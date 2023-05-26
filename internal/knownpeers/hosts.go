package knownpeers

import (
	"fmt"

	"github.com/harmony-one/harmony/p2p"
)

type hosts struct {
	m     map[string]struct{}
	n     []string
	i     int
	limit int
}

func PeerToString(peer p2p.Peer) string {
	return fmt.Sprintf("%s:%s", peer.IP, peer.Port)
}

func newHosts(limit int) *hosts {
	if limit < 0 {
		panic("limit must be non-negative")
	}
	return &hosts{
		limit: limit,
		m:     make(map[string]struct{}),
	}
}

func (a *hosts) get(n uint16) []p2p.Peer {
	if n > uint16(len(a.n)) {
		n = uint16(len(a.n))
	}
	out := make([]p2p.Peer, 0, n)
	for n > 0 {
		a.i %= len(a.n)
		out = append(out, p2p.PeerFromIpPortUnchecked(a.n[a.i]))
		n--
		a.i++
	}
	return out
}

func (a *hosts) add(peer string) (added bool) {
	if len(a.m) >= a.limit {
		return false
	}
	if _, ok := a.m[peer]; ok {
		return false
	}
	a.m[peer] = struct{}{}
	a.n = append(a.n, peer)
	return true
}

func (a *hosts) has(host string) bool {
	_, ok := a.m[host]
	return ok
}

func (a *hosts) remove(host string) (removed bool) {
	if _, ok := a.m[host]; !ok {
		return false
	}
	delete(a.m, host)
	for i, h := range a.n {
		if h == host {
			a.n = append(a.n[:i], a.n[i+1:]...)
			break
		}
	}
	return true
}

func (a *hosts) count() int {
	return len(a.n)
}

type knownPeers struct {
	checkedHosts   *hosts
	uncheckedHosts *hosts
}

func NewKnownPeers(limitChecked, limitUnchecked int) KnownPeers {
	return &knownPeers{
		checkedHosts:   newHosts(limitChecked),
		uncheckedHosts: newHosts(limitUnchecked),
	}
}

func NewKnownPeersThreadSafe() KnownPeers {
	return WrapThreadSafe(NewKnownPeers(200, 1000))
}

func (a *knownPeers) GetChecked(limit int) []p2p.Peer {
	if limit < 0 {
		panic("limit must be non-negative")
	}
	return a.checkedHosts.get(uint16(limit))
}

func (a *knownPeers) GetUnchecked(limit int) []p2p.Peer {
	if limit < 0 {
		panic("limit must be non-negative")
	}
	return a.uncheckedHosts.get(uint16(limit))
}

func (a *knownPeers) GetUncheckedCount() int {
	return a.uncheckedHosts.count()
}

func (a *knownPeers) GetCheckedCount() int {
	return a.checkedHosts.count()
}

func (a *knownPeers) AddChecked(hosts ...p2p.Peer) {
	for _, host := range hosts {
		a.checkedHosts.add(PeerToString(host))
		a.uncheckedHosts.remove(PeerToString(host))
	}
}

func (a *knownPeers) AddUnchecked(hosts ...p2p.Peer) {
	for _, host := range hosts {
		a.addUnchecked(PeerToString(host))
	}
}

func (a *knownPeers) addUnchecked(host string) {
	if a.checkedHosts.has(host) {
		return
	}
	a.uncheckedHosts.add(host)
}

func (a *knownPeers) RemoveUnchecked(hosts ...p2p.Peer) {
	a.RemoveUnchecked(hosts...)
}

func (a *knownPeers) RemoveChecked(hosts ...p2p.Peer) {
	a.RemoveChecked(hosts...)
}
