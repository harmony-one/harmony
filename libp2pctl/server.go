package libp2pctl

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"sync"

	"github.com/gorilla/mux"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/pkg/errors"

	"github.com/harmony-one/harmony/internal/utils"
)

// Shorthand aliases
var (
	url64Encode = base64.RawURLEncoding.EncodeToString
	url64Decode = base64.RawURLEncoding.DecodeString
)

// EndpointID is a libp2p connection endpoint.
type EndpointID struct {
	ID   peer.ID
	Addr ma.Multiaddr
}

func EndpointIDFromURLVar(v string) (EndpointID, error) {
	idma := strings.SplitN(v, "@", 2)
	if len(idma) != 2 {
		return EndpointID{}, errors.New("no ID-multiaddr separator")
	}
	id, err := peer.IDB58Decode(idma[0])
	if err != nil {
		return EndpointID{}, errors.Wrap(err, "cannot parse peer ID")
	}
	mab, err := url64Decode(idma[1])
	if err != nil {
		return EndpointID{}, errors.Wrap(err, "cannot decode multiaddr")
	}
	ma, err := ma.NewMultiaddrBytes(mab)
	if err != nil {
		return EndpointID{}, errors.Wrap(err, "cannot parse multiaddr")
	}
	return EndpointID{id, ma}, nil
}

// URLVar returns the encoded form of the receiver, suitable for use in a URL.
func (id EndpointID) URLVar() string {
	return id.ID.String() + "@" + url64Encode(id.Addr.Bytes())
}

// Key returns a struct usable as a map key.
func (ep EndpointID) Key() EndpointKey {
	return EndpointKey{ep.ID, string(ep.Addr.Bytes())}
}

// EndpointKey is a libp2p connection endpoint, usable as a map key.
type EndpointKey struct {
	id   peer.ID
	addr string
}

// ID returns the endpoint ID.
func (k EndpointKey) ID() (EndpointID, error) {
	addr, err := ma.NewMultiaddrBytes([]byte(k.addr))
	if err != nil {
		return EndpointID{}, errors.Wrap(err, "cannot convert multiaddr")
	}
	return EndpointID{k.id, addr}, nil
}

// ConnID is the identifier of a connection.
type ConnID struct {
	Local, Remote EndpointID
}

// Key returns a struct usable as a map key.
func (n ConnID) Key() ConnKey {
	return ConnKey{n.Local.Key(), n.Remote.Key()}
}

// ConnKey is a connection identifier, usable as a map key.
type ConnKey struct {
	local, remote EndpointKey
}

// ID returns the connection ID.
func (k ConnKey) ID() (ConnID, error) {
	local, err := k.local.ID()
	if err != nil {
		return ConnID{}, errors.Wrap(err, "cannot convert local key into ID")
	}
	remote, err := k.remote.ID()
	if err != nil {
		return ConnID{}, errors.Wrap(err, "cannot convert remote key into ID")
	}
	return ConnID{local, remote}, nil
}

// ConnIDFromConn returns the name of the given connection.
func ConnIDFromConn(conn network.Conn) ConnID {
	return ConnID{
		Local:  EndpointID{conn.LocalPeer(), conn.LocalMultiaddr()},
		Remote: EndpointID{conn.RemotePeer(), conn.RemoteMultiaddr()},
	}
}

// Instance is a libp2pctl instance.
type Instance struct {
	host     host.Host
	rtr      *mux.Router
	conns    map[ConnKey]network.Conn
	connsMtx sync.RWMutex
	notifiee *notifiee
}

// New returns a libp2pctl instance associated with the given libp2p host.
func New(host host.Host) *Instance {
	rtr := mux.NewRouter()
	rtrConn := rtr.Path("/conn/{local}/{remote}").Subrouter()
	inst := &Instance{
		host:  host,
		rtr:   rtr,
		conns: map[ConnKey]network.Conn{},
	}
	for _, conn := range host.Network().Conns() {
		inst.addConn(conn)
	}
	inst.notifiee = &notifiee{inst}
	host.Network().Notify(inst.notifiee)

	rtr.Methods("GET").Path("/addrs").HandlerFunc(inst.getAddrs)
	rtr.Methods("GET").Path("/conns").HandlerFunc(inst.getConns)
	rtr.Methods("POST").Path("/conns").HandlerFunc(inst.postConns)
	rtrConn.Methods("GET").Path("").HandlerFunc(inst.getConn).Name("conn")
	rtrConn.Methods("DELETE").Path("").HandlerFunc(inst.deleteConn)
	rtrConn.Methods("GET").Path("/stat").HandlerFunc(inst.getConnStat)

	return inst
}

func (inst *Instance) addConn(conn network.Conn) {
	inst.connsMtx.Lock()
	defer inst.connsMtx.Unlock()
	inst.conns[ConnIDFromConn(conn).Key()] = conn
}

func (inst *Instance) delConn(conn network.Conn) {
	inst.connsMtx.Lock()
	defer inst.connsMtx.Unlock()
	delete(inst.conns, ConnIDFromConn(conn).Key())
}

// Handler returns an HTTP request handler of the libp2pctl instance.
func (inst *Instance) Handler() http.Handler {
	return inst.rtr
}

func (inst *Instance) getAddrs(w http.ResponseWriter, r *http.Request) {
	if err := json.NewEncoder(w).Encode(inst.host.Addrs()); err != nil {
		utils.Logger().Err(err).Msg("cannot serve libp2p addresses")
	}
}

