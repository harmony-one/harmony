package remote

import (
	"bytes"
	"context"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/harmony-one/harmony/internal/tikv/common"
	"github.com/tikv/client-go/v2/config"
	"github.com/tikv/client-go/v2/rawkv"
	"runtime/trace"
	"sync/atomic"
)

var EmptyValueStub = []byte("HarmonyTiKVEmptyValueStub")

type RemoteDatabase struct {
	client   *rawkv.Client
	readOnly bool
	isClose  uint64
}

func NewRemoteDatabase(pdAddr []string, readOnly bool) (*RemoteDatabase, error) {
	client, err := rawkv.NewClient(context.Background(), pdAddr, config.DefaultConfig().Security)
	if err != nil {
		return nil, err
	}

	db := &RemoteDatabase{
		client:   client,
		readOnly: readOnly,
	}

	return db, nil
}

func (d *RemoteDatabase) ReadOnly() {
	d.readOnly = true
}

func (d *RemoteDatabase) Has(key []byte) (bool, error) {
	data, err := d.Get(key)
	if err != nil {
		if err == common.ErrNotFound {
			return false, nil
		}
		return false, err
	} else {
		return len(data) != 0, nil
	}
}

func (d *RemoteDatabase) Get(key []byte) ([]byte, error) {
	if len(key) == 0 {
		return nil, common.ErrEmptyKey
	}

	region := trace.StartRegion(context.Background(), "tikv Get")
	defer region.End()

	get, err := d.client.Get(context.Background(), key)
	if err != nil {
		return nil, err
	}

	if len(get) == 0 {
		return nil, common.ErrNotFound
	}

	if len(get) == len(EmptyValueStub) && bytes.Compare(get, EmptyValueStub) == 0 {
		get = get[:0]
	}

	return get, nil
}

func (d *RemoteDatabase) Put(key []byte, value []byte) error {
	if len(key) == 0 {
		return common.ErrEmptyKey
	}
	if d.readOnly {
		return nil
	}

	if len(value) == 0 {
		value = EmptyValueStub
	}

	return d.client.Put(context.Background(), key, value)
}

func (d *RemoteDatabase) Delete(key []byte) error {
	if len(key) == 0 {
		return common.ErrEmptyKey
	}
	if d.readOnly {
		return nil
	}

	return d.client.Delete(context.Background(), key)
}

func (d *RemoteDatabase) NewBatch() ethdb.Batch {
	if d.readOnly {
		return newNopRemoteBatch(d)
	}

	return newRemoteBatch(d)
}

func (d *RemoteDatabase) NewIterator(start, end []byte) ethdb.Iterator {
	return newRemoteIterator(d, start, end)
}

func (d *RemoteDatabase) Stat(property string) (string, error) {
	return "", common.ErrNotFound
}

func (d *RemoteDatabase) Compact(start []byte, limit []byte) error {
	return nil
}

func (d *RemoteDatabase) Close() error {
	if atomic.CompareAndSwapUint64(&d.isClose, 0, 1) {
		return d.client.Close()
	}

	return nil
}
