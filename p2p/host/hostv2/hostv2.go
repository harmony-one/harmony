package hostv2

//go:generate mockgen -source hostv2.go -destination=mock/hostv2_mock.go

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p"
	libp2p_crypto "github.com/libp2p/go-libp2p-core/crypto"
	libp2p_disc "github.com/libp2p/go-libp2p-core/discovery"
	libp2p_host "github.com/libp2p/go-libp2p-core/host"
	libp2p_peer "github.com/libp2p/go-libp2p-core/peer"
	libp2p_peerstore "github.com/libp2p/go-libp2p-core/peerstore"
	libp2p_disc_impl "github.com/libp2p/go-libp2p-discovery"
	libp2p_dht_impl "github.com/libp2p/go-libp2p-kad-dht"
	libp2p_pubsub "github.com/libp2p/go-libp2p-pubsub"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"

	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
)

const (
	// BatchSizeInByte The batch size in byte (64MB) in which we return data
	BatchSizeInByte = 1 << 16
	// ProtocolID The ID of protocol used in stream handling.
	ProtocolID = "/harmony/0.0.1"

	// Constants for discovery service.
	//numIncoming = 128
	//numOutgoing = 16
)

// pubsub captures the pubsub interface we expect from libp2p.
type pubsub interface {
	Publish(topic string, data []byte) error
	Subscribe(topic string, opts ...libp2p_pubsub.SubOpt) (*libp2p_pubsub.Subscription, error)
}

// discoverer captures the consumer/discoverer part of the service discovery
// interface we expect from libp2p.
// See "github.com/libp2p/go-libp2p-core/discovery".Discoverer for details.
type discoverer interface {
	FindPeers(
		ctx context.Context, ns string, opts ...libp2p_disc.Option,
	) (peers <-chan libp2p_peer.AddrInfo, err error)
}

// advertiser captures the provider/advertiser part of the service discovery
// interface we expect from libp2p.
// See "github.com/libp2p/go-libp2p-core/discovery".Advertiser for details.
type advertiser interface {
	Advertise(
		ctx context.Context, ns string, opts ...libp2p_disc.Option,
	) (ttl time.Duration, err error)
}

type loggingAdvertiser struct {
	wrapped advertiser
}

func (l loggingAdvertiser) Advertise(
	ctx context.Context, ns string, opts ...libp2p_disc.Option,
) (ttl time.Duration, err error) {
	ns64 := base64.StdEncoding.EncodeToString([]byte(ns))
	logger := utils.Logger().With().
		Str("ns", ns64).
		Interface("opts", opts).
		Logger()
	logger.Debug().Msg("advertising")
	t1 := time.Now()
	ttl, err = l.wrapped.Advertise(ctx, ns, opts...)
	t2 := time.Now()
	logger = logger.With().Dur("elapsed", t2.Sub(t1)).Logger()
	if err == nil {
		logger.Debug().Dur("ttl", ttl).Msg("advertised")
	} else {
		logger.Warn().Dur("ttl", ttl).Err(err).Msg("")
	}
	return
}

// bootstrapper captures the bootstrap functionality of the peer routing
// interface we expect from libp2p.
// See "github.com/libp2p/go-libp2p-core/discovery".Routing for details.
type bootstrapper interface {
	Bootstrap(ctx context.Context) error
}

// HostV2 is the version 2 p2p host
type HostV2 struct {
	h      libp2p_host.Host
	pubsub pubsub
	self   p2p.Peer
	priKey libp2p_crypto.PrivKey
	lock   sync.Mutex

	//incomingPeers []p2p.Peer // list of incoming Peers. TODO: fixed number incoming
	//outgoingPeers []p2p.Peer // list of outgoing Peers. TODO: fixed number of outgoing

	// logger
	logger *zerolog.Logger

	// DHT-based content/peer/service routing
	dht  io.Closer // used only for closing
	boot bootstrapper
	adv  advertiser
	disc discoverer

	groupDisc map[p2p.GroupID]*groupDiscovery
}

