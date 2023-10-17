package security

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p"
	ic "github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	libp2p_network "github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/stretchr/testify/assert"
)

type ConnectCallback func(net libp2p_network.Network, conn libp2p_network.Conn) error
type DisconnectCallback func(conn libp2p_network.Conn) error

type fakeHost struct {
	onConnections []ConnectCallback
	onDisconnects []DisconnectCallback
}

func (fh *fakeHost) Listen(libp2p_network.Network, ma.Multiaddr)      {}
func (fh *fakeHost) ListenClose(libp2p_network.Network, ma.Multiaddr) {}
func (fh *fakeHost) Connected(net libp2p_network.Network, conn libp2p_network.Conn) {
	for _, function := range fh.onConnections {
		if err := function(net, conn); err != nil {
			fmt.Println("failed on peer connected callback")
		}
	}
}

func (fh *fakeHost) Disconnected(net libp2p_network.Network, conn libp2p_network.Conn) {
	for _, function := range fh.onDisconnects {
		if err := function(conn); err != nil {
			fmt.Println("failed on peer disconnected callback")
		}
	}
}

func (mh *fakeHost) OpenedStream(libp2p_network.Network, libp2p_network.Stream) {}
func (mh *fakeHost) ClosedStream(libp2p_network.Network, libp2p_network.Stream) {}
func (mh *fakeHost) SetConnectCallback(callback ConnectCallback) {
	mh.onConnections = append(mh.onConnections, callback)
}

func (mh *fakeHost) SetDisconnectCallback(callback DisconnectCallback) {
	mh.onDisconnects = append(mh.onDisconnects, callback)
}

func TestManager_OnConnectCheck(t *testing.T) {
	h1, err := newPeer(50550)
	assert.Nil(t, err)
	defer h1.Close()

	fakeHost := &fakeHost{}
	security := NewManager(2, 1)
	h1.Network().Notify(fakeHost)
	fakeHost.SetConnectCallback(security.OnConnectCheck)
	fakeHost.SetDisconnectCallback(security.OnDisconnectCheck)
	h2, err := newPeer(50551)
	assert.Nil(t, err)
	defer h2.Close()
	err = h2.Connect(context.Background(), peer.AddrInfo{ID: h1.ID(), Addrs: h1.Network().ListenAddresses()})
	assert.Nil(t, err)

	security.peers.Range(func(k, v interface{}) bool {
		peers := v.([]string)
		assert.Equal(t, 1, len(peers))
		return true
	})

	h3, err := newPeer(50552)
	assert.Nil(t, err)
	defer h3.Close()
	err = h3.Connect(context.Background(), peer.AddrInfo{ID: h1.ID(), Addrs: h1.Network().ListenAddresses()})
	assert.Nil(t, err)
	security.peers.Range(func(k, v interface{}) bool {
		peers := v.([]string)
		assert.Equal(t, 2, len(peers))
		return true
	})

	h4, err := newPeer(50553)
	assert.Nil(t, err)
	defer h4.Close()
	err = h4.Connect(context.Background(), peer.AddrInfo{ID: h1.ID(), Addrs: h1.Network().ListenAddresses()})
	assert.Nil(t, err)
	security.peers.Range(func(k, v interface{}) bool {
		peers := v.([]string)
		assert.Equal(t, 2, len(peers))
		return true
	})
}

func TestManager_OnDisconnectCheck(t *testing.T) {
	h1, err := newPeer(50550)
	assert.Nil(t, err)
	defer h1.Close()

	fakeHost := &fakeHost{}
	security := NewManager(2, 0)
	h1.Network().Notify(fakeHost)
	fakeHost.SetConnectCallback(security.OnConnectCheck)
	fakeHost.SetDisconnectCallback(security.OnDisconnectCheck)
	h2, err := newPeer(50551)
	assert.Nil(t, err)
	defer h2.Close()
	err = h2.Connect(context.Background(), peer.AddrInfo{ID: h1.ID(), Addrs: h1.Network().ListenAddresses()})
	assert.Nil(t, err)

	security.peers.Range(func(k, v interface{}) bool {
		peers := v.([]string)
		assert.Equal(t, 1, len(peers))
		return true
	})

	err = h2.Network().ClosePeer(h1.ID())
	assert.Nil(t, err)
	time.Sleep(200 * time.Millisecond)
	security.peers.Range(func(k, v interface{}) bool {
		peers := v.([]string)
		assert.Equal(t, 0, len(peers))
		return true
	})
}

func newPeer(port int) (host.Host, error) {
	priv, _, err := ic.GenerateKeyPair(ic.RSA, 2048)
	if err != nil {
		return nil, err
	}

	listenAddr := fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", port)
	host, err := libp2p.New(libp2p.ListenAddrStrings(listenAddr), libp2p.DisableRelay(), libp2p.Identity(priv), libp2p.NoSecurity)
	if err != nil {
		return nil, err
	}

	return host, nil
}

type fakeConn struct{}

func (conn *fakeConn) ID() string                                               { return "" }
func (conn *fakeConn) NewStream(context.Context) (libp2p_network.Stream, error) { return nil, nil }
func (conn *fakeConn) GetStreams() []libp2p_network.Stream                      { return nil }
func (conn *fakeConn) Close() error                                             { return nil }
func (conn *fakeConn) LocalPeer() peer.ID                                       { return "" }
func (conn *fakeConn) LocalPrivateKey() ic.PrivKey                              { return nil }
func (conn *fakeConn) RemotePeer() peer.ID                                      { return "" }
func (conn *fakeConn) RemotePublicKey() ic.PubKey                               { return nil }
func (conn *fakeConn) ConnState() libp2p_network.ConnectionState {
	return libp2p_network.ConnectionState{}
}
func (conn *fakeConn) LocalMultiaddr() ma.Multiaddr { return nil }
func (conn *fakeConn) RemoteMultiaddr() ma.Multiaddr {
	addr, _ := ma.NewMultiaddr("/ip6/fe80::7802:31ff:fee9:c093/tcp/50550")
	return addr
}
func (conn *fakeConn) Stat() libp2p_network.ConnStats  { return libp2p_network.ConnStats{} }
func (conn *fakeConn) Scope() libp2p_network.ConnScope { return nil }
func (conn *fakeConn) IsClosed() bool                  { return false }

func TestGetRemoteIP(t *testing.T) {
	ip, err := getRemoteIP(&fakeConn{})
	assert.Nil(t, err)
	assert.Equal(t, "fe80::7802:31ff:fee9:c093", ip)
}
