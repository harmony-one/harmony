package main

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	msg_pb "github.com/harmony-one/harmony/api/proto/message"
)

// Constants for main demo.
const (
	Port    = "31313"
	LocalIP = "127.0.0.1"
)

var (
	server     *http.Server
	grpcClient = msg_pb.NewClient(LocalIP)
)

// Enter ---
func Enter(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	key := r.FormValue("key")
	amount, err := strconv.ParseInt(r.FormValue("amount"), 10, 0)
	if err != nil {
		fmt.Println(err)
		json.NewEncoder(w).Encode("")
		return
	}

	msg := &msg_pb.Message{
		Type: msg_pb.MessageType_LOTTERY_REQUEST,
		Request: &msg_pb.Message_LotteryRequest{
			LotteryRequest: &msg_pb.LotteryRequest{
				Type:       msg_pb.LotteryRequest_ENTER,
				PrivateKey: key,
				Amount:     amount,
			},
		},
	}
	res, err := grpcClient.Process(msg)
	if err != nil {
		fmt.Println(err)
		json.NewEncoder(w).Encode("")
		return
	}
	json.NewEncoder(w).Encode(res)
}

// Result --
func Result(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	key := r.FormValue("key")

	json.NewEncoder(w).Encode("")
	msg := &msg_pb.Message{
		Type: msg_pb.MessageType_LOTTERY_REQUEST,
		Request: &msg_pb.Message_LotteryRequest{
			LotteryRequest: &msg_pb.LotteryRequest{
				Type:       msg_pb.LotteryRequest_RESULT,
				PrivateKey: key,
			},
		},
	}

	res, err := grpcClient.Process(msg)
	if err != nil {
		fmt.Println(err)
		json.NewEncoder(w).Encode("")
		return
	}
	json.NewEncoder(w).Encode(res)
}

// PickWinner picks a winner.
func PickWinner(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	json.NewEncoder(w).Encode("")
	msg := &msg_pb.Message{
		Type: msg_pb.MessageType_LOTTERY_REQUEST,
		Request: &msg_pb.Message_LotteryRequest{
			LotteryRequest: &msg_pb.LotteryRequest{
				Type: msg_pb.LotteryRequest_PICK_WINNER,
			},
		},
	}

	res, err := grpcClient.Process(msg)
	if err != nil {
		fmt.Println(err)
		json.NewEncoder(w).Encode("")
		return
	}
	json.NewEncoder(w).Encode(res)
}

func main() {
	addr := net.JoinHostPort("", Port)

	router := mux.NewRouter()
	// Set up router for server.
	router.Path("/enter").Queries("key", "{[0-9A-Fa-fx]*?}", "amount", "[0-9]*").HandlerFunc(Enter).Methods("GET")
	router.Path("/enter").HandlerFunc(Enter)

	router.Path("/result").HandlerFunc(Result)
	router.Path("/winner").HandlerFunc(PickWinner)

	server := &http.Server{Addr: addr, Handler: router}
	fmt.Println("Serving")
	server.ListenAndServe()
}
