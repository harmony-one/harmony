package explorer

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/gorilla/mux"
	libp2p_peer "github.com/libp2p/go-libp2p-peer"

	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/bech32"
	common2 "github.com/harmony-one/harmony/internal/common"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
)

// Constants for explorer service.
const (
	explorerPortDifference = 4000
	paginationOffset       = 10
	txViewNone             = "NONE"
	txViewAll              = "ALL"
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
	// Set up router for blocks.
	// Blocks are divided into pages in consequent groups of offset size.
	s.router.Path("/blocks").Queries("from", "{[0-9]*?}", "to", "{[0-9]*?}", "page", "{[0-9]*?}", "offset", "{[0-9]*?}").HandlerFunc(s.GetExplorerBlocks).Methods("GET")
	s.router.Path("/blocks").HandlerFunc(s.GetExplorerBlocks)

	// Set up router for tx.
	s.router.Path("/tx").Queries("id", "{[0-9A-Fa-fx]*?}").HandlerFunc(s.GetExplorerTransaction).Methods("GET")
	s.router.Path("/tx").HandlerFunc(s.GetExplorerTransaction)

	// Set up router for address.
	// Address transactions are divided into pages in consequent groups of offset size.
	s.router.Path("/address").Queries("id", fmt.Sprintf("{([0-9A-Fa-fx]*?)|(t?one1[%s]{38})}", bech32.Charset), "tx_view", "{[A-Z]*?}", "page", "{[0-9]*?}", "offset", "{[0-9]*?}").HandlerFunc(s.GetExplorerAddress).Methods("GET")
	s.router.Path("/address").HandlerFunc(s.GetExplorerAddress)

	// Set up router for node count.
	s.router.Path("/nodes").HandlerFunc(s.GetExplorerNodeCount) // TODO(ricl): this is going to be replaced by /node-count
	s.router.Path("/node-count").HandlerFunc(s.GetExplorerNodeCount)

	// Set up router for shard
	s.router.Path("/shard").Queries("id", "{[0-9]*?}").HandlerFunc(s.GetExplorerShard).Methods("GET")
	s.router.Path("/shard").HandlerFunc(s.GetExplorerShard)

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

// ReadBlocksFromDB returns a list of types.Block to server blocks end-point.
func (s *Service) ReadBlocksFromDB(from, to int) []*types.Block {
	blocks := []*types.Block{}
	for i := from - 1; i <= to+1; i++ {
		if i < 0 {
			blocks = append(blocks, nil)
			continue
		}
		key := GetBlockKey(i)
		data, err := s.Storage.db.Get([]byte(key))
		if err != nil {
			blocks = append(blocks, nil)
			continue
		}
		block := new(types.Block)
		if rlp.DecodeBytes(data, block) != nil {
			utils.Logger().Error().Msg("Error on getting from db")
			os.Exit(1)
		}
		blocks = append(blocks, block)
	}
	return blocks
}

// GetExplorerBlocks serves end-point /blocks
func (s *Service) GetExplorerBlocks(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	from := r.FormValue("from")
	to := r.FormValue("to")
	pageParam := r.FormValue("page")
	offsetParam := r.FormValue("offset")
	order := r.FormValue("order")
	data := &Data{
		Blocks: []*Block{},
	}
	defer func() {
		if err := json.NewEncoder(w).Encode(data.Blocks); err != nil {
			utils.Logger().Warn().Err(err).Msg("cannot JSON-encode blocks")
		}
	}()

	if from == "" {
		utils.Logger().Warn().Msg("Missing from parameter")
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	db := s.Storage.GetDB()
	fromInt, err := strconv.Atoi(from)
	if err != nil {
		utils.Logger().Warn().Err(err).Msg("invalid from parameter")
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	var toInt int
	if to == "" {
		toInt, err = func() (int, error) {
			bytes, err := db.Get([]byte(BlockHeightKey))
			if err == nil {
				return strconv.Atoi(string(bytes))
			}
			return toInt, err
		}()
	} else {
		toInt, err = strconv.Atoi(to)
	}
	if err != nil {
		utils.Logger().Warn().Err(err).Msg("invalid to parameter")
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	var offset int
	if offsetParam != "" {
		offset, err = strconv.Atoi(offsetParam)
		if err != nil || offset < 1 {
			utils.Logger().Warn().Msg("invalid offset parameter")
			w.WriteHeader(http.StatusBadRequest)
			return
		}
	} else {
		offset = paginationOffset
	}
	var page int
	if pageParam != "" {
		page, err = strconv.Atoi(pageParam)
		if err != nil {
			utils.Logger().Warn().Err(err).Msg("invalid page parameter")
			w.WriteHeader(http.StatusBadRequest)
			return
		}
	} else {
		page = 0
	}

	accountBlocks := s.ReadBlocksFromDB(fromInt, toInt)
	for id, accountBlock := range accountBlocks {
		if id == 0 || id == len(accountBlocks)-1 || accountBlock == nil {
			continue
		}
		block := NewBlock(accountBlock, id+fromInt-1)
		// Populate transactions
		for _, tx := range accountBlock.Transactions() {
			transaction := GetTransaction(tx, accountBlock)
			if transaction != nil {
				block.TXs = append(block.TXs, transaction)
			}
		}
		if accountBlocks[id-1] == nil {
			block.BlockTime = int64(0)
			block.PrevBlock = RefBlock{
				ID:     "",
				Height: "",
			}
		} else {
			block.BlockTime = accountBlock.Time().Int64() - accountBlocks[id-1].Time().Int64()
			block.PrevBlock = RefBlock{
				ID:     accountBlocks[id-1].Hash().Hex(),
				Height: strconv.Itoa(id + fromInt - 2),
			}
		}
		if accountBlocks[id+1] == nil {
			block.NextBlock = RefBlock{
				ID:     "",
				Height: "",
			}
		} else {
			block.NextBlock = RefBlock{
				ID:     accountBlocks[id+1].Hash().Hex(),
				Height: strconv.Itoa(id + fromInt),
			}
		}
		data.Blocks = append(data.Blocks, block)
	}
	if offset*page >= len(data.Blocks) {
		data.Blocks = []*Block{}
	} else if offset*page+offset > len(data.Blocks) {
		data.Blocks = data.Blocks[offset*page:]
	} else {
		data.Blocks = data.Blocks[offset*page : offset*page+offset]
	}
	if order == "DESC" {
		sort.Slice(data.Blocks[:], func(i, j int) bool {
			return data.Blocks[i].Timestamp > data.Blocks[j].Timestamp
		})
	} else {
		sort.Slice(data.Blocks[:], func(i, j int) bool {
			return data.Blocks[i].Timestamp < data.Blocks[j].Timestamp
		})
	}
}

// GetExplorerBlocks rpc end-point.
func (s *ServiceAPI) GetExplorerBlocks(ctx context.Context, from, to, page, offset int, order string) ([]*Block, error) {
	if offset == 0 {
		offset = paginationOffset
	}
	db := s.Service.Storage.GetDB()
	if to == 0 {
		bytes, err := db.Get([]byte(BlockHeightKey))
		if err == nil {
			to, err = strconv.Atoi(string(bytes))
			if err != nil {
				utils.Logger().Info().Msg("failed to fetch block height")
				return nil, err
			}
		}
	}
	blocks := make([]*Block, 0)
	accountBlocks := s.Service.ReadBlocksFromDB(from, to)
	for id, accountBlock := range accountBlocks {
		if id == 0 || id == len(accountBlocks)-1 || accountBlock == nil {
			continue
		}
		block := NewBlock(accountBlock, id+from-1)
		// Populate transactions
		for _, tx := range accountBlock.Transactions() {
			transaction := GetTransaction(tx, accountBlock)
			if transaction != nil {
				block.TXs = append(block.TXs, transaction)
			}
		}
		if accountBlocks[id-1] == nil {
			block.BlockTime = int64(0)
			block.PrevBlock = RefBlock{
				ID:     "",
				Height: "",
			}
		} else {
			block.BlockTime = accountBlock.Time().Int64() - accountBlocks[id-1].Time().Int64()
			block.PrevBlock = RefBlock{
				ID:     accountBlocks[id-1].Hash().Hex(),
				Height: strconv.Itoa(id + from - 2),
			}
		}
		if accountBlocks[id+1] == nil {
			block.NextBlock = RefBlock{
				ID:     "",
				Height: "",
			}
		} else {
			block.NextBlock = RefBlock{
				ID:     accountBlocks[id+1].Hash().Hex(),
				Height: strconv.Itoa(id + from),
			}
		}
		blocks = append(blocks, block)
	}
	if offset*page >= len(blocks) {
		blocks = []*Block{}
	} else if offset*page+offset > len(blocks) {
		blocks = blocks[offset*page:]
	} else {
		blocks = blocks[offset*page : offset*page+offset]
	}
	if order == "DESC" {
		sort.Slice(blocks[:], func(i, j int) bool {
			return blocks[i].Timestamp > blocks[j].Timestamp
		})
	} else {
		sort.Slice(blocks[:], func(i, j int) bool {
			return blocks[i].Timestamp < blocks[j].Timestamp
		})
	}
	return blocks, nil
}

// GetExplorerTransaction servers /tx end-point.
func (s *Service) GetExplorerTransaction(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	id := r.FormValue("id")

	data := &Data{}
	defer func() {
		if err := json.NewEncoder(w).Encode(data.TX); err != nil {
			utils.Logger().Warn().Err(err).Msg("cannot JSON-encode TX")
		}
	}()
	if id == "" {
		utils.Logger().Warn().Msg("invalid id parameter")
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	db := s.Storage.GetDB()
	bytes, err := db.Get([]byte(GetTXKey(id)))
	if err != nil {
		utils.Logger().Warn().Err(err).Str("id", id).Msg("cannot read TX")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	tx := new(Transaction)
	if rlp.DecodeBytes(bytes, tx) != nil {
		utils.Logger().Warn().Str("id", id).Msg("cannot convert data from DB")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	data.TX = *tx
}

// GetExplorerTransaction rpc end-point.
func (s *ServiceAPI) GetExplorerTransaction(ctx context.Context, id string) (*Transaction, error) {
	if id == "" {
		utils.Logger().Warn().Msg("invalid id parameter")
		return nil, nil
	}
	db := s.Service.Storage.GetDB()
	bytes, err := db.Get([]byte(GetTXKey(id)))
	if err != nil {
		utils.Logger().Warn().Err(err).Str("id", id).Msg("cannot read TX")
		return nil, err
	}
	tx := new(Transaction)
	if rlp.DecodeBytes(bytes, tx) != nil {
		utils.Logger().Warn().Str("id", id).Msg("cannot convert data from DB")
		return nil, err
	}
	return tx, nil
}

// GetExplorerAddress serves /address end-point.
func (s *Service) GetExplorerAddress(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	id := r.FormValue("id")
	if strings.HasPrefix(id, "0x") {
		parsedAddr := common2.ParseAddr(id)
		oneAddr, err := common2.AddressToBech32(parsedAddr)
		if err != nil {
			utils.Logger().Warn().Err(err).Msg("unrecognized address format")
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		id = oneAddr
	}
	key := GetAddressKey(id)
	txViewParam := r.FormValue("tx_view")
	pageParam := r.FormValue("page")
	offsetParam := r.FormValue("offset")
	order := r.FormValue("order")
	txView := txViewNone
	if txViewParam != "" {
		txView = txViewParam
	}
	utils.Logger().Info().Str("Address", id).Msg("Querying address")
	data := &Data{}
	defer func() {
		if err := json.NewEncoder(w).Encode(data.Address); err != nil {
			utils.Logger().Warn().Err(err).Msg("cannot JSON-encode Address")
		}
	}()
	if id == "" {
		utils.Logger().Warn().Msg("missing address id param")
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	var err error
	var offset int
	if offsetParam != "" {
		offset, err = strconv.Atoi(offsetParam)
		if err != nil || offset < 1 {
			utils.Logger().Warn().Msg("invalid offset parameter")
			w.WriteHeader(http.StatusBadRequest)
			return
		}
	} else {
		offset = paginationOffset
	}
	var page int
	if pageParam != "" {
		page, err = strconv.Atoi(pageParam)
		if err != nil {
			utils.Logger().Warn().Err(err).Msg("invalid page parameter")
			w.WriteHeader(http.StatusBadRequest)
			return
		}
	} else {
		page = 0
	}

	data.Address.ID = id
	// Try to populate the banace by directly calling get balance.
	// Check the balance from blockchain rather than local DB dump
	balanceAddr := big.NewInt(0)
	if s.GetAccountBalance != nil {
		address := common2.ParseAddr(id)
		balance, err := s.GetAccountBalance(address)
		if err == nil {
			balanceAddr = balance
		}
	}

	db := s.Storage.GetDB()
	bytes, err := db.Get([]byte(key))
	if err != nil {
		utils.Logger().Warn().Err(err).Str("id", id).Msg("cannot fetch address from db")
		data.Address.Balance = balanceAddr
		return
	}

	if err = rlp.DecodeBytes(bytes, &data.Address); err != nil {
		utils.Logger().Warn().Str("id", id).Msg("cannot convert address data")
		data.Address.Balance = balanceAddr
		return
	}

	data.Address.Balance = balanceAddr

	switch txView {
	case txViewNone:
		data.Address.TXs = nil
	case Received:
		receivedTXs := make([]*Transaction, 0)
		for _, tx := range data.Address.TXs {
			if tx.Type == Received {
				receivedTXs = append(receivedTXs, tx)
			}
		}
		data.Address.TXs = receivedTXs
	case Sent:
		sentTXs := make([]*Transaction, 0)
		for _, tx := range data.Address.TXs {
			if tx.Type == Sent {
				sentTXs = append(sentTXs, tx)
			}
		}
		data.Address.TXs = sentTXs
	}
	if offset*page >= len(data.Address.TXs) {
		data.Address.TXs = []*Transaction{}
	} else if offset*page+offset > len(data.Address.TXs) {
		data.Address.TXs = data.Address.TXs[offset*page:]
	} else {
		data.Address.TXs = data.Address.TXs[offset*page : offset*page+offset]
	}
	if order == "DESC" {
		sort.Slice(data.Address.TXs[:], func(i, j int) bool {
			return data.Address.TXs[i].Timestamp > data.Address.TXs[j].Timestamp
		})
	} else {
		sort.Slice(data.Address.TXs[:], func(i, j int) bool {
			return data.Address.TXs[i].Timestamp < data.Address.TXs[j].Timestamp
		})
	}
}

// GetExplorerAddress rpc end-point.
func (s *ServiceAPI) GetExplorerAddress(ctx context.Context, id, txView string, page, offset int, order string) (*Address, error) {
	if strings.HasPrefix(id, "0x") {
		parsedAddr := common2.ParseAddr(id)
		oneAddr, err := common2.AddressToBech32(parsedAddr)
		if err != nil {
			utils.Logger().Warn().Msg("unrecognized address format")
			return nil, err
		}
		id = oneAddr
	}
	if offset == 0 {
		offset = paginationOffset
	}
	if txView == "" {
		txView = txViewNone
	}
	utils.Logger().Info().Str("Address", id).Msg("Querying address")
	if id == "" {
		utils.Logger().Warn().Msg("missing address id param")
		return nil, nil
	}
	address := &Address{}
	address.ID = id
	// Try to populate the banace by directly calling get balance.
	// Check the balance from blockchain rather than local DB dump
	balanceAddr := big.NewInt(0)
	if s.Service.GetAccountBalance != nil {
		addr := common2.ParseAddr(id)
		balance, err := s.Service.GetAccountBalance(addr)
		if err == nil {
			balanceAddr = balance
		}
	}

	key := GetAddressKey(id)
	db := s.Service.Storage.GetDB()
	bytes, err := db.Get([]byte(key))
	if err != nil {
		utils.Logger().Warn().Err(err).Str("id", id).Msg("cannot fetch address from db")
		address.Balance = balanceAddr
		return address, err
	}
	if err = rlp.DecodeBytes(bytes, &address); err != nil {
		utils.Logger().Warn().Str("id", id).Msg("cannot convert address data")
		address.Balance = balanceAddr
		return address, err
	}

	address.Balance = balanceAddr

	switch txView {
	case txViewNone:
		address.TXs = nil
	case Received:
		receivedTXs := make([]*Transaction, 0)
		for _, tx := range address.TXs {
			if tx.Type == Received {
				receivedTXs = append(receivedTXs, tx)
			}
		}
		address.TXs = receivedTXs
	case Sent:
		sentTXs := make([]*Transaction, 0)
		for _, tx := range address.TXs {
			if tx.Type == Sent {
				sentTXs = append(sentTXs, tx)
			}
		}
		address.TXs = sentTXs
	}

	if offset*page >= len(address.TXs) {
		address.TXs = []*Transaction{}
	} else if offset*page+offset > len(address.TXs) {
		address.TXs = address.TXs[offset*page:]
	} else {
		address.TXs = address.TXs[offset*page : offset*page+offset]
	}
	if order == "DESC" {
		sort.Slice(address.TXs[:], func(i, j int) bool {
			return address.TXs[i].Timestamp > address.TXs[j].Timestamp
		})
	} else {
		sort.Slice(address.TXs[:], func(i, j int) bool {
			return address.TXs[i].Timestamp < address.TXs[j].Timestamp
		})
	}
	return address, nil
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
