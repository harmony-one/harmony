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
	GetHeaderByNumberRLPHex  = "GetHeaderByNumberRLPHex"
	GetProof                 = "GetProof"
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
