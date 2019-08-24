package genesis

import "testing"

func TestTNHarmonyAccounts(t *testing.T) {
	testDeployAccounts(t, TNHarmonyAccounts)
}

func TestTNFoundationalAccounts(t *testing.T) {
	testDeployAccounts(t, TNFoundationalAccounts)
}