// Bootstrap bootstraps this node on the network using the given initial set
// of peers.
func (host *HostV2) Bootstrap(
	ctx context.Context, peers []libp2p_peer.AddrInfo,
) error {
	done := make(chan libp2p_peer.AddrInfo)
	connected := make(chan libp2p_peer.AddrInfo)
	logger := host.logger
	h := host.h
	for _, peer := range peers {
		go func(peer libp2p_peer.AddrInfo) {
			defer func() { done <- peer }()
			logger := logger.With().Interface("peer", peer).Logger()
			for trial := 1; trial <= 5; trial++ {
				if func() bool {
					logger := logger.With().Int("trial", trial).Logger()
					logger.Debug().Msg("connecting to initial peer")
					ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
					defer cancel()
					err := h.Connect(ctx, peer)
					if err == nil {
						connected <- peer
						return true
					}
					logger.Warn().Err(err).Msg("cannot connect to initial peer")
					return false
				}() {
					return
				}
			}
			logger.Warn().Msg("giving up connecting to initial peer")
		}(peer)
	}
	proceed := make(chan []libp2p_peer.AddrInfo)
	go func() {
		var connectedPeers []libp2p_peer.AddrInfo
		sent := false
		defer func() {
			if !sent {
				proceed <- connectedPeers
			}
		}()
		remaining := len(peers)
		deadline := time.Now().Add(30 * time.Second)
		timer := time.NewTimer(30 * time.Second)
		for remaining > 0 {
			select {
			case <-timer.C:
				logger.Info().
					Interface("peers", connectedPeers).
					Msg("done connecting to initial peers")
				if !sent {
					proceed <- connectedPeers
					sent = true
				}
			case peer := <-done:
				remaining--
				logger.Debug().
					Interface("peer", peer).
					Int("remaining", remaining).
					Msg("done with initial peer")
			case peer := <-connected:
				connectedPeers = append(connectedPeers, peer)
				logger.Debug().
					Interface("peer", peer).
					Int("num_connected", len(connectedPeers)).
					Msg("connected to initial peer')")
				if len(connectedPeers) == 0 {
					// first peer, shorten deadline to 10s from now
					dl2 := time.Now().Add(10 * time.Second)
					if dl2.Before(deadline) && timer.Stop() {
						deadline = dl2
						timer.Reset(10 * time.Second)
					}
				}
			}
		}
	}()
	numConnected := len(<-proceed)
	if numConnected > 0 {
		logger.Info().
			Int("requested", len(peers)).
			Int("connected", numConnected).
			Msg("connected to initial peers")
	} else {
		logger.Warn().Msg("could not connect to any initial peer")
	}
	if err := host.boot.Bootstrap(ctx); err != nil {
		return errors.Wrapf(err, "cannot bootstrap DHT")
	}
	logger.Info().Msg("successfully bootstrapped the network")
	return nil
}

type groupDiscovery struct {
	groupID  p2p.GroupID
	limit    int
	cooldown time.Duration
	found    []p2p.Peer
	quit     chan struct{}
	waiters  chan chan []p2p.Peer
}

// SendMessageToGroups sends a message to one or more multicast groups.
func (host *HostV2) SendMessageToGroups(groups []p2p.GroupID, msg []byte) error {
	var error error
	for _, group := range groups {
		err := host.pubsub.Publish(string(group), msg)
		if err != nil {
			error = err
		}
	}
	return error
}

// subscription captures the subscription interface we expect from libp2p.
type subscription interface {
	Next(ctx context.Context) (*libp2p_pubsub.Message, error)
	Cancel()
}

// GroupReceiverImpl is a multicast group receiver implementation.
type GroupReceiverImpl struct {
	sub subscription
}

// Close closes this receiver.
func (r *GroupReceiverImpl) Close() error {
	r.sub.Cancel()
	r.sub = nil
	return nil
}

// Receive receives a message.
func (r *GroupReceiverImpl) Receive(ctx context.Context) (
	msg []byte, sender libp2p_peer.ID, err error,
) {
	if r.sub == nil {
		return nil, libp2p_peer.ID(""), fmt.Errorf("GroupReceiver has been closed")
	}
	m, err := r.sub.Next(ctx)
	if err == nil {
		msg = m.Data
		sender = m.GetFrom()
		utils.Logger().Debug().Interface("sender", sender).Int("len", len(msg)).Msg("received a group message")
	} else {
		utils.Logger().Debug().Err(err).Msg("failed to receive message")
	}
	return msg, sender, err
}

