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

// GetExplorerPort ...
func GetExplorerPort(nodePort string) string {
	if port, err := strconv.Atoi(nodePort); err == nil {
		return fmt.Sprintf("%d", port-explorerPortDifference)
	}
	fmt.Println("error on parsing.")
	return ""
}

// Init is to do init for ExplorerService.
func (s *Service) Init(remove bool) {
	s.storage = GetStorageInstance(s.IP, s.Port, remove)
}

// Run is to run serving explorer.
func (s *Service) Run() {
	// Init address.
	addr := net.JoinHostPort("", GetExplorerPort(s.Port))

	// Set up router
	s.router = mux.NewRouter()
	// s.router.Path("/block_info").Queries("from", "{[0-9]*?}", "to", "{[0-9]*?}").HandlerFunc(s.GetExplorerBlockInfo).Methods("GET")
	// s.router.Path("/block_info").HandlerFunc(s.GetExplorerBlockInfo)

	s.router.Path("/blocks").Queries("from", "{[0-9]*?}", "to", "{[0-9]*?}").HandlerFunc(s.GetExplorerBlocks).Methods("GET")
	s.router.Path("/blocks").HandlerFunc(s.GetExplorerBlocks)

	s.router.Path("/tx").Queries("id", "{[0-9A-Fa-fx]*?}").HandlerFunc(s.GetExplorerTransaction).Methods("GET")
	s.router.Path("/tx").HandlerFunc(s.GetExplorerTransaction)
	// Do serving now.
	fmt.Println("Listening to:", GetExplorerPort(s.Port))
	log.Fatal(http.ListenAndServe(addr, s.router))
}

// GetExplorerBlockInfo ...
// func (s *Service) GetExplorerBlockInfo(w http.ResponseWriter, r *http.Request) {
// 	w.Header().Set("Content-Type", "application/json")
// 	from := r.FormValue("from")
// 	to := r.FormValue("to")

// 	data := &Data{
// 		Blocks: []*BlockInfo{},
// 	}
// 	if from == "" {
// 		json.NewEncoder(w).Encode(data.Blocks)
// 		return
// 	}
// 	db := s.storage.GetDB()
// 	fromInt, err := strconv.Atoi(from)
// 	var toInt int
// 	if to == "" {
// 		bytes, err := db.Get([]byte(BlockHeightKey))
// 		if err == nil {
// 			toInt, err = strconv.Atoi(string(bytes))
// 		}
// 	} else {
// 		toInt, err = strconv.Atoi(to)
// 	}
// 	fmt.Println("from", fromInt, "to", toInt)
// 	if err != nil {
// 		json.NewEncoder(w).Encode(data.Blocks)
// 		return
// 	}

// 	data.Blocks = s.PopulateBlockInfo(fromInt, toInt)
// 	json.NewEncoder(w).Encode(data.Blocks)
// }

// PopulateBlockInfo ...
func (s *Service) PopulateBlockInfo(from, to int) []*BlockInfo {
	blocks := []*BlockInfo{}
	for i := from; i <= to; i++ {
		key := GetBlockInfoKey(i)
		fmt.Println("getting blockinfo with key ", key)
		data, err := storage.db.Get([]byte(key))
		if err != nil {
			fmt.Println("Error on getting from db")
			os.Exit(1)
		}
		block := new(BlockInfo)
		if rlp.DecodeBytes(data, block) != nil {
			fmt.Println("RLP Decoding error")
			os.Exit(1)
		}
		blocks = append(blocks, block)
	}
	return blocks
}

// GetAccountBlocks ...
func (s *Service) GetAccountBlocks(from, to int) []*types.Block {
	blocks := []*types.Block{}
	for i := from - 1; i <= to+1; i++ {
		if i < 0 {
			blocks = append(blocks, nil)
			continue
		}
		key := GetBlockKey(i)
		fmt.Println("getting block with key ", key)
		data, err := storage.db.Get([]byte(key))
		if err != nil {
			blocks = append(blocks, nil)
			continue
		}
		block := new(types.Block)
		if rlp.DecodeBytes(data, block) != nil {
			fmt.Println("RLP Block decoding error")
			os.Exit(1)
		}
		blocks = append(blocks, block)
	}
	return blocks
}

// GetTransaction ...
func GetTransaction(tx *types.Transaction, accountBlock *types.Block) *Transaction {
	if tx.To() == nil {
		return nil
	}
	return &Transaction{
		ID:        tx.Hash().Hex(),
		Timestamp: strconv.Itoa(int(accountBlock.Time().Int64() * 1000)),
		From:      tx.To().Hex(),
		To:        tx.To().Hex(),
		Value:     strconv.Itoa(int(tx.GasPrice().Int64())),
		Bytes:     strconv.Itoa(int(tx.Size())),
	}
}

// GetExplorerBlocks ...
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
	fmt.Println("from", fromInt, "to", toInt)
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

// GetExplorerTransaction ...
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

// GetExplorerAddress ...
func (s *Service) GetExplorerAddress(w http.ResponseWriter, r *http.Request) {
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
