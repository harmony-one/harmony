package prefix

import "github.com/ethereum/go-ethereum/ethdb"

type PrefixBatch struct {
	prefix []byte
	batch  ethdb.Batch
}

func newPrefixBatch(prefix []byte, batch ethdb.Batch) *PrefixBatch {
	return &PrefixBatch{prefix: prefix, batch: batch}
}

func (p *PrefixBatch) Put(key []byte, value []byte) error {
	return p.batch.Put(append(append([]byte{}, p.prefix...), key...), value)
}

func (p *PrefixBatch) Delete(key []byte) error {
	return p.batch.Delete(append(append([]byte{}, p.prefix...), key...))
}
func (p *PrefixBatch) ValueSize() int {
	return p.batch.ValueSize()
}

func (p *PrefixBatch) Write() error {
	return p.batch.Write()
}

func (p *PrefixBatch) Reset() {
	p.batch.Reset()
}

func (p *PrefixBatch) Replay(w ethdb.KeyValueWriter) error {
	return p.batch.Replay(newPrefixBatchReplay(p.prefix, w))
}
