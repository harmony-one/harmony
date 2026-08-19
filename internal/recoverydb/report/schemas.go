package report

// Shared cross-command report schemas (the hash-chained contract files, plan
// §4 "Integrity and hash-chaining"). Each command writes its own report and
// consumes upstream ones through checksum gates; keeping the schemas in one
// package lets verify-db cross-check compact-db's records without import
// cycles.

// Schema versions.
const (
	InspectSchemaV1      = "hmy-recovery-inspect-v1"
	InventorySchemaV1    = "hmy-recovery-inventory-v1"
	AgreementSchemaV1    = "hmy-recovery-baseline-agreement-v1"
	ReplaySchemaV1       = "hmy-recovery-replay-v1"
	CompactSchemaV1      = "hmy-recovery-compact-v1"
	VerificationSchemaV1 = "hmy-recovery-verification-v1"
	PackageSchemaV1      = "hmy-recovery-package-v1"
	ReleaseSchemaV1      = "hmy-recovery-release-v1"
)

// Modes for the optional metadata-reference convergence proof.
const (
	ModeReference = "reference"
	ModeInternal  = "internal"
)

// HeadTuple is a resolved head pointer.
type HeadTuple struct {
	Key       string `json:"key"` // LastBlock / LastHeader / LastFast / LastFinalized
	Hash      string `json:"hash"`
	Height    uint64 `json:"height"`
	Epoch     uint64 `json:"epoch"`
	ViewID    uint64 `json:"view_id"`
	StateRoot string `json:"state_root"`
}

// Check is one named verification check result.
type Check struct {
	ID     string `json:"id"`
	OK     bool   `json:"ok"`
	Detail string `json:"detail,omitempty"`
}

// SourceIdentity binds a report to the physical database directory it
// described (plan WS2).
type SourceIdentity struct {
	AbsolutePath string `json:"absolute_path"`
	DeviceID     uint64 `json:"device_id"`
	FileCount    uint64 `json:"file_count"`
	TotalBytes   uint64 `json:"total_bytes"`
}

// PreimageCoverage is the WS2 preimage-coverage accounting.
type PreimageCoverage struct {
	Checked                 bool   `json:"checked"`
	Required                bool   `json:"required"`
	MissingAccountPreimages uint64 `json:"missing_account_preimages"`
	MissingStoragePreimages uint64 `json:"missing_storage_preimages"`
}

// InspectReport is inspect-db's output (plan WS2).
type InspectReport struct {
	Meta

	Source             SourceIdentity `json:"source"`
	LayoutOK           bool           `json:"layout_ok"`
	GenesisHash        string         `json:"genesis_hash"`
	ChainConfigPresent bool           `json:"chain_config_present"`
	DatabaseVersion    *uint64        `json:"database_version"`

	Heads              []HeadTuple     `json:"heads"`
	HeadsAgree         bool            `json:"heads_agree"`
	CanonicalHeadMatch bool            `json:"canonical_head_match"`
	MarkerPresence     map[string]bool `json:"marker_presence"` // LastFinalized, LastPivot, SnapdbInfo, snapshot*, skeleton*, unclean-shutdown, InvalidBlock

	TargetHeight uint64 `json:"target_height,omitempty"`

	FullStateCheck    bool             `json:"full_state_check"`
	FullOffchainCheck bool             `json:"full_offchain_check"`
	Preimages         PreimageCoverage `json:"preimages"`
	DigestSet         *DigestSet       `json:"digest_set,omitempty"`

	ReplayPreflight struct {
		Ran          bool     `json:"ran"`
		FullArchival bool     `json:"full_archival"`
		Failures     []string `json:"failures,omitempty"`
	} `json:"replay_preflight"`

	BaselineGate struct {
		Ran      bool     `json:"ran"`
		Passed   bool     `json:"passed"`
		Failures []string `json:"failures,omitempty"`
	} `json:"baseline_gate"`

	Checks []Check `json:"checks"`
}

