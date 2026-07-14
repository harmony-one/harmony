package staking

import "github.com/ethereum/go-ethereum/common"

// PrecompileAddress is the EVM staking precompile at 0x…fc (decimal 252).
var PrecompileAddress = common.BytesToAddress([]byte{252})
