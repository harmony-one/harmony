package stagedstreamsync

import (
	"github.com/ledgerwatch/erigon-lib/kv"
)

// SyncStageID represents the sync stage ID
type SyncStageID string

const (
	Heads         SyncStageID = "Heads"         // Heads are downloaded
	ShortRange    SyncStageID = "ShortRange"    // short range
	SyncEpoch     SyncStageID = "SyncEpoch"     // epoch sync
	BlockHashes   SyncStageID = "BlockHashes"   // block hashes
	BlockBodies   SyncStageID = "BlockBodies"   // Block bodies are downloaded, TxHash and UncleHash are getting verified
	States        SyncStageID = "States"        // will construct most recent state from downloaded blocks
	StateSync     SyncStageID = "StateSync"     // State sync
	FullStateSync SyncStageID = "FullStateSync" // Full State Sync
	Receipts      SyncStageID = "Receipts"      // Receipts
	Finish        SyncStageID = "Finish"        // Nominal stage after all other stages
)

// GetStageName returns the stage name in string
func GetStageName(stage string, isBeaconShard bool, prune bool) string {
	name := stage
	if isBeaconShard {
		name = "beacon_" + name
	}
	if prune {
		name = "prune_" + name
	}
	return name
}

// GetStageID returns the stage name in bytes
func GetStageID(stage SyncStageID, isBeaconShard bool, prune bool) []byte {
	return []byte(GetStageName(string(stage), isBeaconShard, prune))
}

// GetStageProgress retrieves saved progress of a given sync stage from the database
func GetStageProgress(db kv.Getter, stage SyncStageID, isBeaconShard bool) (uint64, error) {
	stgID := GetStageID(stage, isBeaconShard, false)
	v, err := db.GetOne(StageProgressBucket, stgID)
	if err != nil {
		return 0, err
	}
	return unmarshalData(v)
}

// SaveStageProgress saves progress of given sync stage
func SaveStageProgress(db kv.Putter, stage SyncStageID, isBeaconShard bool, progress uint64) error {
	stgID := GetStageID(stage, isBeaconShard, false)
	return db.Put(StageProgressBucket, stgID, marshalData(progress))
}

// GetStageCleanUpProgress retrieves saved progress of given sync stage from the database
func GetStageCleanUpProgress(db kv.Getter, stage SyncStageID, isBeaconShard bool) (uint64, error) {
	stgID := GetStageID(stage, isBeaconShard, true)
	v, err := db.GetOne(StageProgressBucket, stgID)
	if err != nil {
		return 0, err
	}
	return unmarshalData(v)
}

// SaveStageCleanUpProgress stores the progress of the clean up for a given sync stage to the database
func SaveStageCleanUpProgress(db kv.Putter, stage SyncStageID, isBeaconShard bool, progress uint64) error {
	stgID := GetStageID(stage, isBeaconShard, true)
	return db.Put(StageProgressBucket, stgID, marshalData(progress))
}