func (inst *Instance) connNames() (connNames []ConnID) {
	for _, conn := range inst.host.Network().Conns() {
		connNames = append(connNames, ConnIDFromConn(conn))
	}
	return connNames
}

type connJSON struct {
	LocalPeer  peer.ID
	LocalAddr  ma.Multiaddr
	RemotePeer peer.ID
	RemoteAddr ma.Multiaddr
}

func newConnJSON(conn network.Conn) connJSON {
	return connJSON{
		LocalPeer:  conn.LocalPeer(),
		LocalAddr:  conn.LocalMultiaddr(),
		RemotePeer: conn.RemotePeer(),
		RemoteAddr: conn.RemoteMultiaddr(),
	}
}

type connEntryJSON struct {
	ID         ConnID
	URL        string
	connJSON
}

type connTableJSON []connEntryJSON

func (inst *Instance) connTableJSON() (connTableJSON, error) {
	resp := connTableJSON{}
	for key, conn := range inst.conns {
		id, err := key.ID()
		if err != nil {
			return nil, err
		}
		url, err := inst.rtr.Get("conn").URL(
			"local", id.Local.URLVar(),
			"remote", id.Remote.URLVar(),
		)
		if err != nil {
			return nil, err
		}
		resp = append(resp, connEntryJSON{
			ID:       id,
			URL:      url.String(),
			connJSON: newConnJSON(conn),
		})
	}
	return resp, nil
}

func (inst *Instance) getConns(w http.ResponseWriter, r *http.Request) {
	conns, err := inst.connTableJSON()
	if err != nil {
		utils.Logger().Err(err).Msg("cannot build connection table")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if err := json.NewEncoder(w).Encode(conns); err != nil {
		utils.Logger().Err(err).Msg("cannot serve libp2p connections")
	}
	return
}

func (inst *Instance) lookupConn(r *http.Request) (network.Conn, error) {
	vars := mux.Vars(r)
	if vars == nil {
		return nil, errors.New("cannot get route vars")
	}
	local, err := EndpointIDFromURLVar(vars["local"])
	if err != nil {
		return nil, errors.Wrapf(err, "invalid local endpoint %#v", vars["local"])
	}
	remote, err := EndpointIDFromURLVar(vars["local"])
	if err != nil {
		return nil, errors.Wrapf(err, "invalid remote endpoint %#v", vars["remote"])
	}
	inst.connsMtx.RLock()
	defer inst.connsMtx.RUnlock()
	return inst.conns[ConnID{local, remote}.Key()], nil
}

func (inst *Instance) getConn(w http.ResponseWriter, r *http.Request) {
	conn, err := inst.lookupConn(r)
	switch {
	case err != nil:
		utils.Logger().Err(err).Msg("cannot retrieve connection")
		w.WriteHeader(http.StatusInternalServerError)
	case conn == nil:
		w.WriteHeader(http.StatusNotFound)
	default:
		if err := json.NewEncoder(w).Encode(conn); err != nil {
			utils.Logger().Err(err).Interface("conn", conn).
				Msg("cannot write connection details")
		}
	}
}

func (inst *Instance) deleteConn(w http.ResponseWriter, r *http.Request) {
	conn, err := inst.lookupConn(r)
	switch {
	case err != nil:
		utils.Logger().Err(err).Msg("cannot retrieve connection")
		w.WriteHeader(http.StatusInternalServerError)
	case conn == nil:
		w.WriteHeader(http.StatusNotFound)
	default:
		if err := conn.Close(); err != nil {
			utils.Logger().Err(err).
				Interface("conn", ConnIDFromConn(conn)).
				Msg("cannot close connection")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}
}

func (inst *Instance) postConns(w http.ResponseWriter, r *http.Request) {
	switch r.Header.Get("Content-Type") {
	case "", "application/json":
	default:
		w.WriteHeader(http.StatusUnsupportedMediaType)
		return
	}
	ctx := r.Context()
	var pi peer.AddrInfo
	if err := json.NewDecoder(r.Body).Decode(&pi); err != nil {
		utils.Logger().Err(err).Msg("cannot parse request body")
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	utils.Logger().Info().Interface("remote", pi).
		Msg("connecting to libp2p peer per libp2pctl request")
	if err := inst.host.Connect(ctx, pi); err != nil {
		utils.Logger().Err(err).Msg("cannot connect to libp2p peer")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
}

func (inst *Instance) getConnStat(w http.ResponseWriter, r *http.Request) {
	conn, err := inst.lookupConn(r)
	switch {
	case err != nil:
		utils.Logger().Err(err).Msg("cannot retrieve connection")
		w.WriteHeader(http.StatusInternalServerError)
	case conn == nil:
		w.WriteHeader(http.StatusNotFound)
	default:
		if err := json.NewEncoder(w).Encode(conn.Stat()); err != nil {
			utils.Logger().Err(err).Msg("cannot serve connection stat")
		}
	}
}

type notifiee struct {
	inst *Instance
}

func (n *notifiee) Connected(_ network.Network, conn network.Conn)    { n.inst.addConn(conn) }
func (n *notifiee) Disconnected(_ network.Network, conn network.Conn) { n.inst.delConn(conn) }
func (n *notifiee) Listen(network.Network, ma.Multiaddr)              {}
func (n *notifiee) ListenClose(network.Network, ma.Multiaddr)         {}
func (n *notifiee) OpenedStream(network.Network, network.Stream)      {}
func (n *notifiee) ClosedStream(network.Network, network.Stream)      {}