// InventoryBucket is one namespace accounting row (plan WS2, minimal
// inventory).
type InventoryBucket struct {
	Bucket       string `json:"bucket"`
	Count        uint64 `json:"count"`
	LogicalBytes uint64 `json:"logical_bytes"`
}

// InventoryReport is inventory-db's output.
type InventoryReport struct {
	Meta
	Source        SourceIdentity    `json:"source"`
	Buckets       []InventoryBucket `json:"buckets"`
	MalformedKeys []string          `json:"malformed_keys,omitempty"` // hex, capped
	TotalKeys     uint64            `json:"total_keys"`
	TotalBytes    uint64            `json:"total_bytes"`
}

// AgreementVerdict is the two-copy agreement output (plan WS2): both inspect
// reports named by SHA-256, with per-field diffs on failure.
type AgreementVerdict struct {
	Meta
	LeftReport  string   `json:"left_report_sha256"`
	RightReport string   `json:"right_report_sha256"`
	Agreed      bool     `json:"agreed"`
	Differences []string `json:"differences,omitempty"`
}

// PendingQueueClear records the WS4 step-8a intentional clear.
type PendingQueueClear struct {
	CrosslinkKeyWasPresent bool `json:"crosslink_key_was_present"`
	SlashingKeyWasPresent  bool `json:"slashing_key_was_present"`
	Cleared                bool `json:"cleared"`
}

// ReplayReport is replay-bundle's output (plan WS4).
type ReplayReport struct {
	Meta

	Destination    string `json:"destination"`
	BaselineHeight uint64 `json:"baseline_height"`
	BaselineHash   string `json:"baseline_hash"`
	RangeFrom      uint64 `json:"range_from"`
	RangeTo        uint64 `json:"range_to"`
	BlocksReplayed uint64 `json:"blocks_replayed"`

	FinalHeads []HeadTuple `json:"final_heads"`

	PendingQueues           PendingQueueClear `json:"pending_queues"`
	RuntimeCleanup          []string          `json:"runtime_cleanup"` // deleted marker keys, itemized
	TargetCertificateSHA256 string            `json:"target_certificate_sha256"`

	Gate struct {
		Passed bool    `json:"passed"`
		Checks []Check `json:"checks"`
	} `json:"gate"`

	DigestSet *DigestSet `json:"digest_set"`

	WallSeconds float64 `json:"wall_seconds"`
}

// CompactReport is compact-db's output (plan WS5).
type CompactReport struct {
	Meta

	SourceDB      string `json:"source_db"`
	DestinationDB string `json:"destination_db"`

	Window     DigestWindow `json:"window"`
	TargetHash string       `json:"target_hash"`
	StateRoot  string       `json:"state_root"`

	Counts           map[string]uint64 `json:"counts"`
	DestinationBytes uint64            `json:"destination_bytes"`
	DestinationFiles uint64            `json:"destination_files"`
	WallSeconds      float64           `json:"wall_seconds"`

	ValidatorStatsIncluded bool `json:"validator_stats_included"`

	Mode                    string            `json:"mode"` // "reference" | "internal"
	MetadataReferenceDigest string            `json:"metadata_reference_digest"`
	NormalizedSections      map[string]string `json:"normalized_sections"`
	NormalizedOutputDigest  string            `json:"normalized_output_digest"`

	LogicalKVDigest string            `json:"logical_kv_digest"`
	LogicalBuckets  map[string]Digest `json:"logical_buckets"`

	// Marker is the recovery-completion marker exactly as written (JSON
	// object mirrored; verify-db cross-checks by exact field equality).
	Marker map[string]interface{} `json:"marker"`

	DigestSet *DigestSet `json:"digest_set"`

	SizeGate struct {
		LimitBytes  uint64 `json:"limit_bytes"`
		ActualBytes uint64 `json:"actual_bytes"`
		Passed      bool   `json:"passed"`
	} `json:"size_gate"`

	JournalState string `json:"journal_state"`
}

