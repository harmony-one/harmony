// SPDX-License-Identifier: GPL-3.0
pragma solidity 0.6.12;
pragma experimental ABIEncoderV2;

struct Description {
	string Name;
	string Identity;
	string Website;
	string SecurityContact;
	string Details;
}
struct CommissionRates{
	// the commission rate charged to delegators, as a fraction
	uint256 Rate;
	// maximum commission rate which validator can ever charge, as a fraction
	uint256 MaxRate;
	// maximum increase of the validator commission every epoch, as a fraction
	uint256 MaxChangeRate;
}

struct CreateValidator {
	address ValidatorAddress;
	Description _Description;
	CommissionRates _CommissionRates;
	uint256 MinSelfDelegation;
	uint256 MaxTotalDelegation;
	bytes[] SlotPubKeys;
	bytes[] SlotKeySigs;
	uint256 Amount;
}
struct EditValidator {
	address ValidatorAddress;
	Description _Description;
	uint256 CommissionRate;
	uint256 MinSelfDelegation;
	uint256 MaxTotalDelegation;
	bytes SlotKeyToRemove;
	bytes SlotKeyToAdd;
	bytes SlotKeyToAddSig;
	byte EPOSStatus;
}

struct Delegate {
	address DelegatorAddress;
	address ValidatorAddress;
	uint256 Amount;
}

struct CollectRewards {
	address DelegatorAddress;
}


// Undelegate - type for removing delegation responsibility
struct Undelegate {
	address DelegatorAddress;
	address ValidatorAddress;
	uint256 Amount;
}

struct Undelegation {
	uint256 Amount;
	uint256 Epoch;
}

struct _Delegation {
    address	DelegatorAddress;
    uint256	Amount;
    uint256	Reward;
	Undelegation[] Undelegations;
}
struct Delegation {
    address Validator;
	_Delegation Delegation;
}

abstract contract Staking {
    Staking constant precompiled = Staking(address(uint160(252)));
    function createValidator(CreateValidator calldata stkMsg) external virtual;
    function editValidator(EditValidator calldata stkMsg) external virtual;
    function delegate(Delegate calldata stkMsg) external virtual;
    function undelegate(Undelegate calldata stkMsg) external virtual;
    function collectRewards(CollectRewards calldata stkMsg) external virtual;
    function getDelegationsByDelegator(address delegator) external view virtual returns(Delegation[] memory);
}

