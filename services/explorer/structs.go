package explorer

/*
 * All the code here is work of progress for the sprint.
 */

// Data ...
type Data struct {
	Blocks []*Block `json:"blocks"`
	// Block   Block        `json:"block"`
	Address Address `json:"address"`
	TX      Transaction
}

// Address ...
type Address struct {
	ID      string        `json:"id"`
	Balance float64       `json:"balance"`
	TXCount int           `json:"txCount"`
	TXs     []Transaction `json:"txs"`
}

// Transaction ...
type Transaction struct {
	ID        string `json:"id"`
	Timestamp string `json:"timestamp"`
	From      string `json:"from"`
	To        string `json:"to"`
	Value     string `json:"value"`
}

// BlockInfo ...
type BlockInfo struct {
	ID        string `json:"id"`
	Height    string `json:"height"`
	Timestamp string `json:"timestamp"`
	TXCount   string `json:"txCount"`
	Size      string `json:"size"`
}

// Block ...
type Block struct {
	Height     string        `json:"height"`
	ID         string        `json:"id"`
	TXCount    string        `json:"txCount"`
	Timestamp  string        `json:"timestamp"`
	MerkleRoot string        `json:"merkleRoot"`
	PrevBlock  RefBlock      `json:"prevBlock"`
	Bytes      string        `json:"bytes"`
	NextBlock  RefBlock      `json:"nextBlock"`
	TXs        []Transaction `json:"txs"`
}

// RefBlock ...
type RefBlock struct {
	ID     string `json:"id"`
	Height string `json:"height"`
}