// VerificationReport is verify-db's output (plan WS6).
type VerificationReport struct {
	Meta

	DBPath string `json:"db_path"`
	Mode   string `json:"mode"`

	Checks []Check `json:"checks"`
	Passed bool    `json:"passed"`

	DigestSet               *DigestSet `json:"digest_set"`
	LogicalKVDigest         string     `json:"logical_kv_digest"`
	NormalizedOutputDigest  string     `json:"normalized_output_digest"`
	MetadataReferenceDigest string     `json:"metadata_reference_digest"` // or "internal:none"

	CertificatesVerified uint64  `json:"certificates_verified"`
	WallSeconds          float64 `json:"wall_seconds"`

	JournalState string `json:"journal_state"` // of the verified destination
}

// ReleaseJSON is the small release.json (plan WS7). No digest of SHA256SUMS
// may appear here (round 12 finding 1 — SHA256SUMS is generated last, over
// this file too).
type ReleaseJSON struct {
	SchemaVersion string `json:"schema_version"`
	ReleaseID     string `json:"release_id"`
	Network       string `json:"network"`
	ShardID       uint32 `json:"shard_id"`
	Profile       string `json:"profile"` // "validator"

	TargetHeight     uint64 `json:"target_height"`
	TargetHash       string `json:"target_hash"`
	TargetParentHash string `json:"target_parent_hash"`
	TargetEpoch      uint64 `json:"target_epoch"`
	StateRoot        string `json:"state_root"`

	AbandonedChildHeight uint64 `json:"abandoned_child_height"`
	AbandonedChildHash   string `json:"abandoned_child_hash"`
	RejectedShard1Height uint64 `json:"rejected_shard1_height"`
	RejectedShard1Hash   string `json:"rejected_shard1_hash"`

	DatabaseFormat  string `json:"database_format"`   // "leveldb/harmony_db_0"
	StateTrieScheme string `json:"state_trie_scheme"` // "hashScheme"

	PayloadBytes uint64 `json:"payload_bytes"`
	PayloadFiles uint64 `json:"payload_files"`

	ProducerBinarySHA256     string `json:"producer_binary_sha256"`
	ProducerToolVersion      string `json:"producer_tool_version"`
	VerificationReportSHA256 string `json:"verification_report_sha256"`

	Mode                    string `json:"mode"`
	MetadataReferenceDigest string `json:"metadata_reference_digest"`
	NormalizedOutputDigest  string `json:"normalized_output_digest"`
	LogicalKVDigest         string `json:"logical_kv_digest"`

	// Optional in-place integration fields ("absent" when flags omitted,
	// revision 11: never blocking).
	RecoveryHarmonyBinarySHA256   string `json:"recovery_harmony_binary_sha256"`
	ProvisionalMinimumStartViewID string `json:"provisional_minimum_start_view_id"`

	// Hash-chain links of every upstream input.
	Inputs []ChainLink `json:"inputs"`

	CreatedAt string `json:"created_at"` // informational; excluded from release-ID derivation
}

// ChainLink mirrors integrity.InputRef for the release file (kept local so
// release.json's schema is self-contained).
type ChainLink struct {
	Name   string `json:"name"`
	SHA256 string `json:"sha256"`
}

// PackageReport is package-db's package.json output.
type PackageReport struct {
	Meta

	ReleaseID  string `json:"release_id"`
	ReleaseDir string `json:"release_dir"`

	TargetHeight uint64 `json:"target_height"`
	TargetHash   string `json:"target_hash"`

	PayloadBytes uint64 `json:"payload_bytes"`
	PayloadFiles uint64 `json:"payload_files"`
	SumsEntries  uint64 `json:"sums_entries"`

	ReleaseJSONSHA256 string `json:"release_json_sha256"`
	JournalState      string `json:"journal_state"`
}
