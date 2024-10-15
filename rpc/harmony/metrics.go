package rpc

import (
	prom "github.com/harmony-one/harmony/api/service/prometheus"
	"github.com/prometheus/client_golang/prometheus"
)

// rpc name const
const (
	// blockchain
	GetBlockByNumberNew      = "GetBlockByNumberNew"
	GetBlockByNumber         = "GetBlockByNumber"
	GetBlockByHashNew        = "GetBlockByHashNew"
	GetBlockByHash           = "GetBlockByHash"
	GetBlocks                = "GetBlocks"
	IsLastBlock              = "IsLastBlock"
	EpochLastBlock           = "EpochLastBlock"
	GetBlockSigners          = "GetBlockSigners"
	GetBlockReceipts         = "GetBlockReceipts"
	GetBlockSignerKeys       = "GetBlockSignerKeys"
	IsBlockSigner            = "IsBlockSigner"
	GetSignedBlocks          = "GetSignedBlocks"
	GetEpoch                 = "GetEpoch"
	GetLeader                = "GetLeader"
	GetShardingStructure     = "GetShardingStructure"
	GetBalanceByBlockNumber  = "GetBalanceByBlockNumber"
	LatestHeader             = "LatestHeader"
	GetLatestChainHeaders    = "GetLatestChainHeaders"
	GetLastCrossLinks        = "GetLastCrossLinks"
	GetHeaderByNumber        = "GetHeaderByNumber"
	GetHeaderByNumberRLPHex  = "GetHeaderByNumberRLPHex"
	GetProof                 = "GetProof"
	GetCurrentUtilityMetrics = "GetCurrentUtilityMetrics"
	GetSuperCommittees       = "GetSuperCommittees"
	GetCurrentBadBlocks      = "GetCurrentBadBlocks"
	GetTotalSupply           = "GetTotalSupply"
	GetCirculatingSupply     = "GetCirculatingSupply"
	GetStakingNetworkInfo    = "GetStakingNetworkInfo"
	InSync                   = "InSync"
	BeaconInSync             = "BeaconInSync"
	SetNodeToBackupMode      = "SetNodeToBackupMode"

	// contract
	GetCode      = "GetCode"
	GetStorageAt = "GetStorageAt"
	Call         = "Call"
	DoEvmCall    = "DoEVMCall"

	// net
	PeerCount  = "PeerCount"
	NetVersion = "Version"

	// pool
	SendRawTransaction             = "SendRawTransaction"
	SendRawStakingTransaction      = "SendRawStakingTransaction"
	GetPoolStats                   = "GetPoolStats"
	PendingTransactions            = "PendingTransactions"
	PendingStakingTransactions     = "PendingStakingTransactions"
	GetCurrentTransactionErrorSink = "GetCurrentTransactionErrorSink"
	GetCurrentStakingErrorSink     = "GetCurrentStakingErrorSink"
	GetPendingCXReceipts           = "GetPendingCXReceipts"

	// staking
	GetTotalStaking                         = "GetTotalStaking"
	GetMedianRawStakeSnapshot               = "GetMedianRawStakeSnapshot"
	GetElectedValidatorAddresses            = "GetElectedValidatorAddresses"
	GetValidators                           = "GetValidators"
	GetAllValidatorAddresses                = "GetAllValidatorAddresses"
	GetValidatorKeys                        = "GetValidatorKeys"
	GetAllValidatorInformation              = "GetAllValidatorInformation"
	GetAllValidatorInformationByBlockNumber = "GetAllValidatorInformationByBlockNumber"
	GetValidatorInformation                 = "GetValidatorInformation"
	GetValidatorInformationByBlockNumber    = "GetValidatorInformationByBlockNumber"
	GetValidatorsStakeByBlockNumber         = "GetValidatorsStakeByBlockNumber"
	GetValidatorSelfDelegation              = "GetValidatorSelfDelegation"
	GetValidatorTotalDelegation             = "GetValidatorTotalDelegation"
	GetAllDelegationInformation             = "GetAllDelegationInformation"
	GetDelegationsByDelegator               = "GetDelegationsByDelegator"
	GetDelegationsByDelegatorByBlockNumber  = "GetDelegationsByDelegatorByBlockNumber"
	GetDelegationsByValidator               = "GetDelegationsByValidator"
	GetDelegationByDelegatorAndValidator    = "GetDelegationByDelegatorAndValidator"
	GetAvailableRedelegationBalance         = "GetAvailableRedelegationBalance"

	// tracer
	TraceChain         = "TraceChain"
	TraceBlockByNumber = "TraceBlockByNumber"
	TraceBlockByHash   = "TraceBlockByHash"
	TraceBlock         = "TraceBlock"
	TraceTransaction   = "TraceTransaction"
	TraceCall          = "TraceCall"

	// tracer parity
	Block       = "Block"
	Transaction = "Transaction"

	// transaction
	GetAccountNonce                            = "GetAccountNonce"
	GetTransactionCount                        = "GetTransactionCount"
	GetTransactionsCount                       = "GetTransactionsCount"
	GetStakingTransactionsCount                = "GetStakingTransactionsCount"
	RpcEstimateGas                             = "EstimateGas"
	GetTransactionByHash                       = "GetTransactionByHash"
	GetStakingTransactionByHash                = "GetStakingTransactionByHash"
	GetTransactionsHistory                     = "GetTransactionsHistory"
	GetStakingTransactionsHistory              = "GetStakingTransactionsHistory"
	GetBlockTransactionCountByNumber           = "GetBlockTransactionCountByNumber"
	GetBlockTransactionCountByHash             = "GetBlockTransactionCountByHash"
	GetTransactionByBlockNumberAndIndex        = "GetTransactionByBlockNumberAndIndex"
	GetTransactionByBlockHashAndIndex          = "GetTransactionByBlockHashAndIndex"
	GetBlockStakingTransactionCountByNumber    = "GetBlockStakingTransactionCountByNumber"
	GetBlockStakingTransactionCountByHash      = "GetBlockStakingTransactionCountByHash"
	GetStakingTransactionByBlockNumberAndIndex = "GetStakingTransactionByBlockNumberAndIndex"
	GetStakingTransactionByBlockHashAndIndex   = "GetStakingTransactionByBlockHashAndIndex"
	GetTransactionReceipt                      = "GetTransactionReceipt"
	GetCXReceiptByHash                         = "GetCXReceiptByHash"
	ResendCx                                   = "ResendCx"

	// filters
	NewPendingTransactionFilter = "NewPendingTransactionFilter"
	NewPendingTransactions      = "NewPendingTransactions"
	NewBlockFilter              = "NewBlockFilter"
	NewHeads                    = "NewHeads"
	GetFilterChanges            = "GetFilterChanges"
	Logs                        = "Logs"
	NewFilter                   = "NewFilter"
	GetLogs                     = "GetLogs"
	UninstallFilter             = "UninstallFilter"
	GetFilterLogs               = "GetFilterLogs"

	// Web3
	ClientVersion = "ClientVersion"
)

