package params

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
)

// nolint
const (
	// GasLimitBoundDivisor ...
	GasLimitBoundDivisor uint64 = 1024 // The bound divisor of the gas limit, used in update calculations.
	// MinGasLimit ...
	MinGasLimit uint64 = 5000 // Minimum the gas limit may ever be.
	// GenesisGasLimit ...
	GenesisGasLimit uint64 = 4712388 // Gas limit of the Genesis block.
	// TestGenesisGasLimit ..
	TestGenesisGasLimit uint64 = 80000000 // A Gas limit in testing of the Genesis block (set same as current mainnet)
	// MaximumExtraDataSize ...
	MaximumExtraDataSize uint64 = 32 // Maximum size extra data may be after Genesis.
	// ExpByteGas ...
	ExpByteGas uint64 = 10 // Times ceil(log256(exponent)) for the EXP instruction.
	// SloadGas ...
	SloadGas uint64 = 50 // Multiplied by the number of 32-byte words that are copied (round up) for any *COPY operation and added.
	// CallValueTransferGas ...
	CallValueTransferGas uint64 = 9000 // Paid for CALL when the value transfer is non-zero.
	// CallNewAccountGas ...
	CallNewAccountGas uint64 = 25000 // Paid for CALL when the destination address didn't exist prior.
	// TxGas ...
	TxGas uint64 = 21000 // Per transaction not creating a contract. NOTE: Not payable on data of calls between transactions.
	// TxGasXShard
	TxGasXShard uint64 = 23000 // Approximate cost for transferring native tokens across shards. Used in balance migration
	// TxGasContractCreation ...
	TxGasContractCreation uint64 = 53000 // Per transaction that creates a contract. NOTE: Not payable on data of calls between transactions.
	// TxGasValidatorCreation ...
	TxGasValidatorCreation uint64 = 5300000 // Per transaction that creates a new validator. NOTE: Not payable on data of calls between transactions.
	// TxDataZeroGas ...
	TxDataZeroGas uint64 = 4 // Per byte of data attached to a transaction that equals zero. NOTE: Not payable on data of calls between transactions.
	// QuadCoeffDiv ...
	QuadCoeffDiv uint64 = 512 // Divisor for the quadratic particle of the memory cost equation.
	// LogDataGas ...
	LogDataGas uint64 = 8 // Per byte in a LOG* operation's data.
	// CallStipend ...
	CallStipend uint64 = 2300 // Free gas given at beginning of call.

	// Sha3Gas ...
	Sha3Gas uint64 = 30 // Once per SHA3 operation.
	// Sha3WordGas ...
	Sha3WordGas uint64 = 6 // Once per word of the SHA3 operation's data.

	// SstoreSetGas ...
	SstoreSetGas uint64 = 20000 // Once per SLOAD operation.
	// SstoreResetGas ...
	SstoreResetGas uint64 = 5000 // Once per SSTORE operation if the zeroness changes from zero.
	// SstoreClearGas ...
	SstoreClearGas uint64 = 5000 // Once per SSTORE operation if the zeroness doesn't change.
	// SstoreRefundGas ...
	SstoreRefundGas uint64 = 15000 // Once per SSTORE operation if the zeroness changes to zero.

	// NetSstoreNoopGas ...
	NetSstoreNoopGas uint64 = 200 // Once per SSTORE operation if the value doesn't change.
	// NetSstoreInitGas ...
	NetSstoreInitGas uint64 = 20000 // Once per SSTORE operation from clean zero.
	// NetSstoreCleanGas ...
	NetSstoreCleanGas uint64 = 5000 // Once per SSTORE operation from clean non-zero.
	// NetSstoreDirtyGas ...
	NetSstoreDirtyGas uint64 = 200 // Once per SSTORE operation from dirty.

	// NetSstoreClearRefund ...
	NetSstoreClearRefund uint64 = 15000 // Once per SSTORE operation for clearing an originally existing storage slot
	// NetSstoreResetRefund ...
	NetSstoreResetRefund uint64 = 4800 // Once per SSTORE operation for resetting to the original non-zero value
	// NetSstoreResetClearRefund ...
	NetSstoreResetClearRefund uint64 = 19800 // Once per SSTORE operation for resetting to the original zero value

	// SstoreSentryGasEIP2200 ...
	SstoreSentryGasEIP2200 uint64 = 2300 // Minimum gas required to be present for an SSTORE call, not consumed
	// SstoreNoopGasEIP2200 ...
	SstoreNoopGasEIP2200 uint64 = 800 // Once per SSTORE operation if the value doesn't change.
	// SstoreDirtyGasEIP2200 ...
	SstoreDirtyGasEIP2200 uint64 = 800 // Once per SSTORE operation if a dirty value is changed.
	// SstoreInitGasEIP2200 ...
	SstoreInitGasEIP2200 uint64 = 20000 // Once per SSTORE operation from clean zero to non-zero
	// SstoreInitRefundEIP2200 ...
	SstoreInitRefundEIP2200 uint64 = 19200 // Once per SSTORE operation for resetting to the original zero value
	// SstoreCleanGasEIP2200 ...
	SstoreCleanGasEIP2200 uint64 = 5000 // Once per SSTORE operation from clean non-zero to something else
	// SstoreCleanRefundEIP2200 ...
	SstoreCleanRefundEIP2200 uint64 = 4200 // Once per SSTORE operation for resetting to the original non-zero value
	// SstoreClearRefundEIP2200 ...
	SstoreClearRefundEIP2200 uint64 = 15000 // Once per SSTORE operation for clearing an originally existing storage slot

	// todo(sun): implement eip2929
	ColdAccountAccessCostEIP2929 uint64 = 2600 // COLD_ACCOUNT_ACCESS_COST
	ColdSloadCostEIP2929         uint64 = 2100 // COLD_SLOAD_COST
	WarmStorageReadCostEIP2929   uint64 = 100  // WARM_STORAGE_READ_COST

	// JumpdestGas ...
	JumpdestGas uint64 = 1 // Refunded gas, once per SSTORE operation if the zeroness changes to zero.
	// EpochDuration ...
	EpochDuration uint64 = 30000 // Duration between proof-of-work epochs.
	// CallGas ...
	CallGas uint64 = 40 // Once per CALL operation & message call transaction.
	// CreateDataGas ...
	CreateDataGas uint64 = 200 //
	// CallCreateDepth ...
	CallCreateDepth uint64 = 1024 // Maximum depth of call/create stack.
	// ExpGas ...
	ExpGas uint64 = 10 // Once per EXP instruction
	// LogGas ...
	LogGas uint64 = 375 // Per LOG* operation.
	// CopyGas ...
	CopyGas uint64 = 3 //
	// StackLimit ...
	StackLimit uint64 = 1024 // Maximum size of VM stack allowed.
	// TierStepGas ...
	TierStepGas uint64 = 0 // Once per operation, for a selection of them.
	// LogTopicGas ...
	LogTopicGas uint64 = 375 // Multiplied by the * of the LOG*, per LOG transaction. e.g. LOG0 incurs 0 * c_txLogTopicGas, LOG4 incurs 4 * c_txLogTopicGas.
	// CreateGas ...
	CreateGas uint64 = 32000 // Once per CREATE operation & contract-creation transaction.
	// Create2Gas ...
	Create2Gas uint64 = 32000 // Once per CREATE2 operation
	// SelfdestructRefundGas ...
	SelfdestructRefundGas uint64 = 24000 // Refunded following a selfdestruct operation.
	// MemoryGas ...
	MemoryGas uint64 = 3 // Times the address of the (highest referenced byte in memory + 1). NOTE: referencing happens on read, write and in instructions such as RETURN and CALL.
	// TxDataNonZeroGas ...
	TxDataNonZeroGasFrontier uint64 = 68 // Per byte of data attached to a transaction that is not equal to zero. NOTE: Not payable on data of calls between transactions.
	// TxDataNonZeroGasEIP2028 ...
	TxDataNonZeroGasEIP2028 uint64 = 16 // Per byte of non zero data attached to a transaction after EIP 2028 (part in Istanbul)

	// These have been changed during the course of the chain
	CallGasFrontier              uint64 = 40  // Once per CALL operation & message call transaction.
	CallGasEIP150                uint64 = 700 // Static portion of gas for CALL-derivates after EIP 150 (Tangerine)
	BalanceGasFrontier           uint64 = 20  // The cost of a BALANCE operation
	BalanceGasEIP150             uint64 = 400 // The cost of a BALANCE operation after Tangerine
	BalanceGasEIP1884            uint64 = 700 // The cost of a BALANCE operation after EIP 1884 (part of Istanbul)
	ExtcodeSizeGasFrontier       uint64 = 20  // Cost of EXTCODESIZE before EIP 150 (Tangerine)
	ExtcodeSizeGasEIP150         uint64 = 700 // Cost of EXTCODESIZE after EIP 150 (Tangerine)
	SloadGasFrontier             uint64 = 50
	SloadGasEIP150               uint64 = 200
	SloadGasEIP1884              uint64 = 800  // Cost of SLOAD after EIP 1884 (part of Istanbul)
	ExtcodeHashGasConstantinople uint64 = 400  // Cost of EXTCODEHASH (introduced in Constantinople)
	ExtcodeHashGasEIP1884        uint64 = 700  // Cost of EXTCODEHASH after EIP 1884 (part in Istanbul)
	SelfdestructGasEIP150        uint64 = 5000 // Cost of SELFDESTRUCT post EIP 150 (Tangerine)

	// EXP has a dynamic portion depending on the size of the exponent
	ExpByteFrontier uint64 = 10 // was set to 10 in Frontier
	ExpByteEIP158   uint64 = 50 // was raised to 50 during Eip158 (Spurious Dragon)

	// Extcodecopy has a dynamic AND a static cost. This represents only the
	// static portion of the gas. It was changed during EIP 150 (Tangerine)
	ExtcodeCopyBaseFrontier uint64 = 20
	ExtcodeCopyBaseEIP150   uint64 = 700

	// CreateBySelfdestructGas is used when the refunded account is one that does
	// not exist. This logic is similar to call.
	// Introduced in Tangerine Whistle (Eip 150)
	CreateBySelfdestructGas uint64 = 25000

	// MaxCodeSize ...
	MaxCodeSize = 24576 // Maximum bytecode to permit for a contract

	HistoryServeWindow = 8192 // Number of blocks to serve historical block hashes for, EIP-2935

	// Precompiled contract gas prices
	EcrecoverGas        uint64 = 3000 // Elliptic curve sender recovery gas price
	Sha256BaseGas       uint64 = 60   // Base price for a SHA256 operation
	Sha256PerWordGas    uint64 = 12   // Per-word price for a SHA256 operation
	Ripemd160BaseGas    uint64 = 600  // Base price for a RIPEMD160 operation
	Ripemd160PerWordGas uint64 = 120  // Per-word price for a RIPEMD160 operation
	IdentityBaseGas     uint64 = 15   // Base price for a data copy operation
	IdentityPerWordGas  uint64 = 3    // Per-work price for a data copy operation
	ModExpQuadCoeffDiv  uint64 = 20   // Divisor for the quadratic particle of the big int modular exponentiation

	Bn256AddGasByzantium             uint64 = 500    // Byzantium gas needed for an elliptic curve addition
	Bn256AddGasIstanbul              uint64 = 150    // Gas needed for an elliptic curve addition
	Bn256ScalarMulGasByzantium       uint64 = 40000  // Byzantium gas needed for an elliptic curve scalar multiplication
	Bn256ScalarMulGasIstanbul        uint64 = 6000   // Gas needed for an elliptic curve scalar multiplication
	Bn256PairingBaseGasByzantium     uint64 = 100000 // Byzantium base price for an elliptic curve pairing check
	Bn256PairingBaseGasIstanbul      uint64 = 45000  // Base price for an elliptic curve pairing check
	Bn256PairingPerPointGasByzantium uint64 = 80000  // Byzantium per-point price for an elliptic curve pairing check
	Bn256PairingPerPointGasIstanbul  uint64 = 34000  // Per-point price for an elliptic curve pairing check

	//SHA3-FIPS Precompiled contracts gas price esstimation as per ethereum yellow paper appendix G
	Sha3FipsGas     uint64 = 30 // Once per SHA3-256 operation.
	Sha3FipsWordGas uint64 = 6  // Once per word of the SHA3-256 operation's data.

	Bls12381G1AddGas          uint64 = 375   // Price for BLS12-381 elliptic curve G1 point addition
	Bls12381G1MulGas          uint64 = 12000 // Price for BLS12-381 elliptic curve G1 point scalar multiplication
	Bls12381G2AddGas          uint64 = 600   // Price for BLS12-381 elliptic curve G2 point addition
	Bls12381G2MulGas          uint64 = 22500 // Price for BLS12-381 elliptic curve G2 point scalar multiplication
	Bls12381PairingBaseGas    uint64 = 37700 // Base gas price for BLS12-381 elliptic curve pairing check
	Bls12381PairingPerPairGas uint64 = 32600 // Per-point pair gas price for BLS12-381 elliptic curve pairing check
	Bls12381MapG1Gas          uint64 = 5500  // Gas price for BLS12-381 mapping field element to G1 operation
	Bls12381MapG2Gas          uint64 = 23800 // Gas price for BLS12-381 mapping field element to G2 operation
)

