package explorer

/*
 * All the code here is work of progress for the sprint.
 */

// Data ...
type Data struct {
	Blocks  []Block `json:"blocks"`
	Block   Block2  `json:"block"`
	Address Address `json:"address"`
}

// Address ...
type Address struct {
	Hash    string        `json:"hash"`
	Balance float64       `json:"balance"`
	TXCount int           `json:"txCount"`
	TXs     []Transaction `json:"txs"`
}

// Transaction ...
type Transaction struct {
	ID        string  `json:"id"`
	Timestamp string  `json:"timestamp"`
	From      string  `json:"from"`
	To        string  `json:"to"`
	Value     float64 `json:"value"`
}

// Block ...
type Block struct {
	ID        string `json:"id"`
	Height    string `json:"height"`
	Timestamp string `json:"timestamp"`
	TXCount   string `json:"txCount"`
	Size      string `json:"size"`
}

// Block2 ...
type Block2 struct {
	Height     string        `json:"height"`
	Hash       string        `json:"hash"`
	TXCount    string        `json:"txCount"`
	Timestamp  string        `json:"timestamp"`
	MerkleRoot string        `json:"merkleRoot"`
	PrevBlock  RefBlock      `json:"prevBlock"`
	Bits       string        `json:"bits"`
	Bytes      string        `json:"bytes"`
	NextBlock  RefBlock      `json:"nextBlock"`
	TXs        []Transaction `json:"txs"`
}

// RefBlock ...
type RefBlock struct {
	ID     string `json:"id"`
	Height string `json:"height"`
}
