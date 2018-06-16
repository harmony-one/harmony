package blockchain

// UTXOPool stores transactions and balance associated with each address.
type UTXOPool struct {
	// Mapping from address to a map of transaction id to that balance.
	// The assumption here is that one address only appears once output array in a transaction.
	utxo map[string]map[string]int
}
