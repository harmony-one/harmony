package downloader

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestDownloader_Integration(t *testing.T) {
	sp := newTestSyncProtocol(1000, 48, nil)
	bc := newTestBlockChain(0, nil)
	ctx, cancel := context.WithCancel(context.Background())
	c := Config{}
	c.fixValues() // use default config values

	d := &Downloader{
		bc:           bc,
		syncProtocol: sp,
		downloadC:    make(chan struct{}),
		closeC:       make(chan struct{}),
		ctx:          ctx,
		cancel:       cancel,
		config:       c,
	}

	// subscribe download event
	finishedCh := make(chan struct{}, 1)
	finishedSub := d.SubscribeDownloadFinished(finishedCh)
	startedCh := make(chan struct{}, 1)
	startedSub := d.SubscribeDownloadStarted(startedCh)
	defer finishedSub.Unsubscribe()
	defer startedSub.Unsubscribe()

	// Start the downloader
	d.Start()
	defer d.Close()

	// During bootstrap, trigger two download task: one long range, one short range.
	// The second one will not trigger start / finish events.
	if err := checkReceiveChanMulTimes(startedCh, 1, 10*time.Second); err != nil {
		t.Fatal(err)
	}
	if err := checkReceiveChanMulTimes(finishedCh, 1, 10*time.Second); err != nil {
		t.Fatal(err)
	}
	if curBN := d.bc.CurrentBlock().NumberU64(); curBN != 1000 {
		t.Fatal("blockchain not synced to the latest")
	}

	// Increase the remote block number, and trigger one download task manually
	sp.changeBlockNumber(1010)
	d.DownloadAsync()
	// We shall do short range test twice
	if err := checkReceiveChanMulTimes(startedCh, 1, 10*time.Second); err != nil {
		t.Fatal(err)
	}
	if err := checkReceiveChanMulTimes(finishedCh, 1, 10*time.Second); err != nil {
		t.Fatal(err)
	}
	if curBN := d.bc.CurrentBlock().NumberU64(); curBN != 1010 {
		t.Fatal("blockchain not synced to the latest")
	}

	// Remote block number unchanged, and trigger one download task manually
	d.DownloadAsync()
	if err := checkReceiveChanMulTimes(startedCh, 0, 10*time.Second); err != nil {
		t.Fatal(err)
	}
	if err := checkReceiveChanMulTimes(finishedCh, 0, 10*time.Second); err != nil {
		t.Fatal(err)
	}

	// At last, check number of streams, should be exactly the same as the initial number
	if sp.numStreams != 48 {
		t.Errorf("unexpected number of streams at the end: %v / %v", sp.numStreams, 48)
	}
}

func checkReceiveChanMulTimes(ch chan struct{}, times int, timeout time.Duration) error {
	t := time.Tick(timeout)

	for i := 0; i != times; i++ {
		select {
		case <-ch:
		case <-t:
			return fmt.Errorf("timed out %v", timeout)
		}
	}
	select {
	case <-ch:
		return fmt.Errorf("received an extra event")
	case <-time.After(100 * time.Millisecond):
	}
	return nil
}
