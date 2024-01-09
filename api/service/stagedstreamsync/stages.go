package stagedstreamsync

import (
	"github.com/ledgerwatch/erigon-lib/kv"
)

// SyncStageID represents the stages in the Mode.StagedSync mode
type SyncStageID string

const (
	Heads         SyncStageID = "Heads"         // Heads are downloaded
	ShortRange    SyncStageID = "ShortRange"    // short range
	SyncEpoch     SyncStageID = "SyncEpoch"     // epoch sync
	BlockBodies   SyncStageID = "BlockBodies"   // Block bodies are downloaded, TxHash and UncleHash are getting verified
	States        SyncStageID = "States"        // will construct most recent state from downloaded blocks
	StateSync     SyncStageID = "StateSync"     // State sync
	FullStateSync SyncStageID = "FullStateSync" // Full State Sync
	Receipts      SyncStageID = "Receipts"      // Receipts
	LastMile      SyncStageID = "LastMile"      // update blocks after sync and update last mile blocks as well
	Finish        SyncStageID = "Finish"        // Nominal stage after all other stages
)

// GetStageName returns the stage name in string
func GetStageName(stage string, isBeacon bool, prune bool) string {
	name := stage
	if isBeacon {
		name = "beacon_" + name
	}
	if prune {
		name = "prune_" + name
	}
	return name
}

// GetStageID returns the stage name in bytes
func GetStageID(stage SyncStageID, isBeacon bool, prune bool) []byte {
	return []byte(GetStageName(string(stage), isBeacon, prune))
}

// GetStageProgress retrieves saved progress of a given sync stage from the database
func GetStageProgress(db kv.Getter, stage SyncStageID, isBeacon bool) (uint64, error) {
	stgID := GetStageID(stage, isBeacon, false)
	v, err := db.GetOne(kv.SyncStageProgress, stgID)
	if err != nil {
		return 0, err
	}
	return unmarshalData(v)
}

// SaveStageProgress saves progress of given sync stage
func SaveStageProgress(db kv.Putter, stage SyncStageID, isBeacon bool, progress uint64) error {
	stgID := GetStageID(stage, isBeacon, false)
	return db.Put(kv.SyncStageProgress, stgID, marshalData(progress))
}

// GetStageCleanUpProgress retrieves saved progress of given sync stage from the database
func GetStageCleanUpProgress(db kv.Getter, stage SyncStageID, isBeacon bool) (uint64, error) {
	stgID := GetStageID(stage, isBeacon, true)
	v, err := db.GetOne(kv.SyncStageProgress, stgID)
	if err != nil {
		return 0, err
	}
	return unmarshalData(v)
}

// SaveStageCleanUpProgress stores the progress of the clean up for a given sync stage to the database
func SaveStageCleanUpProgress(db kv.Putter, stage SyncStageID, isBeacon bool, progress uint64) error {
	stgID := GetStageID(stage, isBeacon, true)
	return db.Put(kv.SyncStageProgress, stgID, marshalData(progress))
}