// GroupReceiver returns a receiver of messages sent to a multicast group.
// See the GroupReceiver interface for details.
func (host *HostV2) GroupReceiver(group p2p.GroupID) (
	receiver p2p.GroupReceiver, err error,
) {
	sub, err := host.pubsub.Subscribe(string(group))
	if err != nil {
		return nil, err
	}
	return &GroupReceiverImpl{sub: sub}, nil
}

// Peerstore returns the peer store
func (host *HostV2) Peerstore() libp2p_peerstore.Peerstore {
	return host.h.Peerstore()
}

// New creates a host for p2p communication
func New(self *p2p.Peer, priKey libp2p_crypto.PrivKey) *HostV2 {
	listenAddr, err := ma.NewMultiaddr(fmt.Sprintf("/ip4/0.0.0.0/tcp/%s", self.Port))
	// TODO: Convert to zerolog or internal logger interface
	logger := utils.Logger()
	if err != nil {
		logger.Error().Str("IP", self.IP).Str("Port", self.Port).Msg("New MA Error")
		return nil
	}
	addrAdder := NewAddrAdder()
	addrAdder.Start(listenAddr)
	// TODO – use WithCancel for orderly host teardown (which we don't have yet)
	ctx := context.Background()
	p2pHost, err := libp2p.New(ctx,
		libp2p.ListenAddrs(listenAddr), libp2p.Identity(priKey),
		libp2p.AddrsFactory(addrAdder.AddrsFactory),
	)
	catchError(err)
	pubsub, err := libp2p_pubsub.NewGossipSub(ctx, p2pHost)
	// pubsub, err := libp2p_pubsub.NewFloodSub(ctx, p2pHost)
	catchError(err)

	dht, err := libp2p_dht_impl.New(ctx, p2pHost)
	catchError(err)

	disc := libp2p_disc_impl.NewRoutingDiscovery(dht)

	self.PeerID = p2pHost.ID()

	subLogger := logger.With().Str("hostID", p2pHost.ID().Pretty()).Logger()
	// has to save the private key for host
	h := &HostV2{
		h:      p2pHost,
		pubsub: pubsub,
		self:   *self,
		priKey: priKey,
		logger: &subLogger,

		dht:  dht,
		boot: dht,
		adv:  loggingAdvertiser{disc},
		disc: disc,

		groupDisc: make(map[p2p.GroupID]*groupDiscovery),
	}

	h.logger.Debug().
		Str("port", self.Port).
		Str("id", p2pHost.ID().Pretty()).
		Str("addr", listenAddr.String()).
		Msg("HostV2 is up!")

	return h
}

// AddrAdder is a libp2p address factory that makes the host advertise
// arbitrary Multiaddr as its endpoint addresses,
// in addition to the discovered ones.
type AddrAdder interface {
	// Start/Stop make the host start/stop advertising the given multiaddr.
	Start(addr ma.Multiaddr)
	Stop(addr ma.Multiaddr)

	// AddrFactory is the address factory to pass to the host ctor.
	// Wrap it in a "github.com/libp2p/go-libp2p".AddrFactory option.
	AddrsFactory([]ma.Multiaddr) []ma.Multiaddr
}

type addrAdderMap = map[ /*(*Multiaddr).Bytes()*/ string] /*ignored*/ bool

type addrAdder struct {
	mtx   sync.RWMutex
	addrs addrAdderMap
}

func NewAddrAdder() *addrAdder {
	return &addrAdder{addrs: make(addrAdderMap)}
}

func (a *addrAdder) Start(addr ma.Multiaddr) {
	a.mtx.Lock()
	defer a.mtx.Unlock()
	a.addrs[string(addr.Bytes())] = true
}

func (a *addrAdder) Stop(addr ma.Multiaddr) {
	a.mtx.Lock()
	defer a.mtx.Unlock()
	delete(a.addrs, string(addr.Bytes()))
}