// info type const
const (
	QueryNumber       = "query_number"
	FailedNumber      = "failed_number"
	RateLimitedNumber = "rate_limited_number"
)

func init() {
	prom.PromRegistry().MustRegister(
		rpcRateLimitCounterVec,
		rpcQueryInfoCounterVec,
		rpcRequestDurationVec,
		rpcRequestDurationGaugeVec,
	)
}

var (
	rpcRateLimitCounterVec = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "hmy",
			Subsystem: "rpc",
			Name:      "over_ratelimit",
			Help:      "number of times triggered rpc rate limit",
		},
		[]string{"limiter_name"},
	)

	rpcQueryInfoCounterVec = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "hmy",
			Subsystem: "rpc",
			Name:      "query_info",
			Help:      "different types of RPC query information",
		},
		[]string{"rpc_name", "info_type"},
	)

	rpcRequestDurationVec = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "hmy",
			Subsystem: "rpc",
			Name:      "request_delay_histogram",
			Help:      "delay in seconds to do rpc requests",
			// buckets: 50ms, 100ms, 200ms, 400ms, 800ms, 1600ms, 3200ms, +INF
			Buckets: prometheus.ExponentialBuckets(0.05, 2, 8),
		},
		[]string{"rpc_name"},
	)

	rpcRequestDurationGaugeVec = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "hmy",
		Subsystem: "rpc",
		Name:      "request_delay_gauge",
		Help:      "delay in seconds to do rpc requests",
	},
		[]string{"rpc_name"},
	)
)

func DoMetricRPCRequest(rpcName string) *prometheus.Timer {
	DoMetricRPCQueryInfo(rpcName, QueryNumber)
	pLabel := getRPCDurationPromLabel(rpcName)
	timer := prometheus.NewTimer(rpcRequestDurationVec.With(pLabel))
	return timer
}

func DoRPCRequestDuration(rpcName string, timer *prometheus.Timer) {
	pLabel := getRPCDurationPromLabel(rpcName)
	rpcRequestDurationGaugeVec.With(pLabel).Set(timer.ObserveDuration().Seconds())
}

func DoMetricRPCQueryInfo(rpcName string, infoType string) {
	pLabel := getRPCQueryInfoPromLabel(rpcName, infoType)
	rpcQueryInfoCounterVec.With(pLabel).Inc()
}

func getRPCDurationPromLabel(rpcName string) prometheus.Labels {
	return prometheus.Labels{
		"rpc_name": rpcName,
	}
}

func getRPCQueryInfoPromLabel(rpcName string, infoType string) prometheus.Labels {
	return prometheus.Labels{
		"rpc_name":  rpcName,
		"info_type": infoType,
	}
}
