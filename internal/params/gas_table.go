package params

// GasTable organizes gas prices for different ethereum phases.
type GasTable struct {
	ExtcodeSize uint64
	ExtcodeCopy uint64
	ExtcodeHash uint64
	Balance     uint64
	SLoad       uint64
	Calls       uint64
	Suicide     uint64

	ExpByte uint64

	// CreateBySuicide occurs when the
	// refunded account is one that does
	// not exist. This logic is similar
	// to call. May be left nil. Nil means
	// not charged.
	CreateBySuicide uint64
}

// Variables containing gas prices for different ethereum phases.
var (
	// GasTableHomestead contain the gas prices for
	// the homestead phase.
	GasTableHomestead = GasTable{
		ExtcodeSize: 20,
		ExtcodeCopy: 20,
		Balance:     20,
		SLoad:       50,
		Calls:       40,
		Suicide:     0,
		ExpByte:     10,
	}

	// GasTableEIP150 contain the gas re-prices for
	// the EIP150 phase.
	GasTableEIP150 = GasTable{
		ExtcodeSize: 700,
		ExtcodeCopy: 700,
		Balance:     400,
		SLoad:       200,
		Calls:       700,
		Suicide:     5000,
		ExpByte:     10,

		CreateBySuicide: 25000,
	}
	// GasTableEIP158 contain the gas re-prices for
	// the EIP155/EIP158 phase.
	GasTableEIP158 = GasTable{
		ExtcodeSize: 700,
		ExtcodeCopy: 700,
		Balance:     400,
		SLoad:       200,
		Calls:       700,
		Suicide:     5000,
		ExpByte:     50,

		CreateBySuicide: 25000,
	}
	// GasTableConstantinople contain the gas re-prices for
	// the constantinople phase.
	GasTableConstantinople = GasTable{
		ExtcodeSize: 700,
		ExtcodeCopy: 700,
		ExtcodeHash: 400,
		Balance:     400,
		SLoad:       200,
		Calls:       700,
		Suicide:     5000,
		ExpByte:     50,

		CreateBySuicide: 25000,
	}
)
