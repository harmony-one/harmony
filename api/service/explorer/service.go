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
	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
	libp2p_peer "github.com/libp2p/go-libp2p-peer"
)

// Constants for explorer service.
const (
	explorerPortDifference = 4000
)

// Service is the struct for explorer service.
type Service struct {
	router            *mux.Router
	IP                string
	Port              string
	GetNodeIDs        func() []libp2p_peer.ID
	storage           *Storage
	server            *http.Server
	messageChan       chan *msg_pb.Message
	GetAccountBalance func(common.Address) (*big.Int, error)
}

// New returns explorer service.
func New(selfPeer *p2p.Peer, GetNodeIDs func() []libp2p_peer.ID, GetAccountBalance func(common.Address) (*big.Int, error)) *Service {
	return &Service{
		IP:                selfPeer.IP,
		Port:              selfPeer.Port,
		GetNodeIDs:        GetNodeIDs,
		GetAccountBalance: GetAccountBalance,
	}
}

// StartService starts explorer service.
func (s *Service) StartService() {
	utils.GetLogInstance().Info("Starting explorer service.")
	s.Init(true)
	s.server = s.Run()
}

// StopService shutdowns explorer service.
func (s *Service) StopService() {
	utils.GetLogInstance().Info("Shutting down explorer service.")
	if err := s.server.Shutdown(context.Background()); err != nil {
		utils.GetLogInstance().Error("Error when shutting down explorer server", "error", err)
	} else {
		utils.GetLogInstance().Info("Shutting down explorer server successufully")
	}
}

// GetExplorerPort returns the port serving explorer dashboard. This port is explorerPortDifference less than the node port.
func GetExplorerPort(nodePort string) string {
	if port, err := strconv.Atoi(nodePort); err == nil {
		return fmt.Sprintf("%d", port-explorerPortDifference)
	}
	utils.GetLogInstance().Error("error on parsing.")
	return ""
}

// Init is to initialize for ExplorerService.
func (s *Service) Init(remove bool) {
	s.storage = GetStorageInstance(s.IP, s.Port, remove)
}

// Run is to run serving explorer.
func (s *Service) Run() *http.Server {
	// Init address.
	addr := net.JoinHostPort("", GetExplorerPort(s.Port))

	s.router = mux.NewRouter()
	// Set up router for blocks.
	s.router.Path("/blocks").Queries("from", "{[0-9]*?}", "to", "{[0-9]*?}").HandlerFunc(s.GetExplorerBlocks).Methods("GET")
	s.router.Path("/blocks").HandlerFunc(s.GetExplorerBlocks)

	// Set up router for tx.
	s.router.Path("/tx").Queries("id", "{[0-9A-Fa-fx]*?}").HandlerFunc(s.GetExplorerTransaction).Methods("GET")
	s.router.Path("/tx").HandlerFunc(s.GetExplorerTransaction)

	// Set up router for address.
	s.router.Path("/address").Queries("id", "{[0-9A-Fa-fx]*?}").HandlerFunc(s.GetExplorerAddress).Methods("GET")
	s.router.Path("/address").HandlerFunc(s.GetExplorerAddress)

	// Set up router for node count.
	s.router.Path("/nodes").HandlerFunc(s.GetExplorerNodeCount) // TODO(ricl): this is going to be replaced by /node-count
	s.router.Path("/node-count").HandlerFunc(s.GetExplorerNodeCount)

	// Set up router for shard
	s.router.Path("/shard").Queries("id", "{[0-9]*?}").HandlerFunc(s.GetExplorerShard).Methods("GET")
	s.router.Path("/shard").HandlerFunc(s.GetExplorerShard)

	// Do serving now.
	utils.GetLogInstance().Info("Listening on ", "port: ", GetExplorerPort(s.Port))
	server := &http.Server{Addr: addr, Handler: s.router}
	go server.ListenAndServe()
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
		data, err := storage.db.Get([]byte(key))
		if err != nil {
			blocks = append(blocks, nil)
			continue
		}
		block := new(types.Block)
		if rlp.DecodeBytes(data, block) != nil {
			utils.GetLogInstance().Error("Error on getting from db")
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

	data := &Data{
		Blocks: []*Block{},
	}
	if from == "" {
		json.NewEncoder(w).Encode(data.Blocks)
		return
	}
	db := s.storage.GetDB()
	fromInt, err := strconv.Atoi(from)
	if err != nil {
		json.NewEncoder(w).Encode(data.Blocks)
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
		json.NewEncoder(w).Encode(data.Blocks)
		return
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
			block.PrevBlock = RefBlock{
				ID:     "",
				Height: "",
			}
		} else {
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
	json.NewEncoder(w).Encode(data.Blocks)
}

// GetExplorerTransaction servers /tx end-point.
func (s *Service) GetExplorerTransaction(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	id := r.FormValue("id")

	data := &Data{}
	if id == "" {
		json.NewEncoder(w).Encode(data.TX)
		return
	}
	db := s.storage.GetDB()
	bytes, err := db.Get([]byte(GetTXKey(id)))
	if err != nil {
		json.NewEncoder(w).Encode(data.TX)
		return
	}
	tx := new(Transaction)
	if rlp.DecodeBytes(bytes, tx) != nil {
		json.NewEncoder(w).Encode(data.TX)
		return
	}
	data.TX = *tx
	json.NewEncoder(w).Encode(data.TX)
}

// GetExplorerAddress serves /address end-point.
func (s *Service) GetExplorerAddress(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	id := r.FormValue("id")
	key := GetAddressKey(id)

	data := &Data{}
	if id == "" {
		json.NewEncoder(w).Encode(nil)
		return
	}
	db := s.storage.GetDB()
	bytes, err := db.Get([]byte(key))
	if err != nil {
		json.NewEncoder(w).Encode(nil)
		return
	}
	var address Address
	if err = rlp.DecodeBytes(bytes, &address); err != nil {
		json.NewEncoder(w).Encode(nil)
		return
	}
	data.Address = address

	// Check the balance from blockchain rather than local DB dump
	if s.GetAccountBalance != nil {
		address := common.HexToAddress(id)
		balance, err := s.GetAccountBalance(address)
		if err == nil {
			data.Address.Balance = balance
		}
	}

	json.NewEncoder(w).Encode(data.Address)
}

// GetExplorerNodeCount serves /nodes end-point.
func (s *Service) GetExplorerNodeCount(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(len(s.GetNodeIDs()))
}

// GetExplorerShard serves /shard end-point
func (s *Service) GetExplorerShard(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var nodes []Node
	for _, nodeID := range s.GetNodeIDs() {
		nodes = append(nodes, Node{
			ID: libp2p_peer.IDB58Encode(nodeID),
		})
	}
	json.NewEncoder(w).Encode(Shard{
		Nodes: nodes,
	})
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
