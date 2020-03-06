package explorer

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/gorilla/mux"

	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	internal_common "github.com/harmony-one/harmony/internal/common"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/shard"
)

// Constants for explorer service.
const (
	explorerPortDifference = 4000
	defaultPageSize        = "1000"
	maxAddresses           = 100000
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
	shardID     uint32
	messageChan chan *msg_pb.Message
	GetBalance  func(common.Address) (*big.Int, error)
	ReadState   func(epoch *big.Int) (shard.State, error)
}

// New returns explorer service.
func New(selfPeer *p2p.Peer, shard uint32, GetAddressBalance func(common.Address) (*big.Int, error),
	ReadShardState func(epoch *big.Int) (shard.State, error)) *Service {
	return &Service{
		IP:         selfPeer.IP,
		Port:       selfPeer.Port,
		shardID:    shard,
		GetBalance: GetAddressBalance,
		ReadState:  ReadShardState,
	}
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
	if s.GetBalance == nil || s.ReadState == nil {
		return
	}
	ticker := time.NewTicker(time.Minute)
	prevEpoch := int64(-1)
	utils.Logger().Info().Msg("start updating top addresses")
	for range ticker.C {
		topAddresses := make([]AddrBalance, 0)
		bytes, err := s.Storage.GetDB().Get([]byte(EpochPrefix), nil)
		if err != nil {
			utils.Logger().Error().Err(err).Msg("error reading epoch")
			continue
		}
		epoch := big.NewInt(0)
		if err = rlp.DecodeBytes(bytes, &epoch); err != nil {
			utils.Logger().Error().Err(err).Msg("cannot decode epoch")
			continue
		}
		for curEpoch := prevEpoch + 1; curEpoch < epoch.Int64(); curEpoch++ {
			state, err := s.ReadState(big.NewInt(curEpoch))
			if err != nil {
				utils.Logger().Error().Err(err).Msg("error reading state")
				continue
			}
			for _, committee := range state {
				if committee.ShardID == s.shardID {
					for _, validator := range committee.NodeList {
						addr, err := internal_common.AddressToBech32(validator.EcdsaAddress)
						if err != nil {
							utils.Logger().Error().Err(err).Msg("error conversion to bech32")
							continue
						}
						if err = s.Storage.DumpAddress(addr); err != nil {
							utils.Logger().Error().Err(err).Msg("error dumping address")
						}
					}
				}
			}
		}
		prevEpoch = epoch.Int64()
		prefix := ""
		for {
			addresses, err := s.Storage.GetAddresses(maxAddresses, prefix)
			if err != nil {
				utils.Logger().Error().Err(err).Msg("addresses fetch error")
				break
			}
			for _, address := range addresses {
				addr := internal_common.ParseAddr(address)
				balance, err := s.GetBalance(addr)
				if err != nil {
					utils.Logger().Error().Err(err).Msg("balance fetch error")
					continue
				}
				key := AddrBalance{Address: address, Balance: balance}
				topAddresses = append(topAddresses, key)
				sort.Slice(topAddresses, func(i, j int) bool {
					return topAddresses[i].Balance.Cmp(topAddresses[j].Balance) < 1
				})
				if len(topAddresses) > TopAddrLen {
					topAddresses = topAddresses[:TopAddrLen-1]
				}
			}
			if len(addresses) < maxAddresses {
				encoded, err := rlp.EncodeToBytes(topAddresses)
				if err != nil {
					utils.Logger().Error().Err(err).Msg("top addresses encoding error")
					break
				}
				if err = s.Storage.GetDB().Put([]byte(TopPrefix), encoded, nil); err != nil {
					utils.Logger().Error().Err(err).Msg("top addresses db dump error")
				}
				break
			}
			prefix = addresses[len(addresses)-1]
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
