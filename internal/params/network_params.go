package params

// These are network parameters that need to be constant between clients, but
// aren't necessarily consensus related.

const (
	// BloomBitsBlocks is the number of blocks a single bloom bit section vector
	// contains on the server side.
	BloomBitsBlocks uint64 = 4096

	// BloomBitsBlocksClient is the number of blocks a single bloom bit section vector
	// contains on the light client side
	BloomBitsBlocksClient uint64 = 32768

	// BloomConfirms is the number of confirmation blocks before a bloom section is
	// considered probably final and its rotated bits are calculated.
	BloomConfirms = 256

	// CHTFrequencyClient is the block frequency for creating CHTs on the client side.
	CHTFrequencyClient = 32768

	// CHTFrequencyServer is the block frequency for creating CHTs on the server side.
	// Eventually this can be merged back with the client version, but that requires a
	// full database upgrade, so that should be left for a suitable moment.
	CHTFrequencyServer = 4096

	// BloomTrieFrequency is the block frequency for creating BloomTrie on both
	// server/client sides.
	BloomTrieFrequency = 32768

	// HelperTrieConfirmations is the number of confirmations before a client is expected
	// to have the given HelperTrie available.
	HelperTrieConfirmations = 2048

	// HelperTrieProcessConfirmations is the number of confirmations before a HelperTrie
	// is generated
	HelperTrieProcessConfirmations = 256
)