// nolint
var (
	DifficultyBoundDivisor = big.NewInt(2048)   // The bound divisor of the difficulty, used in the update calculations.
	GenesisDifficulty      = big.NewInt(131072) // Difficulty of the Genesis block.
	MinimumDifficulty      = big.NewInt(131072) // The minimum that the difficulty may ever be.
	DurationLimit          = big.NewInt(13)     // The decision boundary on the blocktime duration used to determine whether difficulty should go up or not.
)

// Gas discount table for BLS12-381 G1 multi exponentiation operation
var Bls12381G1MultiExpDiscountTable = [128]uint64{1000, 949, 848, 797, 764, 750, 738, 728, 719, 712, 705, 698, 692, 687, 682, 677, 673, 669, 665, 661, 658, 654, 651, 648, 645, 642, 640, 637, 635, 632, 630, 627, 625, 623, 621, 619, 617, 615, 613, 611, 609, 608, 606, 604, 603, 601, 599, 598, 596, 595, 593, 592, 591, 589, 588, 586, 585, 584, 582, 581, 580, 579, 577, 576, 575, 574, 573, 572, 570, 569, 568, 567, 566, 565, 564, 563, 562, 561, 560, 559, 558, 557, 556, 555, 554, 553, 552, 551, 550, 549, 548, 547, 547, 546, 545, 544, 543, 542, 541, 540, 540, 539, 538, 537, 536, 536, 535, 534, 533, 532, 532, 531, 530, 529, 528, 528, 527, 526, 525, 525, 524, 523, 522, 522, 521, 520, 520, 519}

