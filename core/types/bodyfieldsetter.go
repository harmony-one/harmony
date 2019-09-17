package types

import "github.com/harmony-one/harmony/block"

// BodyFieldSetter is a body field setter.
type BodyFieldSetter struct {
	b *Body
}

// Transactions sets the Transactions field of the body.
func (bfs BodyFieldSetter) Transactions(newTransactions []*Transaction) BodyFieldSetter {
	bfs.b.SetTransactions(newTransactions)
	return bfs
}

// Uncles sets the Uncles field of the body.
func (bfs BodyFieldSetter) Uncles(newUncles []*block.Header) BodyFieldSetter {
	bfs.b.SetUncles(newUncles)
	return bfs
}

// IncomingReceipts sets the IncomingReceipts field of the body.
func (bfs BodyFieldSetter) IncomingReceipts(newIncomingReceipts CXReceiptsProofs) BodyFieldSetter {
	bfs.b.SetIncomingReceipts(newIncomingReceipts)
	return bfs
}

// Body ends the field setter chain and returns the underlying body itself.
func (bfs BodyFieldSetter) Body() *Body {
	return bfs.b
}
