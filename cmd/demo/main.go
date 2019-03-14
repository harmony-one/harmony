package main

import (
	"encoding/json"
	"net"
	"net/http"

	"github.com/gorilla/mux"
	msg_pb "github.com/harmony-one/harmony/api/proto/message"
)

const (
	Port    = "313131"
	LocalIP = "127.0.0.1"
)

var (
	server     *http.Server
	grpcClient = msg_pb.NewClient(LocalIP)
)

// Enter processes /enter end point.
func Enter(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode("")
}

// Result processes /result end point.
func Result(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode("")
}

func main() {
	addr := net.JoinHostPort("", Port)

	router := mux.NewRouter()
	// Set up router for server.
	router.Path("/enter").Queries("key", "{[0-9A-Fa-fx]*?}", "ether", "[0-9]*").HandlerFunc(Enter).Methods("GET")
	router.Path("/enter").HandlerFunc(Enter)

	router.Path("/result").HandlerFunc(Result)

	server := &http.Server{Addr: addr, Handler: router}
	server.ListenAndServe()
}