func (a *addrAdder) AddrsFactory(in []ma.Multiaddr) (out []ma.Multiaddr) {
	addrs := make(addrAdderMap)
	for _, addr := range in {
		addrs[string(addr.Bytes())] = true
	}
	out = append(in[:0:0], in...)
	for addrBin, _ := range a.addrs {
		if _, ok := addrs[addrBin]; !ok {
			addrBytes := []byte(addrBin)
			addr, err := ma.NewMultiaddrBytes(addrBytes)
			if err != nil {
				utils.Logger().Warn().Err(err).Bytes("addr_bytes", addrBytes).
					Msg("cannot create multiaddr from bytes")
			} else {
				out = append(out, addr)
			}
		}
	}
	return out
}

// GetID returns ID.Pretty
func (host *HostV2) GetID() libp2p_peer.ID {
	return host.h.ID()
}

// GetSelfPeer gets self peer
func (host *HostV2) GetSelfPeer() p2p.Peer {
	return host.self
}

// Close closes the host
func (host *HostV2) Close() error {
	if err := host.dht.Close(); err != nil {
		host.logger.Warn().Err(err).Msg("cannot close DHT")
	}
	return host.h.Close()
}

// GetP2PHost returns the p2p.Host
func (host *HostV2) GetP2PHost() libp2p_host.Host {
	return host.h
}

// GetPeerCount ...
func (host *HostV2) GetPeerCount() int {
	return host.h.Peerstore().Peers().Len()
}

// ConnectHostPeer connects to peer host
func (host *HostV2) ConnectHostPeer(peer p2p.Peer) {
	ctx := context.Background()
	addr := fmt.Sprintf("/ip4/%s/tcp/%s/ipfs/%s", peer.IP, peer.Port, peer.PeerID.Pretty())
	peerAddr, err := ma.NewMultiaddr(addr)
	if err != nil {
		host.logger.Error().Err(err).Interface("peer", peer).Msg("ConnectHostPeer")
		return
	}
	peerInfo, err := libp2p_peer.AddrInfoFromP2pAddr(peerAddr)
	if err != nil {
		host.logger.Error().Err(err).Interface("peer", peer).Msg("ConnectHostPeer")
		return
	}
	if err := host.h.Connect(ctx, *peerInfo); err != nil {
		host.logger.Warn().Err(err).Interface("peer", peer).Msg("can't connect to peer")
	} else {
		host.logger.Info().Interface("node", *peerInfo).Msg("connected to peer host")
	}
}

// DiscoverGroup starts group discovery process.
// groupID is the identifier of the group whose member to discover.
// limit is the desired number of members.
// cooldown is the time between successive discovery rounds.
func (host *HostV2) DiscoverGroup(
	groupID p2p.GroupID, limit int, cooldown time.Duration,
) {
	host.lock.Lock()
	defer host.lock.Unlock()
	if gd, ok := host.groupDisc[groupID]; ok {
		// Found; just change the params and exit
		gd.limit = limit
		gd.cooldown = cooldown
		return
	}
	quit := make(chan struct{})
	groupIDBase64 := base64.StdEncoding.EncodeToString([]byte(groupID))
	logger := host.logger.With().Str("group_id", groupIDBase64).Int("limit", limit).Logger()
	gd := &groupDiscovery{
		groupID:  groupID,
		limit:    limit,
		cooldown: cooldown,
		quit:     quit,
		waiters:  make(chan chan []p2p.Peer),
	}
	host.groupDisc[groupID] = gd
	go gd.discoverLoop(host.disc, host.h.Connect, logger)
}

type connResult struct {
	ai  libp2p_peer.AddrInfo
	err error
}

