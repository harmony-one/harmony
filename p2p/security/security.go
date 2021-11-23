package security

import (
	"fmt"
	"sync"

	"github.com/harmony-one/harmony/internal/utils"
	libp2p_network "github.com/libp2p/go-libp2p-core/network"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/pkg/errors"
)

type Security interface {
	OnConnectCheck(net libp2p_network.Network, conn libp2p_network.Conn) error
	OnDisconnectCheck(conn libp2p_network.Conn) error
}

type Manager struct {
	maxConnPerIP int

	mutex sync.Mutex
	peers sync.Map // All the connected nodes, key is the Peer's IP, value is the peer's ID array
}

func NewManager(maxConnPerIP int) *Manager {
	return &Manager{
		maxConnPerIP: maxConnPerIP,
	}
}

func (m *Manager) OnConnectCheck(net libp2p_network.Network, conn libp2p_network.Conn) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	ip, err := getIP(conn)
	if err != nil {
		return errors.Wrap(err, "failed on get ip")
	}

	value, ok := m.peers.Load(ip)
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

	if len(peers) > m.maxConnPerIP {
		utils.Logger().Warn().Int("len(peers)", len(peers)).Int("maxConnPerIP", m.maxConnPerIP).
			Msg("Too much peers, closing")
		return net.ClosePeer(conn.RemotePeer())
	}

	m.peers.Store(ip, peers)
	return nil
}

func (m *Manager) OnDisconnectCheck(conn libp2p_network.Conn) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	ip, err := getIP(conn)
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
		m.peers.Store(ip, peers)
		if len(peers) == 0 {
			m.peers.Delete(ip)
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

func getIP(conn libp2p_network.Conn) (string, error) {
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
