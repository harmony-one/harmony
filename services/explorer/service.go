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

// // GetPersonEndpoint is the specific person end point.
// func (s *Service) GetPersonEndpoint(w http.ResponseWriter, r *http.Request) {
// 	params := mux.Vars(r)
// 	for _, item := range s.people {
// 		if item.ID == params["id"] {
// 			json.NewEncoder(w).Encode(item)
// 			return
// 		}
// 	}
// 	json.NewEncoder(w).Encode(&Person{})
// }

// // GetPeopleEndpoint is the people end point.
// func (s *Service) GetPeopleEndpoint(w http.ResponseWriter, r *http.Request) {
// 	json.NewEncoder(w).Encode(s.people)
// }

// // CreatePersonEndpoint is post people/{id} end point.
// func (s *Service) CreatePersonEndpoint(w http.ResponseWriter, r *http.Request) {
// 	params := mux.Vars(r)
// 	var person Person
// 	_ = json.NewDecoder(r.Body).Decode(&person)
// 	person.ID = params["id"]
// 	s.people = append(s.people, person)
// 	json.NewEncoder(w).Encode(s.people)
// }

// // DeletePersonEndpoint is delete people/{id} end point.
// func (s *Service) DeletePersonEndpoint(w http.ResponseWriter, r *http.Request) {
// 	params := mux.Vars(r)
// 	for index, item := range s.people {
// 		if item.ID == params["id"] {
// 			s.people = append(s.people[:index], s.people[index+1:]...)
// 			break
// 		}
// 		json.NewEncoder(w).Encode(s.people)
// 	}
// }
