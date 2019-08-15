package genesis

import "testing"

func TestLocalTestAccounts(t *testing.T) {
	for name, accounts := range map[string][]DeployAccount{
		"HarmonyV0":      LocalHarmonyAccounts,
		"HarmonyV1":      LocalHarmonyAccountsV1,
		"HarmonyV2":      LocalHarmonyAccountsV2,
		"FoundationalV0": LocalFnAccounts,
		"FoundationalV1": LocalFnAccountsV1,
		"FoundationalV2": LocalFnAccountsV2,
	} {
		t.Run(name, func(t *testing.T) { testDeployAccounts(t, accounts) })
	}
}
