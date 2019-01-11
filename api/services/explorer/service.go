package explorer

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"

	"github.com/ethereum/go-ethereum/rlp"
	"github.com/gorilla/mux"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/utils"
)

// Constants for explorer service.
const (
	explorerPortDifference = 4000
)

// Service is the struct for explorer service.
type Service struct {
	router  *mux.Router
	IP      string
	Port    string
	storage *Storage
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
func (s *Service) Run() {
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

	// Do serving now.
	utils.GetLogInstance().Info("Listening on ", "port: ", GetExplorerPort(s.Port))
	log.Fatal(http.ListenAndServe(addr, s.router))
}

// GetAccountBlocks returns a list of types.Block to server blocks end-point.
func (s *Service) GetAccountBlocks(from, to int) []*types.Block {
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
	var toInt int
	if to == "" {
		bytes, err := db.Get([]byte(BlockHeightKey))
		if err == nil {
			toInt, err = strconv.Atoi(string(bytes))
		}
	} else {
		toInt, err = strconv.Atoi(to)
	}
	if err != nil {
		json.NewEncoder(w).Encode(data.Blocks)
		return
	}

	accountBlocks := s.GetAccountBlocks(fromInt, toInt)
	for id, accountBlock := range accountBlocks {
		if id == 0 || id == len(accountBlocks)-1 || accountBlock == nil {
			continue
		}
		block := &Block{
			Height:     strconv.Itoa(id + fromInt - 1),
			ID:         accountBlock.Hash().Hex(),
			TXCount:    strconv.Itoa(accountBlock.Transactions().Len()),
			Timestamp:  strconv.Itoa(int(accountBlock.Time().Int64() * 1000)),
			MerkleRoot: accountBlock.Hash().Hex(),
			Bytes:      strconv.Itoa(int(accountBlock.Size())),
		}
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
	json.NewEncoder(w).Encode(data.Address)
}
