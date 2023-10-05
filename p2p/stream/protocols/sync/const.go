package sync

import "time"

const (
	// GetBlockHashesAmountCap is the cap of GetBlockHashes request
	GetBlockHashesAmountCap = 50

	// GetBlocksByNumAmountCap is the cap of request of a single GetBlocksByNum request.
	// This number has an effect on maxMsgBytes as 20MB defined in github.com/harmony-one/harmony/p2p/stream/types.
	// Since we have an assumption that rlp encoded block size is smaller than 2MB (p2p.node.MaxMessageSize),
	// so the max size of a stream message is capped at 2MB * 10 = 20MB.
	GetBlocksByNumAmountCap = 10

	// GetBlocksByHashesAmountCap is the cap of request of single GetBlocksByHashes request
	// This number has an effect on maxMsgBytes as 20MB defined in github.com/harmony-one/harmony/p2p/stream/types.
	// See comments for GetBlocksByNumAmountCap.
	GetBlocksByHashesAmountCap = 10

	// GetNodeDataCap is the cap of request of single GetNodeData request
	// This number has an effect on maxMsgBytes as 20MB defined in github.com/harmony-one/harmony/p2p/stream/types.
	GetNodeDataCap = 256

	// GetReceiptsCap is the cap of request of single GetReceipts request
	// This number has an effect on maxMsgBytes as 20MB defined in github.com/harmony-one/harmony/p2p/stream/types.
	GetReceiptsCap = 128

	// MaxStreamFailures is the maximum allowed failures before stream gets removed
	MaxStreamFailures = 5

	// FaultRecoveryThreshold is the minimum duration before it resets the previous failures
	// So, if stream hasn't had any issue for a certain amount of time since last failure, we can still trust it
	FaultRecoveryThreshold = 30 * time.Minute

	// minAdvertiseInterval is the minimum advertise interval
	minAdvertiseInterval = 1 * time.Minute

	// rateLimiterGlobalRequestPerSecond is the request per second limit for all streams in the sync protocol.
	// This constant helps prevent the node resource from exhausting for being the stream sync host.
	rateLimiterGlobalRequestPerSecond = 50

	// rateLimiterSingleRequestsPerSecond is the request per second limit for a single stream in the sync protocol.
	// This constant helps prevent the node resource from exhausting from a single remote node.
	rateLimiterSingleRequestsPerSecond = 10
)
