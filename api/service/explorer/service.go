package explorer

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/gorilla/mux"
	libp2p_peer "github.com/libp2p/go-libp2p-peer"

	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	"github.com/harmony-one/harmony/consensus/reward"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/numeric"
	"github.com/harmony-one/harmony/p2p"
)

// Constants for explorer service.
const (
	explorerPortDifference = 4000
	totalSupply            = 12600000000
)

// HTTPError is an HTTP error.
type HTTPError struct {
	Code int
	Msg  string
}

// Service is the struct for explorer service.
type Service struct {
	router            *mux.Router
	IP                string
	Port              string
	GetNodeIDs        func() []libp2p_peer.ID
	ShardID           uint32
	Storage           *Storage
	server            *http.Server
	messageChan       chan *msg_pb.Message
	GetAccountBalance func(common.Address) (*big.Int, error)
}

// New returns explorer service.
func New(selfPeer *p2p.Peer, shardID uint32, GetNodeIDs func() []libp2p_peer.ID, GetAccountBalance func(common.Address) (*big.Int, error)) *Service {
	return &Service{
		IP:                selfPeer.IP,
		Port:              selfPeer.Port,
		ShardID:           shardID,
		GetNodeIDs:        GetNodeIDs,
		GetAccountBalance: GetAccountBalance,
	}
}

// ServiceAPI is rpc api.
type ServiceAPI struct {
	Service *Service
}

// NewServiceAPI returns explorer service api.
func NewServiceAPI(explorerService *Service) *ServiceAPI {
	return &ServiceAPI{Service: explorerService}
}

// StartService starts explorer service.
func (s *Service) StartService() {
	utils.Logger().Info().Msg("Starting explorer service.")
	s.Init(true)
	s.server = s.Run()
}

// StopService shutdowns explorer service.
func (s *Service) StopService() {
	utils.Logger().Info().Msg("Shutting down explorer service.")
	if err := s.server.Shutdown(context.Background()); err != nil {
		utils.Logger().Error().Err(err).Msg("Error when shutting down explorer server")
	} else {
		utils.Logger().Info().Msg("Shutting down explorer server successufully")
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
func (s *Service) Init(remove bool) {
	s.Storage = GetStorageInstance(s.IP, s.Port, remove)
}

// Run is to run serving explorer.
func (s *Service) Run() *http.Server {
	// Init address.
	addr := net.JoinHostPort("", GetExplorerPort(s.Port))

	s.router = mux.NewRouter()

	// Set up router for node count.
	s.router.Path("/circulating-supply").Queries().HandlerFunc(s.GetCirculatingSupply).Methods("GET")
	s.router.Path("/circulating-supply").HandlerFunc(s.GetCirculatingSupply)

	// Set up router for node count.
	s.router.Path("/total-supply").Queries().HandlerFunc(s.GetTotalSupply).Methods("GET")
	s.router.Path("/total-supply").HandlerFunc(s.GetTotalSupply)

	// Do serving now.
	utils.Logger().Info().Str("port", GetExplorerPort(s.Port)).Msg("Listening")
	server := &http.Server{Addr: addr, Handler: s.router}
	go func() {
		if err := server.ListenAndServe(); err != nil {
			utils.Logger().Warn().Err(err).Msg("server.ListenAndServe()")
		}
	}()
	return server
}

// GetExplorerNodeCount serves /nodes end-point.
func (s *Service) GetExplorerNodeCount(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(len(s.GetNodeIDs())); err != nil {
		utils.Logger().Warn().Msg("cannot JSON-encode node count")
		w.WriteHeader(http.StatusInternalServerError)
	}
}

// GetExplorerNodeCount rpc end-point.
func (s *ServiceAPI) GetExplorerNodeCount(ctx context.Context) int {
	return len(s.Service.GetNodeIDs())
}

// GetExplorerShard serves /shard end-point.
func (s *Service) GetExplorerShard(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var nodes []Node
	for _, nodeID := range s.GetNodeIDs() {
		nodes = append(nodes, Node{
			ID: libp2p_peer.IDB58Encode(nodeID),
		})
	}
	if err := json.NewEncoder(w).Encode(Shard{Nodes: nodes}); err != nil {
		utils.Logger().Warn().Msg("cannot JSON-encode shard info")
		w.WriteHeader(http.StatusInternalServerError)
	}
}

// GetExplorerShard rpc end-point.
func (s *ServiceAPI) GetExplorerShard(ctx context.Context) *Shard {
	var nodes []Node
	for _, nodeID := range s.Service.GetNodeIDs() {
		nodes = append(nodes, Node{
			ID: libp2p_peer.IDB58Encode(nodeID),
		})
	}
	return &Shard{Nodes: nodes}
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

// NotifyService notify service.
func (s *Service) NotifyService(params map[string]interface{}) {
	return
}

// SetMessageChan sets up message channel to service.
func (s *Service) SetMessageChan(messageChan chan *msg_pb.Message) {
	s.messageChan = messageChan
}

// APIs for the services.
func (s *Service) APIs() []rpc.API {
	return []rpc.API{
		{
			Namespace: "explorer",
			Version:   "1.0",
			Service:   NewServiceAPI(s),
			Public:    true,
		},
	}
}
