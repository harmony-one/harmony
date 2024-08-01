package downloader

import (
	"github.com/harmony-one/abool"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/p2p"
)

// Downloaders is the set of downloaders
type Downloaders struct {
	ds     map[uint32]*Downloader
	active *abool.AtomicBool

	config Config
}

// NewDownloaders creates Downloaders for sync of multiple blockchains
func NewDownloaders(host p2p.Host, bcs []core.BlockChain, config Config) *Downloaders {
	ds := make(map[uint32]*Downloader)
	isBeaconNode := len(bcs) == 1
	for _, bc := range bcs {
		if bc == nil {
			continue
		}
		if _, ok := ds[bc.ShardID()]; ok {
			continue
		}
		ds[bc.ShardID()] = NewDownloader(host, bc, isBeaconNode, config)
	}
	return &Downloaders{
		ds:     ds,
		active: abool.New(),
		config: config,
	}
}

// Start start the downloaders
func (ds *Downloaders) Start() {
	if ds.config.ServerOnly {
		// Run in server only mode. Do not start downloaders.
		return
	}
	ds.active.Set()
	for _, d := range ds.ds {
		d.Start()
	}
}

// Close close the downloaders
func (ds *Downloaders) Close() {
	if ds.config.ServerOnly {
		// Run in server only mode. Downloaders not started.
		return
	}
	ds.active.UnSet()
	for _, d := range ds.ds {
		d.Close()
	}
}

// DownloadAsync triggers a download
func (ds *Downloaders) DownloadAsync(shardID uint32) {
	d, ok := ds.ds[shardID]
	if !ok && d != nil {
		d.DownloadAsync()
	}
}

// GetShardDownloader get the downloader with the given shard ID
func (ds *Downloaders) GetShardDownloader(shardID uint32) *Downloader {
	return ds.ds[shardID]
}

// NumPeers returns the connected peers for each shard
func (ds *Downloaders) NumPeers() map[uint32]int {
	res := make(map[uint32]int)

	for sid, d := range ds.ds {
		res[sid] = d.NumPeers()
	}
	return res
}

// SyncStatus returns whether the given shard is doing syncing task and the target block
// number.
func (ds *Downloaders) SyncStatus(shardID uint32) (bool, uint64, uint64) {
	d, ok := ds.ds[shardID]
	if !ok {
		return false, 0, 0
	}
	return d.SyncStatus()
}

// IsActive returns whether the downloader is active
func (ds *Downloaders) IsActive() bool {
	return ds.active.IsSet()
}
