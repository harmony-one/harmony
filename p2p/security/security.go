package security

import (
	"fmt"
	"sync"
	"time"

	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/internal/utils/blockedpeers"
	libp2p_network "github.com/libp2p/go-libp2p/core/network"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/pkg/errors"
)

type Security interface {
	OnConnectCheck(net libp2p_network.Network, conn libp2p_network.Conn) error
	OnDisconnectCheck(conn libp2p_network.Conn) error
}

type peerMap struct {
	peers map[string][]string
}

func newPeersMap() *peerMap {
	return &peerMap{
		peers: make(map[string][]string),
	}
}

func (peerMap *peerMap) Len() int {
	return len(peerMap.peers)
}

func (peerMap *peerMap) Store(key string, value []string) {
	peerMap.peers[key] = value
}

func (peerMap *peerMap) HasKey(key string) bool {
	_, ok := peerMap.peers[key]
	return ok
}

func (peerMap *peerMap) Delete(key string) {
	delete(peerMap.peers, key)
}

func (peerMap *peerMap) Load(key string) (value []string, ok bool) {
	value, ok = peerMap.peers[key]
	return value, ok
}

func (peerMap *peerMap) Range(f func(key string, value []string) bool) {
	for key, value := range peerMap.peers {
		if !f(key, value) {
			break
		}
	}
}

type Manager struct {
	maxConnPerIP int
	maxPeers     int

	mutex  sync.Mutex
	peers  *peerMap // All the connected nodes, key is the Peer's IP, value is the peer's ID array
	banned *blockedpeers.Manager
}

func NewManager(maxConnPerIP int, maxPeers int, banned *blockedpeers.Manager) *Manager {
	if maxConnPerIP < 0 {
		panic("maximum connections per IP must not be negative")
	}
	if maxPeers < 0 {
		panic("maximum peers must not be negative")
	}
	return &Manager{
		maxConnPerIP: maxConnPerIP,
		maxPeers:     maxPeers,
		peers:        newPeersMap(),
		banned:       banned,
	}
}

func (m *Manager) RangePeers(f func(key string, value []string) bool) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.peers.Range(f)
}

func (m *Manager) OnConnectCheck(net libp2p_network.Network, conn libp2p_network.Conn) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	remoteIp, err := getRemoteIP(conn)
	if err != nil {
		return errors.Wrap(err, "failed on get remote ip")
	}

	peers, _ := m.peers.Load(remoteIp)

	// avoid add repeatedly
	peerID := conn.RemotePeer().String()
	_, ok := find(peers, peerID)
	if !ok {
		peers = append(peers, peerID)
	}

	if m.maxConnPerIP > 0 && len(peers) > m.maxConnPerIP {
		utils.Logger().Warn().
			Int("len(peers)", len(peers)).
			Int("maxConnPerIP", m.maxConnPerIP).
			Msgf("too many connections from %s, closing", remoteIp)
		return net.ClosePeer(conn.RemotePeer())
	}

	currentPeerCount := m.peers.Len()
	// only limit addition if it's a new peer and not an existing peer with new connection
	if m.maxPeers > 0 && currentPeerCount >= m.maxPeers && !m.peers.HasKey(remoteIp) {
		utils.Logger().Warn().
			Int("connected peers", currentPeerCount).
			Str("new peer", remoteIp).
			Msg("too many peers, closing")
		return net.ClosePeer(conn.RemotePeer())
	}
	if m.banned.IsBanned(conn.RemotePeer(), time.Now()) {
		utils.Logger().Warn().
			Str("new peer", remoteIp).
			Msg("peer is banned, closing")
		return net.ClosePeer(conn.RemotePeer())
	}

	m.peers.Store(remoteIp, peers)
	return nil
}

func (m *Manager) OnDisconnectCheck(conn libp2p_network.Conn) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	ip, err := getRemoteIP(conn)
	if err != nil {
		return errors.Wrap(err, "failed on get ip")
	}

	peers, ok := m.peers.Load(ip)
	if !ok {
		return nil
	}

	peerID := conn.RemotePeer().String()
	index, ok := find(peers, peerID)
	if ok {
		peers = append(peers[:index], peers[index+1:]...)
		if len(peers) == 0 {
			m.peers.Delete(ip)
		} else {
			m.peers.Store(ip, peers)
		}
	}

	return nil
}

func find(slice []string, val string) (int, bool) {
	for i, item := range slice {
		if item == val {
			return i, true
		}
	}

	return -1, false
}

func getRemoteIP(conn libp2p_network.Conn) (string, error) {
	for _, protocol := range conn.RemoteMultiaddr().Protocols() {
		switch protocol.Code {
		case ma.P_IP4:
			ip, err := conn.RemoteMultiaddr().ValueForProtocol(ma.P_IP4)
			if err != nil {
				return "", errors.Wrap(err, "failed on get ipv4 addr")
			}
			return ip, nil
		case ma.P_IP6:
			ip, err := conn.RemoteMultiaddr().ValueForProtocol(ma.P_IP6)
			if err != nil {
				return "", errors.Wrap(err, "failed on get ipv6 addr")
			}
			return ip, nil
		}
	}

	return "", errors.New(fmt.Sprintf("failed on get remote peer ip from addr: %s", conn.RemoteMultiaddr().String()))
}
