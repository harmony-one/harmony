package types

// RPCTransactionError ..
type RPCTransactionError struct {
	TxHashID             string `json:"tx-hash-id"`
	StakingDirective     string `json:"directive-kind"`
	TimestampOfRejection int64  `json:"time-at-rejection"`
	ErrMessage           string `json:"error-message"`
}
