package explorer

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"os"
	"strconv"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/gorilla/mux"
	libp2p_peer "github.com/libp2p/go-libp2p-peer"

	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	"github.com/harmony-one/harmony/core/types"
	common2 "github.com/harmony-one/harmony/internal/common"
	"github.com/harmony-one/harmony/internal/ctxerror"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
)

// Constants for explorer service.
const (
	monitoringServicePortDifference = 5000
)

// HTTPrror is an HTTP error.
type HTTPError struct {
	Code int
	Msg  string
}

//
type ConnectionsLog struct {
	Time int
	ConnectionsNumber int
}

// Service is the struct for monitoring service.
type Service struct {
	router            *mux.Router
	IP                string
	Port              string
	GetNodeIDs        func() []libp2p_peer.ID
	storage           *MetricsStorage
	server            *rpc.Server
	messageChan       chan *msg_pb.Message
}

// New returns monitoring service.
func New(selfPeer *p2p.Peer, GetNodeIDs func() []libp2p_peer.ID) *Service {
	return &Service{
		IP:                selfPeer.IP,
		Port:              selfPeer.Port,
		GetNodeIDs:        GetNodeIDs,
	}
}

// StartService starts monitoring service.
func (s *Service) StartService() {
	utils.Logger().Info().Msg("Starting explorer service.")
	s.Init(true)
	s.server = s.Run()
}

// StopService shutdowns monitoring service.
func (s *Service) StopService() {
	utils.Logger().Info().Msg("Shutting down monitoring service.")
	if err := s.server.Shutdown(context.Background()); err != nil {
		utils.Logger().Error().Err(err).Msg("Error when shutting down monitoring server")
	} else {
		utils.Logger().Info().Msg("Shutting down monitoring server successufully")
	}
}

// GetMonitoringServicePort returns the port serving monitorign service dashboard. This port is monitoringServicePortDifference less than the node port.
func GetMonitoringSerivcePort(nodePort string) string {
	if port, err := strconv.Atoi(nodePort); err == nil {
		return fmt.Sprintf("%d", port-monitoringServicePortDifference)
	}
	utils.Logger().Error().Msg("error on parsing.")
	return ""
}

// Init is to initialize for MonitoringService.
func (s *Service) Init(remove bool) {
	s.storage = GetStorageInstance(s.IP, s.Port, remove)
}

// Run is to run serving monitoring service.
func (s *Service) Run() *http.Server {
	// Init address.
	addr := net.JoinHostPort("", GetMonitoringServicePort(s.Port))

	s.router = mux.NewRouter()
	// Set up router for connections number.
	s.router.Path("/connectionsNumber").Queries("since", "{[0-9]*?}", "until", "{[0-9]*?}").HandlerFunc(s.GetMonitoringSerivceConnectionsNumber).Methods("GET")
	s.router.Path("/connectionsNumber").HandlerFunc(s.GetMonitoringSerivceConnectionsNumber)

	// Do serving now.
	utils.Logger().Info().Str("port", GetMonitoringServicePort(s.Port)).Msg("Listening")
	server := &http.Server{Addr: addr, Handler: s.router}
	go func() {
		if err := server.ListenAndServe(); err != nil {
			utils.Logger().Warn().Err(err).Msg("server.ListenAndServe()")
		}
	}()
	return server
}

// ReadConnectionsNumberFromDB returns a list of connections numbers to server connections number end-point.
func (s *Service) ReadConnectionsNumberFromDB(since, until uint) []uint {
	connectionsNumbers := []uint
	for i := since; i <= until; i++ {
		if i < 0 {
			connectionsNumbers = append(connectionsNumbers, nil)
			continue
		}
		key := GetConnectionsNumberKey(i)
		data, err := storage.db.Get([]byte(key))
		if err != nil {
			connectionsNumbers = append(connectionsNumbers, nil)
			continue
		}
		connectionsNumber := uint(0)
		if rlp.DecodeBytes(data, block) != nil {
			utils.Logger().Error().Msg("Error on getting from db")
			os.Exit(1)
		}
		connectionsNumbers = append(connectionsNumbers, connectionsNumber)
	}
	return connectionsNumbers
}

// GetMonitoringServiceConnectionsNumber serves end-point /connectionsNumber
func (s *Service) GetMonitoringSerivceConnectionsNumber(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	since := r.FormValue("since")
	until := r.FormValue("until")

	data := &Data{
		ConnectionsNumbers: []*ConnectionLog,
	}
	defer func() {
		if err := json.NewEncoder(w).Encode(data.Connections); err != nil {
			utils.Logger().Warn().Err(err).Msg("cannot JSON-encode connections")
		}
	}()
	var sinceInt int
	var err error
	if (since == "") {
		since = 0, err = nil
	} else {
		sinceInt, err = strconv.Atoi(since)
	}
	if err != nil {
		utils.Logger().Warn().Err(err).Str("since", since).Msg("invalid since parameter")
		return
	}
	var untilInt int
	if until == "" {
		untilInt = time.Now().Unix()
	} else {
		untilInt, err = strconv.Atoi(until)
	}
	if err != nil {
		utils.Logger().Warn().Err(err).Str("until", until).Msg("invalid until parameter")
		return
	}

	connectionsNumbers := s.ReadConnectionsNumbersDB(sinceInt, untilInt)
	for currentTime, connectionsNumber := range connectionsNumbers {
		data.ConnectionsNumbers = append(data.ConnectionsNumbers, ConnectionsLog{Time: currentTime, ConnectionsNumber: connectionsNumber})
	}

	return
}

// NotifyService notify service
func (s *Service) NotifyService(params map[string]interface{}) {
	return
}

// SetMessageChan sets up message channel to service.
func (s *Service) SetMessageChan(messageChan chan *msg_pb.Message) {
	s.messageChan = messageChan
}

// APIs for the services.
func (s *Service) APIs() []rpc.API {
	return nil
}
