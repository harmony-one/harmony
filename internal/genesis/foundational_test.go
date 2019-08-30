package genesis

import "testing"

func TestFoundationalNodeAccounts(t *testing.T) {
	for name, accounts := range map[string][]DeployAccount{
		"V0":   FoundationalNodeAccounts,
		"V1":   FoundationalNodeAccountsV1,
		"V1_1": FoundationalNodeAccountsV1_1,
		"V1_2": FoundationalNodeAccountsV1_2,
	} {
		t.Run(name, func(t *testing.T) { testDeployAccounts(t, accounts) })
	}
}
