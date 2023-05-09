package knownpeers

import (
	"fmt"
	"strings"

	"github.com/harmony-one/harmony/p2p"
)

type hosts struct {
	m map[string]struct{}
	n []string
	i int
}

// assume that `peer` is always valid.
func peerFromString(peer string) p2p.Peer {
	rs := strings.SplitN(peer, ":", 2)
	if len(rs) != 2 {
		return p2p.Peer{}
	}
	return p2p.Peer{
		IP:   rs[0],
		Port: rs[1],
	}
}

func peerToString(peer p2p.Peer) string {
	return fmt.Sprintf("%s:%s", peer.IP, peer.Port)
}

func newHosts() *hosts {
	return &hosts{
		m: make(map[string]struct{}),
	}
}

func (a *hosts) get(n uint16) []p2p.Peer {
	if n > uint16(len(a.n)) {
		n = uint16(len(a.n))
	}
	out := make([]p2p.Peer, 0, n)
	for n > 0 {
		a.i %= len(a.n)
		out = append(out, peerFromString(a.n[a.i]))
		n--
		a.i++
	}
	return out
}

func (a *hosts) add(peer string) (added bool) {
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

type knownHosts struct {
	checkedHosts   *hosts
	uncheckedHosts *hosts
}

func NewKnownHosts() KnownPeers {
	return &knownHosts{
		checkedHosts:   newHosts(),
		uncheckedHosts: newHosts(),
	}
}

func NewKnownHostsThreadSafe() KnownPeers {
	return WrapThreadSafe(NewKnownHosts())
}

func (a *knownHosts) GetChecked(limit int) []p2p.Peer {
	if limit < 0 {
		panic("limit must be non-negative")
	}
	return a.checkedHosts.get(uint16(limit))
}

func (a *knownHosts) GetUnchecked(limit int) []p2p.Peer {
	if limit < 0 {
		panic("limit must be non-negative")
	}
	return a.uncheckedHosts.get(uint16(limit))
}

func (a *knownHosts) AddChecked(hosts ...p2p.Peer) {
	for _, host := range hosts {
		a.checkedHosts.add(peerToString(host))
		a.uncheckedHosts.remove(peerToString(host))
	}
}

func (a *knownHosts) AddUnchecked(hosts ...p2p.Peer) {
	for _, host := range hosts {
		a.addUnchecked(peerToString(host))
	}
}

func (a *knownHosts) addUnchecked(host string) {
	if a.checkedHosts.has(host) {
		return
	}
	a.uncheckedHosts.add(host)
}