func (gd *groupDiscovery) discoverLoop(
	disc discoverer,
	connect func(context.Context, libp2p_peer.AddrInfo) error,
	logger zerolog.Logger,
) {
	logger.Info().Msg("starting group discovery")
	timer := time.NewTimer(0) // first round occurs immediately
	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		cancel()
		timer.Stop()
		logger.Info().Msg("group discovery stopped")
	}()
	var (
		foundCh       <-chan libp2p_peer.AddrInfo
		found         []p2p.Peer
		numConnecting int = -1
	)
	connResults := make(chan connResult)
	for {
		select {
		case _, ok := <-gd.quit:
			if !ok {
				return
			}
		case <-timer.C:
			limOpt := libp2p_disc.Limit(gd.limit)
			var err error
			foundCh, err = disc.FindPeers(ctx, string(gd.groupID), limOpt)
			if err != nil {
				logger.Warn().Err(err).Msg("cannot discover group members")
				foundCh = nil
				timer.Reset(gd.cooldown)
			}
			numConnecting = 0
		case ai, ok := <-foundCh:
			if !ok {
				foundCh = nil
				break
			}
			if len(ai.Addrs) == 0 {
				// nascent peer; ignore
				break
			}
			go func() { connResults <- connResult{ai, connect(ctx, ai)} }()
			numConnecting++
		case r := <-connResults:
			numConnecting--
			logger := logger.With().Interface("addrinfo", r.ai).Logger()
			if r.err != nil {
				logger.Warn().Err(r.err).Msg("cannot connect to group member")
				break
			}
			peer, err := addrInfoToPeer(r.ai)
			if err != nil {
				logger.Warn().Err(err).
					Msg("cannot convert group member addrinfo into peer")
				break
			}
			found = append(found, peer)
		}
		if foundCh == nil && numConnecting == 0 {
			numConnecting = -1
			gd.found = found
			found = nil
		sendLoop:
			for {
				select {
				case waiter := <-gd.waiters:
					go func(found []p2p.Peer) { waiter <- found }(gd.found)
				default:
					break sendLoop
				}
			}
			timer.Reset(gd.cooldown)
		}
	}
}
func addrInfoToPeer(ai libp2p_peer.AddrInfo) (p2p.Peer, error) {
	logger := utils.Logger().With().Interface("addrinfo", ai).Logger()
	var (
		maxScope     ipScope = -1
		maxScopeIP   net.IP
		maxScopePort uint16
	)
	for _, addr := range ai.Addrs {
		logger := logger.With().Str("multiaddr", addr.String()).Logger()
		ip, port, err := getIPAddrAndPortFromMultiaddr(addr)
		if err != nil {
			logger.Warn().Err(err).Msg("cannot convert multiaddr, ignoring")
			continue
		}
		logger = logger.With().
			IPAddr("peer_ip", ip).
			Uint16("peer_port", port).
			Logger()
		if ip.IsMulticast() || ip.IsUnspecified() {
			// not unicast; skip
			continue
		}
		scope := ipScopeForAddr(ip)
		if scope > maxScope {
			maxScope = scope
			maxScopeIP = ip
			maxScopePort = port
		}
	}
	if maxScope == -1 {
		return p2p.Peer{}, errors.Errorf("no valid address found in %s", ai)
	}
	peer := p2p.Peer{
		IP:     maxScopeIP.String(),
		Port:   fmt.Sprint(maxScopePort),
		Addrs:  ai.Addrs,
		PeerID: ai.ID,
	}
	return peer, nil
}

func getIPAddrAndPortFromMultiaddr(
	addr ma.Multiaddr,
) (ip net.IP, port uint16, err error) {
	logger := utils.Logger().With().Str("multiaddr", addr.String()).Logger()
	ma.ForEach(addr, func(c ma.Component) bool {
		switch c.Protocol().Code {
		case ma.P_IP4:
			logger := logger.With().Str("peer_ip", c.Value()).Logger()
			ip2 := net.ParseIP(c.Value())
			if ip2 == nil {
				logger.Debug().Msg("invalid IP address")
				break
			}
			ip = ip2
		case ma.P_TCP:
			logger := logger.With().Str("peer_port", c.Value()).Logger()
			if p, err := strconv.ParseUint(c.Value(), 10, 16); err != nil {
				logger.Debug().Err(err).Msg("invalid TCP port number")
			} else {
				port = uint16(p)
			}
		}
		if len(ip) > 0 && port > 0 {
			err = nil
		}
		return true // continue ForEach
	})
	err = errors.New("no IP address/port number found")
	switch {
	case len(ip) == 0:
		logger.Debug().Msg("IP address not found in multiaddr")
		return nil, 0, errors.New("IP address not found in multiaddr")
	case port == 0:
		logger.Debug().Msg("port number not found in multiaddr")
		return nil, 0, errors.New("IP address not found in multiaddr")
	}
	return ip, port, nil
}

