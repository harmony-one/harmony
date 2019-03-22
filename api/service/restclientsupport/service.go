package restclientsupport

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	"github.com/harmony-one/harmony/internal/utils"
)

// Constants for rest client support service.
const (
	Port = "30000"
)

// Service is the struct for rest client support service.
type Service struct {
	router                          *mux.Router
	server                          *http.Server
	CreateTransactionForEnterMethod func(int64, string) error
	GetResult                       func(string) ([]string, []*big.Int)
	CreateTransactionForPickWinner  func() error
	messageChan                     chan *msg_pb.Message
}

// New returns new client support service.
func New(
	CreateTransactionForEnterMethod func(int64, string) error,
	GetResult func(string) ([]string, []*big.Int),
	CreateTransactionForPickWinner func() error,
) *Service {
	return &Service{
		CreateTransactionForEnterMethod: CreateTransactionForEnterMethod,
		GetResult:                       GetResult,
		CreateTransactionForPickWinner:  CreateTransactionForPickWinner,
	}
}

// StartService starts rest client support service.
func (s *Service) StartService() {
	utils.GetLogInstance().Info("Starting rest client support.")
	s.server = s.Run()
}

// StopService shutdowns rest client support service.
func (s *Service) StopService() {
	utils.GetLogInstance().Info("Shutting down rest client support service.")
	if err := s.server.Shutdown(context.Background()); err != nil {
		utils.GetLogInstance().Error("Error when shutting down rest client support server", "error", err)
	} else {
		utils.GetLogInstance().Error("Shutting down rest client support server successufully")
	}
}

// Run is to run serving rest client support.
func (s *Service) Run() *http.Server {
	// Init address.
	addr := net.JoinHostPort("", Port)

	s.router = mux.NewRouter()
	// Set up router for blocks.
	s.router.Path("/enter").Queries("key", "{[0-9A-Fa-fx]*?}", "amount", "{[0-9]*?}").HandlerFunc(s.Enter).Methods("GET")
	s.router.Path("/enter").HandlerFunc(s.Enter)

	// Set up router for tx.
	s.router.Path("/result").Queries("key", "{[0-9A-Fa-fx]*?}").HandlerFunc(s.Result).Methods("GET")
	s.router.Path("/result").HandlerFunc(s.Result)

	// Set up router for tx.
	s.router.Path("/winner").HandlerFunc(s.Winner)

	// Do serving now.
	utils.GetLogInstance().Info("Listening on ", "port: ", Port)
	server := &http.Server{Addr: addr, Handler: s.router}
	go server.ListenAndServe()
	return server
}

type Response struct {
	Players  []string `json:"players"`
	Balances []string `json:"balances"`
	Success  bool     `json:"success"`
}

// Enter --
func (s *Service) Enter(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	key := r.FormValue("key")
	amount := r.FormValue("amount")
	fmt.Println("enter: key", key, "amount", amount)
	amountInt, err := strconv.ParseInt(amount, 10, 64)

	res := &Response{Success: false}
	if err != nil {
		json.NewEncoder(w).Encode(res)
		return
	}
	if s.CreateTransactionForEnterMethod == nil {
		json.NewEncoder(w).Encode(res)
		return
	}
	if err := s.CreateTransactionForEnterMethod(amountInt, key); err != nil {
		json.NewEncoder(w).Encode(res)
		return
	}

	res.Success = true
	json.NewEncoder(w).Encode(res)
}

// Result --
func (s *Service) Result(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	key := r.FormValue("key")
	res := &Response{
		Success: false,
	}

	fmt.Println("result: key", key)
	if s.GetResult == nil {
		json.NewEncoder(w).Encode(res)
		return
	}
	players, balances := s.GetResult(key)
	balancesString := []string{}
	for _, balance := range balances {
		balancesString = append(balancesString, balance.String())
	}
	res.Players = players
	res.Balances = balancesString
	res.Success = true
	json.NewEncoder(w).Encode(res)
}

// Result --
func (s *Service) Winner(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	fmt.Println("winner")
	res := &Response{
		Success: false,
	}

	if s.CreateTransactionForPickWinner == nil {
		json.NewEncoder(w).Encode(res)
		return
	}

	if err := s.CreateTransactionForPickWinner(); err != nil {
		utils.GetLogInstance().Error("error", err)
		json.NewEncoder(w).Encode(res)
		return
	}
	res.Success = true
	json.NewEncoder(w).Encode(res)
}

// NotifyService notify service
func (s *Service) NotifyService(params map[string]interface{}) {
	return
}

// SetMessageChan sets up message channel to service.
func (s *Service) SetMessageChan(messageChan chan *msg_pb.Message) {
	s.messageChan = messageChan
}
