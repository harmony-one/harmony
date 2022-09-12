// TODO: refactor this whole module to v0 and v1
package explorer

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"path"
	"strconv"
	"time"

	"github.com/RoaringBitmap/roaring/roaring64"
	harmonyconfig "github.com/harmony-one/harmony/internal/configs/harmony"

	ethCommon "github.com/ethereum/go-ethereum/common"
	"github.com/gorilla/mux"
	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/hmy"
	"github.com/harmony-one/harmony/hmy/tracers"
	"github.com/harmony-one/harmony/internal/chain"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/numeric"
	"github.com/harmony-one/harmony/p2p"
	stakingReward "github.com/harmony-one/harmony/staking/reward"
)

// Constants for explorer service.
const (
	explorerPortDifference = 4000
	defaultPageSize        = "1000"
	maxAddresses           = 100000
	nodeSyncTolerance      = 5
)

// HTTPError is an HTTP error.
type HTTPError struct {
	Code int
	Msg  string
}

// Service is the struct for explorer service.
type Service struct {
	router        *mux.Router
	IP            string
	Port          string
	storage       *storage
	server        *http.Server
	messageChan   chan *msg_pb.Message
	blockchain    core.BlockChain
	backend       hmy.NodeAPI
	harmonyConfig *harmonyconfig.HarmonyConfig
}

// New returns explorer service.
func New(harmonyConfig *harmonyconfig.HarmonyConfig, selfPeer *p2p.Peer, bc core.BlockChain, backend hmy.NodeAPI) *Service {
	dbPath := defaultDBPath(selfPeer.IP, selfPeer.Port)
	storage, err := newStorage(harmonyConfig, bc, dbPath)
	if err != nil {
		utils.Logger().Fatal().Err(err).Msg("cannot open explorer DB")
	}
	return &Service{
		IP:         selfPeer.IP,
		Port:       selfPeer.Port,
		blockchain: bc,
		backend:    backend,
		storage:    storage,
	}
}

// Start starts explorer service.
func (s *Service) Start() error {
	utils.Logger().Info().Msg("Starting explorer service.")
	s.Init()
	s.server = s.Run()
	s.storage.Start()
	return nil
}

// Stop shutdowns explorer service.
func (s *Service) Stop() error {
	utils.Logger().Info().Msg("Shutting down explorer service.")
	if err := s.server.Shutdown(context.Background()); err != nil {
		utils.Logger().Error().Err(err).Msg("Error when shutting down explorer server")
	} else {
		utils.Logger().Info().Msg("Shutting down explorer server successfully")
	}
	s.storage.Close()
	return nil
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
func (s *Service) Init() {}

// Run is to run serving explorer.
func (s *Service) Run() *http.Server {
	// Init address.
	port := GetExplorerPort(s.Port)
	addr := net.JoinHostPort("", port)

	s.router = mux.NewRouter()

	// Set up router for addresses.
	// Fetch addresses request, accepts parameter size: how much addresses to read,
	// parameter prefix: from which address prefix start
	s.router.Path("/addresses").Queries("size", "{[0-9]*?}", "prefix", "{[a-zA-Z0-9]*?}").HandlerFunc(s.GetAddresses).Methods("GET")
	s.router.Path("/addresses").HandlerFunc(s.GetAddresses)
	s.router.Path("/height").HandlerFunc(s.GetHeight)

	// Set up router for supply info
	s.router.Path("/burn-addresses").Queries().HandlerFunc(s.GetInaccessibleAddressInfo).Methods("GET")
	s.router.Path("/burn-addresses").HandlerFunc(s.GetInaccessibleAddressInfo)
	s.router.Path("/circulating-supply").Queries().HandlerFunc(s.GetCirculatingSupply).Methods("GET")
	s.router.Path("/circulating-supply").HandlerFunc(s.GetCirculatingSupply)
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
	fmt.Printf("Started Explorer server at: %v:%v\n", s.IP, port)
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
	data.Addresses, err = s.storage.GetAddresses(size, oneAddress(prefix))
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		utils.Logger().Warn().Err(err).Msg("wasn't able to fetch addresses from storage")
		return
	}
}

type HeightResponse struct {
	S0 uint64 `json:"0,omitempty"`
	S1 uint64 `json:"1,omitempty"`
	S2 uint64 `json:"2,omitempty"`
	S3 uint64 `json:"3,omitempty"`
}

// GetHeight returns heights of current and beacon chains if needed.
func (s *Service) GetHeight(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	bc := s.backend.Blockchain()
	out := HeightResponse{}
	switch bc.ShardID() {
	case 0:
		out.S0 = s.backend.Blockchain().CurrentBlock().NumberU64()
	case 1:
		out.S0 = s.backend.Beaconchain().CurrentBlock().NumberU64()
		out.S1 = s.backend.Blockchain().CurrentBlock().NumberU64()
	case 2:
		out.S0 = s.backend.Beaconchain().CurrentBlock().NumberU64()
		out.S2 = s.backend.Blockchain().CurrentBlock().NumberU64()
	case 3:
		out.S0 = s.backend.Beaconchain().CurrentBlock().NumberU64()
		out.S3 = s.backend.Blockchain().CurrentBlock().NumberU64()
	}

	if err := json.NewEncoder(w).Encode(out); err != nil {
		utils.Logger().Warn().Err(err).Msg("cannot JSON-encode addresses")
	}
}

