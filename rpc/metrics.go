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
	GetShardingStructure     = "GetShardingStructure"
	GetBalanceByBlockNumber  = "GetBalanceByBlockNumber"
	LatestHeader             = "LatestHeader"
	GetLatestChainHeaders    = "GetLatestChainHeaders"
	GetLastCrossLinks        = "GetLastCrossLinks"
	GetHeaderByNumber        = "GetHeaderByNumber"
	GetCurrentUtilityMetrics = "GetCurrentUtilityMetrics"
	GetSuperCommittees       = "GetSuperCommittees"
	GetCurrentBadBlocks      = "GetCurrentBadBlocks"
	GetStakingNetworkInfo    = "GetStakingNetworkInfo"

	// contract
	GetCode      = "GetCode"
	GetStorageAt = "GetStorageAt"
	DoEvmCall    = "DoEVMCall"

	// net
	PeerCount = "PeerCount"

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
	GetAllValidatorInformation              = "GetAllValidatorInformation"
	GetAllValidatorInformationByBlockNumber = "GetAllValidatorInformationByBlockNumber"
	GetValidatorInformation                 = "GetValidatorInformation"
	GetValidatorInformationByBlockNumber    = "GetValidatorInformationByBlockNumber"
	GetAllDelegationInformation             = "GetAllDelegationInformation"
	GetDelegationsByDelegator               = "GetDelegationsByDelegator"
	GetDelegationsByValidator               = "GetDelegationsByValidator"
	GetDelegationByDelegatorAndValidator    = "GetDelegationByDelegatorAndValidator"

	// tracer
	TraceBlockByNumber = "TraceBlockByNumber"
	TraceBlockByHash   = "TraceBlockByHash"
	TraceBlock         = "TraceBlock"
	TraceTransaction   = "TraceTransaction"
	TraceCall          = "TraceCall"

	// transaction
	GetTransactionByHash          = "GetTransactionByHash"
	GetStakingTransactionByHash   = "GetStakingTransactionByHash"
	GetTransactionsHistory        = "GetTransactionsHistory"
	GetStakingTransactionsHistory = "GetStakingTransactionsHistory"

	// filters
	GetLogs         = "GetLogs"
	UninstallFilter = "UninstallFilter"
	GetFilterLogs   = "GetFilterLogs"
)

// info type const
const (
	QueryNumber  = "query_number"
	FailedNumber = "failed_number"
)

func init() {
	prom.PromRegistry().MustRegister(
		rpcRateLimitCounterVec,
		rpcQueryInfoCounterVec,
		rpcRequestDurationVec,
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
		[]string{"rate_limit"},
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
			Name:      "rpc_request_delay",
			Help:      "delay in seconds to do rpc requests",
			// buckets: 20ms, 40ms, 80ms, 160ms, 320ms, 640ms, 1280ms, +INF
			Buckets: prometheus.ExponentialBuckets(0.02, 2, 8),
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
