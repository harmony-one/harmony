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
)

// Constants for explorer service.
const (
	ExplorerServicePort = "5000"
)

// Service is the struct for explorer service.
type Service struct {
	router  *mux.Router
	IP      string
	Port    string
	storage *Storage
}

// Init is to do init for ExplorerService.
func (s *Service) Init() {
	s.storage = GetStorageInstance(s.IP, s.Port, false)
}

// Run is to run serving explorer.
func (s *Service) Run() {
	// Init address.
	addr := net.JoinHostPort("", ExplorerServicePort)

	// Set up router
	s.router = mux.NewRouter()
	s.router.Path("/block_info").Queries("from", "{[0-9]*?}", "to", "{[0-9]*?}").HandlerFunc(s.GetExplorerBlockInfo).Methods("GET")
	s.router.Path("/block_info").HandlerFunc(s.GetExplorerBlockInfo)

	s.router.HandleFunc("/block", s.GetExplorerBlock).Methods("GET")
	s.router.HandleFunc("/address", s.GetExplorerAddress).Methods("GET")
	s.router.HandleFunc("/tx", s.GetExplorerTransaction).Methods("GET")
	// Do serving now.
	fmt.Println("Listening to:", ExplorerServicePort)
	go log.Fatal(http.ListenAndServe(addr, s.router))
}

// GetExplorerBlockInfo ...
func (s *Service) GetExplorerBlockInfo(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	from := r.FormValue("from")
	to := r.FormValue("to")

	data := &Data{
		Blocks: []*BlockInfo{},
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

	data.Blocks = s.PopulateBlockInfo(fromInt, toInt)
	json.NewEncoder(w).Encode(data.Blocks)
}

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
		fmt.Println("rlp decode successfully ")
	}
	return blocks
}

// GetExplorerBlock ...
func (s *Service) GetExplorerBlock(w http.ResponseWriter, r *http.Request) {
	// w.Header().Set("Content-Type", "application/json")
	// id := r.FormValue("id")

	// data := &Data{}
	// db := s.storage.GetDB()

	// if id == "" {
	// 	json.NewEncoder(w).Encode(data.Block)
	// 	return
	// }
	// idInt, err := strconv.Atoi(id)
	// if err != nil {
	// 	json.NewEncoder(w).Encode(data.Block)
	// 	return
	// }
	// data, err := db.Get([]byte(GetBlockKey(idInt)))

	// data.Block = storage.GetDB()
	// json.NewEncoder(w).Encode(data.Blocks)
}

// GetExplorerAddress ...
func (s *Service) GetExplorerAddress(w http.ResponseWriter, r *http.Request) {
	// json.NewEncoder(w).Encode(s.data.Address)
}

// GetExplorerTransaction ...
func (s *Service) GetExplorerTransaction(w http.ResponseWriter, r *http.Request) {
	// json.NewEncoder(w).Encode(s.data.Address)
}
