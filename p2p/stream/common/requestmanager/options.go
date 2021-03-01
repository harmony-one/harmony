package requestmanager

import sttypes "github.com/harmony-one/harmony/p2p/stream/types"

// RequestOption is the additional instruction for requests.
// Currently, two options are supported:
// 1. WithHighPriority
// 2. WithBlacklist
// 3. WithWhitelist
type RequestOption func(*request)

// WithHighPriority is the request option to do request with higher priority.
// High priority requests are done first.
func WithHighPriority() RequestOption {
	return func(req *request) {
		req.priority = reqPriorityHigh
	}
}

// WithBlacklist is the request option not to assign the request to the blacklisted
// stream ID.
func WithBlacklist(blacklist []sttypes.StreamID) RequestOption {
	return func(req *request) {
		for _, stid := range blacklist {
			req.addBlacklistedStream(stid)
		}
	}
}

// WithWhitelist is the request option to restrict the request to be assigned to the
// given stream IDs.
// If a request is not with this option, all streams will be allowed.
func WithWhitelist(whitelist []sttypes.StreamID) RequestOption {
	return func(req *request) {
		for _, stid := range whitelist {
			req.addWhiteListStream(stid)
		}
	}
}
