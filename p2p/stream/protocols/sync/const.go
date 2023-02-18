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

	// MaxStreamFailures is the maximum allowed failures before stream gets removed
	MaxStreamFailures = 3

	// minAdvertiseInterval is the minimum advertise interval
	minAdvertiseInterval = 1 * time.Minute

	// rateLimiterGlobalRequestPerSecond is the request per second limit for all streams in the sync protocol.
	// This constant helps prevent the node resource from exhausting for being the stream sync host.
	rateLimiterGlobalRequestPerSecond = 50

	// rateLimiterSingleRequestsPerSecond is the request per second limit for a single stream in the sync protocol.
	// This constant helps prevent the node resource from exhausting from a single remote node.
	rateLimiterSingleRequestsPerSecond = 10
)
