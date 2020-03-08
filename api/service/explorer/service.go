package explorer

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/ethereum/go-ethereum/trie"
	"github.com/gorilla/mux"

	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	"github.com/harmony-one/harmony/core/state"
	internal_common "github.com/harmony-one/harmony/internal/common"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
)

// Constants for explorer service.
const (
	explorerPortDifference = 4000
	defaultPageSize        = "1000"
	maxAddresses           = 100000
	TopAddrLen             = 20
	TopPrefix              = "top"
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
	State       func() (*state.DB, error)
}

// New returns explorer service.
func New(selfPeer *p2p.Peer, State func() (*state.DB, error)) *Service {
	return &Service{IP: selfPeer.IP, Port: selfPeer.Port, State: State}
}

// StartService starts explorer service.
func (s *Service) StartService() {
	utils.Logger().Info().Msg("Starting explorer service.")
	s.Init(true)
	s.server = s.Run()
	go s.UpdateTopAddresses()
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

	// Set up router for addresses.
	// Fetch addresses request, accepts parameter size: how much addresses to read,
	// parameter prefix: from which address prefix start
	s.router.Path("/addresses").Queries("size", "{[0-9]*?}", "prefix", "{[a-zA-Z0-9]*?}").HandlerFunc(s.GetAddresses).Methods("GET")
	s.router.Path("/addresses").HandlerFunc(s.GetAddresses)

	// Set up router for top addresses.
	s.router.Path("/top").Queries().HandlerFunc(s.GetTopAddresses).Methods("GET")
	s.router.Path("/top").HandlerFunc(s.GetTopAddresses)

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
			w.WriteHeader(http.StatusInternalServerError)
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

// UpdateTopAddresses updates 20 top addresses by balance from storage each 10 minutes.
func (s *Service) UpdateTopAddresses() {
	ticker := time.NewTicker(10 * time.Minute)
	utils.Logger().Info().Msg("start updating top addresses")
	for range ticker.C {
		db, err := s.State()
		if err != nil {
			utils.Logger().Error().Err(err).Msg("fetch state error")
		}
		it := trie.NewIterator(db.GetTrie().NodeIterator(nil))
		topAddresses := make([]AddrBalance, 0)
		for it.Next() {
			bytes := db.GetTrie().GetKey(it.Key)
			addr := common.BytesToAddress(bytes)
			address, err := internal_common.AddressToBech32(addr)
			if err != nil {
				utils.Logger().Error().Err(err).Msg("addresses to bech32 error")
			}
			balance := db.GetBalance(addr)
			key := AddrBalance{Address: address, Balance: balance}
			topAddresses = append(topAddresses, key)
			sort.Slice(topAddresses, func(i, j int) bool {
				return topAddresses[i].Balance.Cmp(topAddresses[j].Balance) > -1
			})
			if len(topAddresses) > TopAddrLen {
				topAddresses = topAddresses[:TopAddrLen-1]
			}
		}
		encoded, err := rlp.EncodeToBytes(topAddresses)
		if err != nil {
			utils.Logger().Error().Err(err).Msg("top addresses encoding error")
			break
		}
		if err = s.Storage.GetDB().Put([]byte(TopPrefix), encoded, nil); err != nil {
			utils.Logger().Error().Err(err).Msg("top addresses db dump error")
		}
	}
}

// GetTopAddresses serves end-point /top, returns top <= 20 addresses by balance with respective balance.
func (s *Service) GetTopAddresses(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	bytes, err := s.Storage.GetDB().Get([]byte(TopPrefix), nil)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		utils.Logger().Error().Err(err).Msg("cannot read addresses from db")
		return
	}
	topAddresses := make([]AddrBalance, 0)
	if err = rlp.DecodeBytes(bytes, &topAddresses); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		utils.Logger().Error().Err(err).Msg("cannot decode top addresses")
		return
	}
	if err = json.NewEncoder(w).Encode(topAddresses); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		utils.Logger().Warn().Err(err).Msg("cannot JSON-encode top addresses")
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
	return nil
}
