package keystore

import (
	"sync"

	"github.com/harmony-one/harmony/accounts/keystore"
)

var (
	// DefaultKeyStoreDir is the default directory of the keystore
	DefaultKeyStoreDir = ".hmy/keystore"
	onceForKeyStore    sync.Once
	scryptN            = keystore.StandardScryptN
	scryptP            = keystore.StandardScryptP
	hmyKeystore        *keystore.KeyStore
)

// GetHmyKeyStore returns the only keystore of the node
func GetHmyKeyStore() *keystore.KeyStore {
	onceForKeyStore.Do(func() {
		hmyKeystore = keystore.NewKeyStore(DefaultKeyStoreDir, scryptN, scryptP)
	})
	return hmyKeystore
}