// GetNormalTxHashesByAccount get the normal transaction hashes by account
func (s *Service) GetNormalTxHashesByAccount(address string) ([]ethCommon.Hash, []TxType, error) {
	return s.storage.GetNormalTxsByAddress(address)
}

// GetStakingTxHashesByAccount get the staking transaction hashes by account
func (s *Service) GetStakingTxHashesByAccount(address string) ([]ethCommon.Hash, []TxType, error) {
	return s.storage.GetStakingTxsByAddress(address)
}

func (s *Service) GetTraceResultByHash(hash ethCommon.Hash) (json.RawMessage, error) {
	return s.storage.GetTraceResultByHash(hash)
}

// DumpTraceResult instruct the explorer storage to trace data in explorer DB
func (s *Service) DumpTraceResult(data *tracers.TraceBlockStorage) {
	s.storage.DumpTraceResult(data)
}

// DumpNewBlock instruct the explorer storage to dump block data in explorer DB
func (s *Service) DumpNewBlock(b *types.Block) {
	s.storage.DumpNewBlock(b)
}

// DumpCatchupBlock instruct the explorer storage to dump a catch up block in explorer DB
func (s *Service) DumpCatchupBlock(b *types.Block) {
	s.storage.DumpCatchupBlock(b)
}

// IsAvailable return whether the explorer db is available for now.
func (s *Service) IsAvailable() bool {
	return s.storage.available.IsSet()
}

// InaccessibleAddressInfo ..
type InaccessibleAddressInfo struct {
	EthAddress ethCommon.Address `json:"eth-address"`
	Address    string            `json:"address"`
	Balance    numeric.Dec       `json:"balance"`
	Nonce      uint64            `json:"nonce"`
}

// GetInaccessibleAddressInfo serves /burn-addresses end-point.
func (s *Service) GetInaccessibleAddressInfo(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	accInfos, err := chain.GetInaccessibleAddressInfo(s.blockchain)
	if err != nil {
		utils.Logger().Warn().Err(err).Msg("unable to fetch inaccessible addresses")
		w.WriteHeader(http.StatusInternalServerError)
	}
	display := make([]*InaccessibleAddressInfo, 0, len(accInfos))
	for _, acc := range accInfos {
		display = append(display, &InaccessibleAddressInfo{
			EthAddress: acc.EthAddress,
			Address:    acc.Address,
			Balance:    acc.Balance,
			Nonce:      acc.Nonce,
		})
	}
	if err := json.NewEncoder(w).Encode(accInfos); err != nil {
		utils.Logger().Warn().Msg("cannot JSON-encode inaccessible account info")
		w.WriteHeader(http.StatusInternalServerError)
	}
}

// GetCirculatingSupply serves /circulating-supply end-point.
// Note that known InaccessibleAddresses have their funds removed from the supply for this endpoint.
func (s *Service) GetCirculatingSupply(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	cs, err := chain.GetCirculatingSupply(s.blockchain)
	if err != nil {
		utils.Logger().Warn().Msg("Failed to get circulating supply")
		w.WriteHeader(http.StatusInternalServerError)
	}
	if err := json.NewEncoder(w).Encode(cs); err != nil {
		utils.Logger().Warn().Msg("cannot JSON-encode circulating supply")
		w.WriteHeader(http.StatusInternalServerError)
	}
}

// GetTotalSupply serves /total-supply end-point.
// Note that known InaccessibleAddresses have their funds removed from the supply for this endpoint.
func (s *Service) GetTotalSupply(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	totalSupply, err := stakingReward.GetTotalTokens(s.blockchain)
	if err != nil {
		utils.Logger().Warn().Err(err).Msg("unable to fetch total supply")
		w.WriteHeader(http.StatusInternalServerError)
	}
	totalInaccessible, err := chain.GetInaccessibleTokens(s.blockchain)
	if err != nil {
		utils.Logger().Warn().Err(err).Msg("unable to fetch inaccessible tokens")
		w.WriteHeader(http.StatusInternalServerError)
	}
	if err := json.NewEncoder(w).Encode(totalSupply.Sub(totalInaccessible)); err != nil {
		utils.Logger().Warn().Msg("cannot JSON-encode total supply")
		w.WriteHeader(http.StatusInternalServerError)
	}
}

// GetNodeSync returns status code Teapot 418 if node is not in sync
func (s *Service) GetNodeSync(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	sync, remote, diff := s.backend.SyncStatus(s.blockchain.ShardID())
	if !sync {
		utils.Logger().Debug().Uint64("remote", remote).Uint64("diff", diff).Msg("GetNodeSyncStatus")
		if remote == 0 || diff > nodeSyncTolerance {
			w.WriteHeader(http.StatusTeapot)
		} else {
			// return sync'ed if the diff is less than nodeSyncTolerance
			// this tolerance is only applicable to /node-sync API call
			sync = true
		}
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

// GetCheckpointBitmap get explorer checkpoint bitmap
func (s *Service) GetCheckpointBitmap() *roaring64.Bitmap {
	return s.storage.rb.Clone()
}

func defaultDBPath(ip, port string) string {
	return path.Join(nodeconfig.GetDefaultConfig().DBDir, "explorer_storage_"+ip+"_"+port)
}
