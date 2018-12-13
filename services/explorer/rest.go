package explorer

import (
	"encoding/json"
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
	people []Person
	router *mux.Router
}

// Init is to do init for ExplorerService.
func (s *Service) Init() {
	// s.people = append(s.people, Person{ID: "1", Firstname: "John", Lastname: "Doe", Address: &Address{City: "City X", State: "State X"}})
	// s.people = append(s.people, Person{ID: "2", Firstname: "Koko", Lastname: "Doe", Address: &Address{City: "City Z", State: "State Y"}})
}

// Run is to run serving explorer.
func (s *Service) Run() {
	// Init address.
	addr := net.JoinHostPort("", ExplorerServicePort)

	// Set up router
	s.router = mux.NewRouter()
	s.router.HandleFunc("/people", s.GetPeopleEndpoint).Methods("GET")
	s.router.HandleFunc("/people/{id}", s.GetPersonEndpoint).Methods("GET")
	s.router.HandleFunc("/people/{id}", s.CreatePersonEndpoint).Methods("POST")
	s.router.HandleFunc("/people/{id}", s.DeletePersonEndpoint).Methods("DELETE")
	// Do serving now.
	go log.Fatal(http.ListenAndServe(addr, s.router))
}

// Person is fake struct for testing.
type Person struct {
	ID        string   `json:"id,omitempty"`
	Firstname string   `json:"firstname,omitempty"`
	Lastname  string   `json:"lastname,omitempty"`
	Address   *Address `json:"address,omitempty"`
}

// Address is fake struct for testing.
// type Address struct {
// 	City  string `json:"city,omitempty"`
// 	State string `json:"state,omitempty"`
// }

// GetPersonEndpoint is the specific person end point.
func (s *Service) GetPersonEndpoint(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	for _, item := range s.people {
		if item.ID == params["id"] {
			json.NewEncoder(w).Encode(item)
			return
		}
	}
	json.NewEncoder(w).Encode(&Person{})
}

// GetPeopleEndpoint is the people end point.
func (s *Service) GetPeopleEndpoint(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(s.people)
}

// CreatePersonEndpoint is post people/{id} end point.
func (s *Service) CreatePersonEndpoint(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	var person Person
	_ = json.NewDecoder(r.Body).Decode(&person)
	person.ID = params["id"]
	s.people = append(s.people, person)
	json.NewEncoder(w).Encode(s.people)
}

// DeletePersonEndpoint is delete people/{id} end point.
func (s *Service) DeletePersonEndpoint(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	for index, item := range s.people {
		if item.ID == params["id"] {
			s.people = append(s.people[:index], s.people[index+1:]...)
			break
		}
		json.NewEncoder(w).Encode(s.people)
	}
}
