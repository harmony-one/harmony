package stagedstreamsync

import (
	"fmt"
	"testing"
	"time"

	sttypes "github.com/harmony-one/harmony/p2p/stream/types"
	"github.com/stretchr/testify/assert"
)

func TestInvalidBlockMemoryLeakPrevention(t *testing.T) {
	ib := &InvalidBlock{}

	// Test initial state
	assert.Equal(t, 0, ib.GetStreamIDCount())

	// Add some stream IDs
	for i := 0; i < 150; i++ {
		ib.addBadStream(sttypes.StreamID(fmt.Sprintf("stream%d", i)))
	}

	// Verify we have 150 stream IDs
	assert.Equal(t, 150, ib.GetStreamIDCount())

	// Clean up to keep only last 100
	ib.CleanupStreamIDs(100)
	assert.Equal(t, 100, ib.GetStreamIDCount())

	// Verify we kept the most recent ones (stream50 to stream149)
	// The first 50 should be removed
	assert.Equal(t, sttypes.StreamID("stream50"), ib.StreamID[0])
	assert.Equal(t, sttypes.StreamID("stream149"), ib.StreamID[99])

	// Test clear all
	ib.ClearStreamIDs()
	assert.Equal(t, 0, ib.GetStreamIDCount())
	assert.Equal(t, 0, len(ib.StreamID))
}

func TestStagedStreamSyncTimingsMemoryLeakPrevention(t *testing.T) {
	// Create a minimal StagedStreamSync for testing
	sss := &StagedStreamSync{
		timings: make([]Timing, 0),
	}

	// Add some timing entries
	for i := 0; i < 1500; i++ {
		sss.timings = append(sss.timings, Timing{
			stage: SyncStageID(fmt.Sprintf("stage%d", i)),
			took:  time.Duration(i) * time.Millisecond,
		})
	}

	// Verify we have 1500 timing entries
	assert.Equal(t, 1500, len(sss.timings))

	// Clear all timings (they're only needed per sync cycle for logging)
	sss.ClearTimings()
	assert.Equal(t, 0, len(sss.timings))
}

func TestInvalidBlockAddBadStreamUniqueness(t *testing.T) {
	ib := &InvalidBlock{}

	// Add the same stream ID multiple times
	streamID := sttypes.StreamID("test-stream")
	ib.addBadStream(streamID)
	ib.addBadStream(streamID)
	ib.addBadStream(streamID)

	// Should only have one entry
	assert.Equal(t, 1, ib.GetStreamIDCount())
	assert.Equal(t, streamID, ib.StreamID[0])
}

func TestMemoryLeakPreventionIntegration(t *testing.T) {
	// Test that all cleanup methods work together
	ib := &InvalidBlock{}
	sss := &StagedStreamSync{
		timings: make([]Timing, 0),
	}

	// Add data to all structures
	for i := 0; i < 200; i++ {
		ib.addBadStream(sttypes.StreamID(fmt.Sprintf("stream%d", i)))

		sss.timings = append(sss.timings, Timing{
			stage: SyncStageID(fmt.Sprintf("stage%d", i)),
			took:  time.Duration(i) * time.Millisecond,
		})
	}

	// Apply cleanup to all structures
	ib.CleanupStreamIDs(100)
	sss.ClearTimings()

	// Verify cleanup worked
	assert.Equal(t, 100, ib.GetStreamIDCount())
	assert.Equal(t, 0, len(sss.timings))
}