// StopDiscoverGroup stops group discovery process
func (host *HostV2) StopDiscoverGroup(groupID p2p.GroupID) {
	host.lock.Lock()
	defer host.lock.Unlock()
	if gd, ok := host.groupDisc[groupID]; ok {
		close(gd.quit)
		delete(host.groupDisc, groupID)
	}
}

// GroupPeers returns the peers discovered for the given group.
// This needs to be called after calling DiscoverGroup.
func (host *HostV2) GroupPeers(id p2p.GroupID) (peers []p2p.Peer, err error) {
	host.lock.Lock()
	defer host.lock.Unlock()
	gd, ok := host.groupDisc[id]
	if !ok {
		return nil, errors.Errorf("group ID %v not found", id)
	}
	return gd.found, nil
}

// WaitForGroupPeers waits for a peer discovery round to finish and returns
// its result.
func (host *HostV2) WaitForGroupPeers(
	ctx context.Context, id p2p.GroupID,
) (peers []p2p.Peer, err error) {
	var (
		gd *groupDiscovery
		ok bool
	)
	func() {
		host.lock.Lock()
		defer host.lock.Unlock()
		gd, ok = host.groupDisc[id]
	}()
	if !ok {
		return nil, errors.Errorf("group ID %v not found", id)
	}
	waiter := make(chan []p2p.Peer, 1)
	waiters := gd.waiters
	go func() { waiters <- waiter }()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case result := <-waiter:
		return result, nil
	}
}

// JoinGroup joins the given broadcast group.
// TODO ek – this should be rolled into group receivers
func (host *HostV2) JoinGroup(id p2p.GroupID) {
	libp2p_disc_impl.Advertise(context.Background(), host.adv, string(id))
}

type ipScope int

const (
	ipScopeNodeLocal         = 0
	ipScopeInterfaceLocal    = 1
	ipScopeLinkLocal         = 2
	ipScopeRealmLocal        = 3
	ipScopeAdminLocal        = 4
	ipScopeSiteLocal         = 5
	ipScopeOrganizationLocal = 8
	ipScopeGlobal            = 14
)

var ipScopeNames = map[ipScope]string{
	ipScopeNodeLocal:         "node-local",
	ipScopeInterfaceLocal:    "interface-local",
	ipScopeLinkLocal:         "link-local",
	ipScopeRealmLocal:        "realm-local",
	ipScopeAdminLocal:        "admin-local",
	ipScopeSiteLocal:         "site-local",
	ipScopeOrganizationLocal: "organization-local",
	ipScopeGlobal:            "global",
}

func (s ipScope) String() string {
	name, ok := ipScopeNames[s]
	if !ok {
		return fmt.Sprint(int(s))
	}
	return name
}

func mustParseCIDR(s string) *net.IPNet {
	_, ipNet, err := net.ParseCIDR(s)
	if err != nil {
		panic(err)
	}
	return ipNet
}

var scopedPrefixes = []struct {
	net   *net.IPNet
	scope ipScope
}{
	// Localhost
	{mustParseCIDR("127.0.0.1/32"), ipScopeNodeLocal},
	{mustParseCIDR("::1/128"), ipScopeNodeLocal},
	// Link-local
	{mustParseCIDR("169.254.0.0/16"), ipScopeInterfaceLocal},
	{mustParseCIDR("fe80::/10"), ipScopeLinkLocal},
	// RFC 1918
	{mustParseCIDR("10.0.0.0/8"), ipScopeSiteLocal},
	{mustParseCIDR("172.16.0.0/12"), ipScopeSiteLocal},
	{mustParseCIDR("192.168.0.0/16"), ipScopeSiteLocal},
	// RFC 6598 CGN
	{mustParseCIDR("100.64.0.0/10"), ipScopeOrganizationLocal},
}

func ipScopeForAddr(ip net.IP) ipScope {
	for _, sp := range scopedPrefixes {
		if sp.net.Contains(ip) {
			return sp.scope
		}
	}
	return ipScopeGlobal
}