// Gas discount table for BLS12-381 G2 multi exponentiation operation
var Bls12381G2MultiExpDiscountTable = [128]uint64{1000, 1000, 923, 884, 855, 832, 812, 796, 782, 770, 759, 749, 740, 732, 724, 717, 711, 704, 699, 693, 688, 683, 679, 674, 670, 666, 663, 659, 655, 652, 649, 646, 643, 640, 637, 634, 632, 629, 627, 624, 622, 620, 618, 615, 613, 611, 609, 607, 606, 604, 602, 600, 598, 597, 595, 593, 592, 590, 589, 587, 586, 584, 583, 582, 580, 579, 578, 576, 575, 574, 573, 571, 570, 569, 568, 567, 566, 565, 563, 562, 561, 560, 559, 558, 557, 556, 555, 554, 553, 552, 552, 551, 550, 549, 548, 547, 546, 545, 545, 544, 543, 542, 541, 541, 540, 539, 538, 537, 537, 536, 535, 535, 534, 533, 532, 532, 531, 530, 530, 529, 528, 528, 527, 526, 526, 525, 524, 524}

// HistoryStorageAddress is where the historical block hashes are stored.
var HistoryStorageAddress = common.HexToAddress("0x25a219378dad9b3503c8268c9ca836a52427a4fb")

// HistoryStorageCode is the code with getters for historical block hashes.
var HistoryStorageCode = common.FromHex("60203611603157600143035f35116029575f356120000143116029576120005f3506545f5260205ff35b5f5f5260205ff35b5f5ffd00")
