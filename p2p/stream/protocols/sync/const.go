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

	// Advertisement timing constants for startup mode optimization
	// Normal mode timings
	BaseTimeoutNormal          = 300 * time.Second // 5 minutes base timeout
	TimeoutIncrementStepNormal = 30 * time.Second  // 30 seconds increment
	MaxTimeoutNormal           = 600 * time.Second // 10 minutes max timeout
	BackoffTimeRatioNormal     = 5 * time.Second   // 5 seconds base backoff
	MaxBackoffNormal           = 30 * time.Second  // 30 seconds max backoff

	// Startup mode timings (faster for initial peer discovery)
	BaseTimeoutStartup          = 30 * time.Second  // 30 seconds base timeout
	TimeoutIncrementStepStartup = 10 * time.Second  // 10 seconds increment
	MaxTimeoutStartup           = 120 * time.Second // 2 minutes max timeout
	BackoffTimeRatioStartup     = 2 * time.Second   // 2 seconds base backoff
	MaxBackoffStartup           = 15 * time.Second  // 15 seconds max backoff

	// Advertisement loop timing constants
	MinSleepTimeNormal  = 30 * time.Second // Minimum sleep time in normal mode
	MaxSleepTimeNormal  = 60 * time.Minute // Maximum sleep time in normal mode
	MinSleepTimeStartup = 10 * time.Second // Minimum sleep time in startup mode
	MaxSleepTimeStartup = 2 * time.Minute  // Maximum sleep time in startup mode

	// Startup mode duration
	StartupModeDuration = 10 * time.Minute // How long to stay in startup mode

	// Adaptive timing constants
	SleepIncreasePerPeer = 1 * time.Second // Increase sleep time per peer found
	SleepDecreaseRatio   = 0.7             // Decrease sleep time by 30% when no peers found

	// DHT Request Limits - How many peers to request from DHT
	// These should be higher than target limits because DHT may return invalid peers
	// Based on stream sync configuration and realistic peer discovery ratios:
	// - Mainnet: Request 20, expect ~8 valid (40% success rate)
	// - Testnet: Request 8, expect ~2 valid (25% success rate)
	// - Devnet: Request 12, expect ~4 valid (33% success rate)
	DHTRequestLimitMainnet   = 20 // Request 20, expect ~8 valid
	DHTRequestLimitTestnet   = 10 // Request 10, expect ~3 valid
	DHTRequestLimitPangaea   = 10 // Request 10, expect ~3 valid
	DHTRequestLimitPartner   = 10 // Request 10, expect ~3 valid
	DHTRequestLimitStressnet = 12 // Request 12, expect ~4 valid
	DHTRequestLimitDevnet    = 12 // Request 12, expect ~4 valid
	DHTRequestLimitLocalnet  = 12 // Request 12, expect ~4 valid

	// Target Valid Peer Counts - How many valid peers we want to find
	// These are the actual peer counts we aim for after filtering
	// Based on stream sync configuration requirements:
	// - Mainnet: InitStreams=8, DiscSoftLowCap=8, DiscHardLowCap=6
	// - Testnet: InitStreams=3, DiscSoftLowCap=3, DiscHardLowCap=3
	// - Localnet: InitStreams=4, DiscSoftLowCap=4, DiscHardLowCap=4
	// - Partner: InitStreams=3, DiscSoftLowCap=3, DiscHardLowCap=3
	// - Else: InitStreams=4, DiscSoftLowCap=4, DiscHardLowCap=4
	TargetValidPeersMainnet   = 8 // Target 8 valid peers (matches InitStreams)
	TargetValidPeersTestnet   = 3 // Target 3 valid peers (matches InitStreams)
	TargetValidPeersPangaea   = 3 // Target 3 valid peers (testnet-like)
	TargetValidPeersPartner   = 3 // Target 3 valid peers (matches InitStreams)
	TargetValidPeersStressnet = 4 // Target 4 valid peers (else config)
	TargetValidPeersDevnet    = 4 // Target 4 valid peers (else config)
	TargetValidPeersLocalnet  = 4 // Target 4 valid peers (matches InitStreams)
)
