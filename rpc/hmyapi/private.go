package hmyapi

import (
	"github.com/ethereum/go-ethereum/common"
)

// PrivateAccountAPI provides an API to access accounts managed by this node.
// It offers methods to create, (un)lock en list accounts. Some methods accept
// passwords and are therefore considered private by default.
type PrivateAccountAPI struct {
	// am        *accounts.Manager
	nonceLock *AddrLocker
	// b         Backend
}

// NewAccount will create a new account and returns the address for the new account.
func (s *PrivateAccountAPI) NewAccount(password string) (common.Address, error) {
	// acc, err := fetchKeystore(s.am).NewAccount(password)
	// if err == nil {
	// 	return acc.Address, nil
	// }
	// return common.Address{}, err
	// TODO: port
	return common.Address{}, nil
}
