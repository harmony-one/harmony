package committee

import "math/big"

// AssignerByEpoch ..
func AssignerByEpoch(epoch *big.Int) Assigner {
	if isHarmonyGenesis(epoch) {
		return GenesisAssigner
	}
	return nil
	// switch moment := epoch.Cmp(big.Int(0)); p {
	// case
	// 	return GenesisAssigner
	// }
}
