package consensus

// timeout constant
const (
	maxLogSize uint32 = 1000
	// threshold between received consensus message blockNum and my blockNum
	consensusBlockNumBuffer uint64 = 2
)

var (
	// NIL is the m2 type message, which suppose to be nil/empty, however
	// we cannot sign on empty message, instead we sign on some default "nil" message
	// to indicate there is no prepared message received when we start view change
	NIL = []byte{0x01}
)
