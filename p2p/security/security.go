package security

import (
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/harmony-one/harmony/internal/utils"
	libp2p_network "github.com/libp2p/go-libp2p/core/network"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/pkg/errors"
)

type Security interface {
	OnConnectCheck(net libp2p_network.Network, conn libp2p_network.Conn) error
	OnDisconnectCheck(conn libp2p_network.Conn) error
}

type Manager struct {
	maxConnPerIP int
	maxPeers     int64

	mutex sync.Mutex
	peers peerMap // All the connected nodes, key is the Peer's IP, value is the peer's ID array
}

type peerMap struct {
	count int64
	peers sync.Map
}

func (peerMap *peerMap) Len() int64 {
	return atomic.LoadInt64(&peerMap.count)
}

func (peerMap *peerMap) Store(key, value interface{}) {
	// only increment if you didn't have this key
	hasKey := peerMap.HasKey(key)
	peerMap.peers.Store(key, value)
	if !hasKey {
		atomic.AddInt64(&peerMap.count, 1)
	}
}

func (peerMap *peerMap) HasKey(key interface{}) bool {
	hasKey := false
	peerMap.peers.Range(func(k, v interface{}) bool {
		if k == key {
			hasKey = true
			return false
		}
		return true
	})
	return hasKey
}

func (peerMap *peerMap) Delete(key interface{}) {
	peerMap.peers.Delete(key)
	atomic.AddInt64(&peerMap.count, -1)
}

func (peerMap *peerMap) Load(key interface{}) (value interface{}, ok bool) {
	return peerMap.peers.Load(key)
}

func (peerMap *peerMap) Range(f func(key, value any) bool) {
	peerMap.peers.Range(f)
}

func NewManager(maxConnPerIP int, maxPeers int64) *Manager {
	if maxConnPerIP < 0 {
		panic("maximum connections per IP must not be negative")
	}
	if maxPeers < 0 {
		panic("maximum peers must not be negative")
	}
	return &Manager{
		maxConnPerIP: maxConnPerIP,
		maxPeers:     maxPeers,
	}
}

func (m *Manager) OnConnectCheck(net libp2p_network.Network, conn libp2p_network.Conn) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	remoteIp, err := getRemoteIP(conn)
	if err != nil {
		return errors.Wrap(err, "failed on get remote ip")
	}

	value, ok := m.peers.Load(remoteIp)
	if !ok {
		value = []string{}
	}

	peers, ok := value.([]string)
	if !ok {
		return errors.New("peers info type err")
	}

	// avoid add repeatedly
	peerID := conn.RemotePeer().String()
	_, ok = find(peers, peerID)
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
			Int64("connected peers", currentPeerCount).
			Str("new peer", remoteIp).
			Msg("too many peers, closing")
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

	value, ok := m.peers.Load(ip)
	if !ok {
		return nil
	}

	peers, ok := value.([]string)
	if !ok {
		return errors.New("peers info type err")
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
