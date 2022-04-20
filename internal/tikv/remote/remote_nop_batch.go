package remote

import (
	"github.com/ethereum/go-ethereum/ethdb"
)

// NopRemoteBatch on readonly mode, write operator will be reject
type NopRemoteBatch struct {
}

func newNopRemoteBatch(db *RemoteDatabase) *NopRemoteBatch {
	return &NopRemoteBatch{}
}

func (b *NopRemoteBatch) Put(key []byte, value []byte) error {
	return nil
}

func (b *NopRemoteBatch) Delete(key []byte) error {
	return nil
}

func (b *NopRemoteBatch) ValueSize() int {
	return 0
}

func (b *NopRemoteBatch) Write() error {
	return nil
}

func (b *NopRemoteBatch) Reset() {
}

func (b *NopRemoteBatch) Replay(w ethdb.KeyValueWriter) error {
	return nil
}
