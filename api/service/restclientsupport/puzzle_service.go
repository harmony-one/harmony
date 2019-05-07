package restclientsupport

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

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
	txID, err := s.CreateTransactionForPlayMethod(key, amountInt)
	if err != nil {
		utils.GetLogInstance().Error("puzzle-play, error", err)
		json.NewEncoder(w).Encode(res)
		return
	}
	res.Success = true
	res.TxID = txID
	json.NewEncoder(w).Encode(res)
}

// Payout triggers play payout of puzzle smart contract.
func (s *Service) Payout(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	key := r.FormValue("key")
	newLevel := r.FormValue("level")
	sequence := r.FormValue("sequence")
	newLevelInt, err := strconv.Atoi(newLevel)

	fmt.Println("Payout: key", key, "new_level", newLevelInt)
	res := &Response{
		Success: false,
	}

	if s.CreateTransactionForPayoutMethod == nil {
		json.NewEncoder(w).Encode(res)
		return
	}
	txID, err := s.CreateTransactionForPayoutMethod(key, newLevelInt, sequence)

	if err != nil {
		utils.GetLogInstance().Error("Payout error", err)
		json.NewEncoder(w).Encode(res)
		return
	}
	res.Success = true
	res.TxID = txID
	json.NewEncoder(w).Encode(res)
}

// End triggers endGame of puzzle smart contract.
func (s *Service) End(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	key := r.FormValue("key")

	fmt.Println("endgame: key", key)
	res := &Response{
		Success: false,
	}

	if s.CreateTransactionForPayoutMethod == nil {
		json.NewEncoder(w).Encode(res)
		return
	}

	txID, err := s.CreateTransactionForEndMethod(key)
	if err != nil {
		utils.GetLogInstance().Error("Payout error", err)
		json.NewEncoder(w).Encode(res)
		return
	}
	res.Success = true
	res.TxID = txID
	json.NewEncoder(w).Encode(res)
}
