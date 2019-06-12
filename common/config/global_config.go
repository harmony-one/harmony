package config

// NetworkType describes the type of Harmony network
type NetworkType int

// Constants for NetworkType
const (
	Mainnet NetworkType = 0
	Testnet NetworkType = 1
	Devnet  NetworkType = 2
)

// Network is the type of Harmony network
var Network = Testnet
