package explorer

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/ethereum/go-ethereum/rpc"
	"github.com/gorilla/mux"
	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	"github.com/harmony-one/harmony/api/service/syncing"
	"github.com/harmony-one/harmony/consensus/reward"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/numeric"
	"github.com/harmony-one/harmony/p2p"
)

// Constants for explorer service.
const (
	explorerPortDifference = 4000
	defaultPageSize        = "1000"
	maxAddresses           = 100000
	totalSupply            = 12600000000
)

// HTTPError is an HTTP error.
type HTTPError struct {
	Code int
	Msg  string
}

// Service is the struct for explorer service.
type Service struct {
	router      *mux.Router
	IP          string
	Port        string
	Storage     *Storage
	server      *http.Server
	messageChan chan *msg_pb.Message
	stateSync   *syncing.StateSync
	blockchain  *core.BlockChain
}

// New returns explorer service.
func New(selfPeer *p2p.Peer, ss *syncing.StateSync, bc *core.BlockChain) *Service {
	return &Service{IP: selfPeer.IP, Port: selfPeer.Port, stateSync: ss, blockchain: bc}
}

// StartService starts explorer service.
func (s *Service) StartService() {
	utils.Logger().Info().Msg("Starting explorer service.")
	s.Init()
	s.server = s.Run()
}

// StopService shutdowns explorer service.
func (s *Service) StopService() {
	utils.Logger().Info().Msg("Shutting down explorer service.")
	if err := s.server.Shutdown(context.Background()); err != nil {
		utils.Logger().Error().Err(err).Msg("Error when shutting down explorer server")
	} else {
		utils.Logger().Info().Msg("Shutting down explorer server successfully")
	}
}

// GetExplorerPort returns the port serving explorer dashboard. This port is explorerPortDifference less than the node port.
func GetExplorerPort(nodePort string) string {
	if port, err := strconv.Atoi(nodePort); err == nil {
		return fmt.Sprintf("%d", port-explorerPortDifference)
	}
	utils.Logger().Error().Msg("error on parsing.")
	return ""
}

// Init is to initialize for ExplorerService.
func (s *Service) Init() {
	s.Storage = GetStorageInstance(s.IP, s.Port)
}

// Run is to run serving explorer.
func (s *Service) Run() *http.Server {
	// Init address.
	addr := net.JoinHostPort("", GetExplorerPort(s.Port))

	s.router = mux.NewRouter()

	// Set up router for addresses.
	// Fetch addresses request, accepts parameter size: how much addresses to read,
	// parameter prefix: from which address prefix start
	s.router.Path("/addresses").Queries("size", "{[0-9]*?}", "prefix", "{[a-zA-Z0-9]*?}").HandlerFunc(s.GetAddresses).Methods("GET")
	s.router.Path("/addresses").HandlerFunc(s.GetAddresses)

	// Set up router for node count.
	s.router.Path("/circulating-supply").Queries().HandlerFunc(s.GetCirculatingSupply).Methods("GET")
	s.router.Path("/circulating-supply").HandlerFunc(s.GetCirculatingSupply)

	// Set up router for node count.
	s.router.Path("/total-supply").Queries().HandlerFunc(s.GetTotalSupply).Methods("GET")
	s.router.Path("/total-supply").HandlerFunc(s.GetTotalSupply)

	// Set up router for node health check.
	s.router.Path("/node-sync").Queries().HandlerFunc(s.GetNodeSync).Methods("GET")
	s.router.Path("/node-sync").HandlerFunc(s.GetNodeSync)

	// Do serving now.
	utils.Logger().Info().Str("port", GetExplorerPort(s.Port)).Msg("[Explorer] Server started.")
	server := &http.Server{
		Addr:         addr,
		Handler:      s.router,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}
	go func() {
		defer func() { utils.Logger().Debug().Msg("[Explorer] Server closed.") }()
		if err := server.ListenAndServe(); err != nil {
			utils.Logger().Warn().Err(err).Msg("[Explorer] Server error.")
		}
	}()
	return server
}

// GetAddresses serves end-point /addresses, returns size of addresses from address with prefix.
func (s *Service) GetAddresses(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	sizeStr := r.FormValue("size")
	prefix := r.FormValue("prefix")
	if sizeStr == "" {
		sizeStr = defaultPageSize
	}
	data := &Data{}
	defer func() {
		if err := json.NewEncoder(w).Encode(data.Addresses); err != nil {
			utils.Logger().Warn().Err(err).Msg("cannot JSON-encode addresses")
		}
	}()

	size, err := strconv.Atoi(sizeStr)
	if err != nil || size > maxAddresses {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	data.Addresses, err = s.Storage.GetAddresses(size, prefix)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		utils.Logger().Warn().Err(err).Msg("wasn't able to fetch addresses from storage")
		return
	}
}

// GetCirculatingSupply serves /circulating-supply end-point.
func (s *Service) GetCirculatingSupply(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	timestamp := time.Now().Unix()
	circulatingSupply := reward.PercentageForTimeStamp(timestamp).Mul(numeric.NewDec(totalSupply))
	if err := json.NewEncoder(w).Encode(circulatingSupply); err != nil {
		utils.Logger().Warn().Msg("cannot JSON-encode circulating supply")
		w.WriteHeader(http.StatusInternalServerError)
	}
}

// GetTotalSupply serves /total-supply end-point.
func (s *Service) GetTotalSupply(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(totalSupply); err != nil {
		utils.Logger().Warn().Msg("cannot JSON-encode total supply")
		w.WriteHeader(http.StatusInternalServerError)
	}
}

// GetNodeSync returns status code 500 if node is not in sync
func (s *Service) GetNodeSync(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	sync := !s.stateSync.IsOutOfSync(s.blockchain)
	if !sync {
		w.WriteHeader(http.StatusTeapot)
	}
	if err := json.NewEncoder(w).Encode(sync); err != nil {
		utils.Logger().Warn().Msg("cannot JSON-encode total supply")
		w.WriteHeader(http.StatusInternalServerError)
	}
}

// NotifyService notify service.
func (s *Service) NotifyService(params map[string]interface{}) {}

// SetMessageChan sets up message channel to service.
func (s *Service) SetMessageChan(messageChan chan *msg_pb.Message) {
	s.messageChan = messageChan
}

// APIs for the services.
func (s *Service) APIs() []rpc.API {
	return nil
}
