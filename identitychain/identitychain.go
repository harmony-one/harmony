package identitychain

import (
	"sync"

	"github.com/simple-rules/harmony-benchmark/waitnode"
)

var mutex sync.Mutex

// IdentityChain (Blockchain) keeps Identities per epoch, currently centralized!
type IdentityChain struct {
	Identities        []*IdentityBlock
	PendingIdentities []*waitnode.WaitNode
}

func main() {
	var IDC IdentityChain

	go func() {
		genesisBlock := &IdentityBlock{0, "127.0.0.1", "8080", 0}
		mutex.Lock()
		IDC.Identities = append(IDC.Identities, genesisBlock)
		mutex.Unlock()

	}()
}
