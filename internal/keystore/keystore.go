package keystore

import (
	"fmt"
	"sync"

	"github.com/harmony-one/harmony/accounts"
	"github.com/harmony-one/harmony/accounts/keystore"
	"github.com/harmony-one/harmony/core/types"
)

var (
	// DefaultKeyStoreDir is the default directory of the keystore
	DefaultKeyStoreDir = ".hmy/keystore"
	onceForKeyStore    sync.Once
	scryptN            = keystore.StandardScryptN
	scryptP            = keystore.StandardScryptP
	hmyKeystore        *keystore.KeyStore
	hmyPass            string
)

// GetHmyKeyStore returns the only keystore of the node
func GetHmyKeyStore() *keystore.KeyStore {
	onceForKeyStore.Do(func() {
		hmyKeystore = keystore.NewKeyStore(DefaultKeyStoreDir, scryptN, scryptP)
	})
	return hmyKeystore
}

// SetHmyPass set the passphrase
func SetHmyPass(pass string) {
	hmyPass = pass
}

// Unlock unlocks the account using passphrase
func Unlock(account accounts.Account) {
	if hmyKeystore != nil {
		hmyKeystore.Unlock(account, hmyPass)
	}
}

// SignTx signs transaction using account key
func SignTx(account accounts.Account, tx *types.Transaction) (*types.Transaction, error) {
	if hmyKeystore != nil {
		return hmyKeystore.SignTx(account, tx, nil)
	}
	return tx, fmt.Errorf("un-initialized keystore")
}
