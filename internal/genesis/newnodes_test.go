package genesis

import "testing"

func TestNewNodeAccounts(t *testing.T) {
	testDeployAccounts(t, NewNodeAccounts[:])
}
