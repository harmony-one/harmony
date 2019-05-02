package restclientsupport

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/internal/utils"
)

// Play triggers play method of puzzle smart contract.
func (s *Service) Play(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	key := r.FormValue("key")
	amount := r.FormValue("amount")
	amountInt, err := strconv.ParseInt(amount, 10, 64)
	fmt.Println("Play: key", key, "amount", amountInt)

	res := &Response{
		Success: false,
	}

	if err != nil {
		json.NewEncoder(w).Encode(res)
		return
	}

	if s.CreateTransactionForPlayMethod == nil {
		fmt.Println("puzzle-play no method", key)
		json.NewEncoder(w).Encode(res)
		return
	}

	if err := s.CreateTransactionForPlayMethod(key, amountInt); err != nil {
		utils.GetLogInstance().Error("puzzle-play, error", err)
		json.NewEncoder(w).Encode(res)
		return
	}
	res.Success = true
	json.NewEncoder(w).Encode(res)
}

// Payout triggers play payout of puzzle smart contract.
func (s *Service) Payout(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	address := r.FormValue("address")
	newLevel := r.FormValue("new_level")
	sequence := r.FormValue("sequence")
	newLevelInt, err := strconv.Atoi(newLevel)

	fmt.Println("Payout: address", address, "new_level", newLevelInt)
	res := &Response{
		Success: false,
	}

	if s.CreateTransactionForPayoutMethod == nil {
		json.NewEncoder(w).Encode(res)
		return
	}

	if err = s.CreateTransactionForPayoutMethod(common.HexToAddress(address), newLevelInt, sequence); err != nil {
		utils.GetLogInstance().Error("Payout error", err)
		json.NewEncoder(w).Encode(res)
		return
	}
	res.Success = true
	json.NewEncoder(w).Encode(res)
}

// End triggers endGame of puzzle smart contract.
func (s *Service) End(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	address := r.FormValue("address")

	fmt.Println("Payout: address", address)
	res := &Response{
		Success: false,
	}

	if s.CreateTransactionForPayoutMethod == nil {
		json.NewEncoder(w).Encode(res)
		return
	}

	if err := s.CreateTransactionForEndMethod(common.HexToAddress(address)); err != nil {
		utils.GetLogInstance().Error("Payout error", err)
		json.NewEncoder(w).Encode(res)
		return
	}
	res.Success = true
	json.NewEncoder(w).Encode(res)
}
