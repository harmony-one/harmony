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

	// GetStorageRangesRequestCap is the cap of request of single GetStorageRanges request
	// This number has an effect on maxMsgBytes as 20MB defined in github.com/harmony-one/harmony/p2p/stream/types.
	GetStorageRangesRequestCap = 256

	// GetByteCodesRequestCap is the cap of request of single GetByteCodes request
	// This number has an effect on maxMsgBytes as 20MB defined in github.com/harmony-one/harmony/p2p/stream/types.
	GetByteCodesRequestCap = 128

	// GetTrieNodesRequestCap is the cap of request of single GetTrieNodes request
	// This number has an effect on maxMsgBytes as 20MB defined in github.com/harmony-one/harmony/p2p/stream/types.
	GetTrieNodesRequestCap = 128

	// stateLookupSlack defines the ratio by how much a state response can exceed
	// the requested limit in order to try and avoid breaking up contracts into
	// multiple packages and proving them.
	stateLookupSlack = 0.1

	// softResponseLimit is the target maximum size of replies to data retrievals.
	softResponseLimit = 2 * 1024 * 1024

	// maxCodeLookups is the maximum number of bytecodes to serve. This number is
	// there to limit the number of disk lookups.
	maxCodeLookups = 1024

	// maxTrieNodeLookups is the maximum number of state trie nodes to serve. This
	// number is there to limit the number of disk lookups.
	maxTrieNodeLookups = 1024

	// maxTrieNodeTimeSpent is the maximum time we should spend on looking up trie nodes.
	// If we spend too much time, then it's a fairly high chance of timing out
	// at the remote side, which means all the work is in vain.
	maxTrieNodeTimeSpent = 5 * time.Second

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
