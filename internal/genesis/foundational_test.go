package genesis

import "testing"

func TestFoundationalNodeAccounts(t *testing.T) {
	for name, accounts := range map[string][]DeployAccount{
		"V0":   FoundationalNodeAccounts,
		"V0_1": FoundationalNodeAccountsV0_1,
		"V0_2": FoundationalNodeAccountsV0_2,
		// V0_3 exempted due to historical mistakes (dups at 187/204 & 202/205)
		//"V0_3": FoundationalNodeAccountsV0_3,
		"V0_4": FoundationalNodeAccountsV0_4,
	} {
		t.Run(name, func(t *testing.T) { testDeployAccounts(t, accounts) })
	}
}
