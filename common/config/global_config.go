package config

type NetworkType int

const (
	Mainnet = 0
	Testnet = 1
	Devnet  = 2
)

var Network = Testnet
