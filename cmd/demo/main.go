package main

import (
	"net"
	"net/http"

	"github.com/gorilla/mux"
)

var (
	server *http.Server
)

const (
	Port = "313131"
)

func main() {
	addr := net.JoinHostPort("", Port)

	router := mux.NewRouter()
	// Set up router for blocks.
	s.router.Path("/blocks").Queries("from", "{[0-9]*?}", "to", "{[0-9]*?}").HandlerFunc(s.GetExplorerBlocks).Methods("GET")
	s.router.Path("/blocks").HandlerFunc(s.GetExplorerBlocks)

	server := &http.Server{Addr: addr, Handler: router}
	server.ListenAndServe()
}
