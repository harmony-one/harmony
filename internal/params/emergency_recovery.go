package params

const (
	// EmergencyRecoveryShard0RetainedBlock is the last canonical shard-0 block
	// retained by the emergency recovery release.
	EmergencyRecoveryShard0RetainedBlock uint64 = 92_730_034

	// EmergencyRecoveryShard1RetainedBlock is the last canonical shard-1 block
	// retained by the emergency recovery release.
	EmergencyRecoveryShard1RetainedBlock uint64 = 94_978_278

	// EmergencyRecoveryViewIDFloor is the recovery release's signed activation
	// floor for mainnet shards 0 and 1.
	EmergencyRecoveryViewIDFloor uint64 = 1_000_000_000
)
