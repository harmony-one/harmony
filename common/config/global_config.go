package config

// NetworkType describes the type of Harmony network
type NetworkType int

// Constants for NetworkType
const (
	Mainnet NetworkType = 0
	Testnet NetworkType = 1
	Devnet  NetworkType = 2
)

func (network NetworkType) String() string {
	switch network {
	case 0:
		return "Mainnet"
	case 1:
		return "Testnet"
	case 2:
		return "Devnet"
	default:
		return "Unknown"
	}
}

// Network is the type of Harmony network
var Network = Testnet
