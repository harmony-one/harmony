package explorer

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"

	"github.com/gorilla/mux"
)

// Constants for explorer service.
const (
	ExplorerServicePort = "5000"
)

// Service is the struct for explorer service.
type Service struct {
	data   Data
	router *mux.Router
}

// Init is to do init for ExplorerService.
func (s *Service) Init() {
	s.data = ReadFakeData()
}

// Run is to run serving explorer.
func (s *Service) Run() {
	// Init address.
	addr := net.JoinHostPort("", ExplorerServicePort)

	// Set up router
	s.router = mux.NewRouter()
	s.router.HandleFunc("/blocks", s.GetExplorerBlocks).Methods("GET")
	s.router.HandleFunc("/block", s.GetExplorerBlock).Methods("GET")
	s.router.HandleFunc("/address", s.GetExplorerAddress).Methods("GET")
	// s.router.HandleFunc("/people", s.GetPeopleEndpoint).Methods("GET")
	// s.router.HandleFunc("/people/{id}", s.GetPersonEndpoint).Methods("GET")
	// s.router.HandleFunc("/people/{id}", s.CreatePersonEndpoint).Methods("POST")
	// s.router.HandleFunc("/people/{id}", s.DeletePersonEndpoint).Methods("DELETE")
	// Do serving now.
	fmt.Println("Listening to:", ExplorerServicePort)
	go log.Fatal(http.ListenAndServe(addr, s.router))
}

// GetExplorerBlocks ...
func (s *Service) GetExplorerBlocks(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(s.data.Blocks)
}

// GetExplorerBlock ...
func (s *Service) GetExplorerBlock(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(s.data.Block)
}

// GetExplorerAddress ...
func (s *Service) GetExplorerAddress(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(s.data.Address)
}
