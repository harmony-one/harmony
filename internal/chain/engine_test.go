package chain

import (
	"bytes"
	"fmt"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/rawdb"
	blockfactory "github.com/harmony-one/harmony/block/factory"
	"github.com/harmony-one/harmony/common/denominations"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/state"
	"github.com/harmony-one/harmony/core/vm"
	"github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/crypto/hash"
	"github.com/harmony-one/harmony/internal/params"
	"github.com/harmony-one/harmony/numeric"
	"github.com/harmony-one/harmony/staking/effective"
	staking "github.com/harmony-one/harmony/staking/types"
	staketest "github.com/harmony-one/harmony/staking/types/test"
)

type fakeReader struct {
	core.FakeChainReader
}

func makeTestAddr(item interface{}) common.Address {
	s := fmt.Sprintf("harmony-one-%v", item)
	return common.BytesToAddress([]byte(s))
}

var (
	validator1 = makeTestAddr("validator1")
	validator2 = makeTestAddr("validator2")
	delegator1 = makeTestAddr("delegator1")
	delegator2 = makeTestAddr("delegator2")
	delegator3 = makeTestAddr("delegator3")
)

var (
	defaultDesc = staking.Description{
		Name:            "SuperHero",
		Identity:        "YouWouldNotKnow",
		Website:         "Secret Website",
		SecurityContact: "LicenseToKill",
		Details:         "blah blah blah",
	}

	defaultCommissionRates = staking.CommissionRates{
		Rate:          numeric.NewDecWithPrec(1, 1),
		MaxRate:       numeric.NewDecWithPrec(9, 1),
		MaxChangeRate: numeric.NewDecWithPrec(5, 1),
	}
)

func (cr *fakeReader) ReadValidatorList() ([]common.Address, error) {
	return []common.Address{validator1, validator2}, nil
}

func getDatabase() *state.DB {
	database := rawdb.NewMemoryDatabase()
	gspec := core.Genesis{Factory: blockfactory.ForTest}
	genesis := gspec.MustCommit(database)
	chain, _ := core.NewBlockChain(database, nil, nil, nil, vm.Config{}, nil)
	db, _ := chain.StateAt(genesis.Root())
	return db
}

func generateBLSKeyAndSig() (bls.SerializedPublicKey, bls.SerializedSignature) {
	blsPriv := bls.RandPrivateKey()
	blsPub := blsPriv.GetPublicKey()
	msgHash := hash.Keccak256([]byte(staking.BLSVerificationStr))
	sig := blsPriv.SignHash(msgHash)

	var shardPub bls.SerializedPublicKey
	copy(shardPub[:], blsPub.Serialize())

	var shardSig bls.SerializedSignature
	copy(shardSig[:], sig.Serialize())

	return shardPub, shardSig
}

func sampleWrapper(address common.Address) *staking.ValidatorWrapper {
	pub, _ := generateBLSKeyAndSig()
	v := staking.Validator{
		Address:              address,
		SlotPubKeys:          []bls.SerializedPublicKey{pub},
		LastEpochInCommittee: new(big.Int),
		MinSelfDelegation:    staketest.DefaultMinSelfDel,
		MaxTotalDelegation:   staketest.DefaultMaxTotalDel,
		Commission: staking.Commission{
			CommissionRates: defaultCommissionRates,
			UpdateHeight:    big.NewInt(100),
		},
		Description:    defaultDesc,
		CreationHeight: big.NewInt(100),
	}
	// ds := staking.Delegations{
	//   staking.NewDelegation(address, big.NewInt(0)),
	// }
	w := &staking.ValidatorWrapper{
		Validator:   v,
		BlockReward: big.NewInt(0),
	}
	w.Counters.NumBlocksSigned = common.Big0
	w.Counters.NumBlocksToSign = common.Big0
	return w
}

func TestPruneStaleStakingData(t *testing.T) {
	blockFactory := blockfactory.ForTest
	header := blockFactory.NewHeader(common.Big0) // epoch
	chain := fakeReader{core.FakeChainReader{InternalConfig: params.LocalnetChainConfig}}
	db := getDatabase()
	// now make the two wrappers and store them
	wrapper := sampleWrapper(validator1)
	wrapper.Status = effective.Inactive
	wrapper.Delegations = staking.Delegations{
		staking.NewDelegation(wrapper.Address, big.NewInt(0)),
		staking.NewDelegation(delegator1, big.NewInt(0)),
		staking.NewDelegation(delegator2, big.NewInt(0)),
		staking.NewDelegation(delegator3, new(big.Int).Mul(big.NewInt(denominations.One), big.NewInt(100))),
	}
	if err := wrapper.Delegations[3].Undelegate(
		big.NewInt(2), new(big.Int).Mul(big.NewInt(denominations.One), big.NewInt(100)), nil,
	); err != nil {
		t.Fatalf("Got error %v", err)
	}
	if wrapper.Delegations[3].Amount.Cmp(common.Big0) != 0 {
		t.Fatalf("Expected 0 delegation but got %v", wrapper.Delegations[3].Amount)
	}
	if err := db.UpdateValidatorWrapper(wrapper.Address, wrapper); err != nil {
		t.Fatalf("Got error %v", err)
	}
	wrapper = sampleWrapper(validator2)
	wrapper.Status = effective.Active
	wrapper.Delegations = staking.Delegations{
		staking.NewDelegation(wrapper.Address, new(big.Int).Mul(big.NewInt(denominations.One), big.NewInt(10000))),
		staking.NewDelegation(delegator1, new(big.Int).Mul(big.NewInt(denominations.One), big.NewInt(100))),
		staking.NewDelegation(delegator2, big.NewInt(0)),
		staking.NewDelegation(delegator3, big.NewInt(0)),
		staking.NewDelegation(validator1, big.NewInt(0)),
	}
	wrapper.Delegations[3].Reward = common.Big257
	if err := db.UpdateValidatorWrapper(wrapper.Address, wrapper); err != nil {
		t.Fatalf("Got error %v", err)
	}
	// we expect
	// (1) validator1 to show up with validator2 only (and not validator1 where the delegation is 0)
	// (2) delegator1 to show up with validator1 only (validator2 has amount)
	// (3) delegator2 to show up with both validator1 and validator2
	// (4) delegator3 to show up with neither validator1 (undelegation) nor validator2 (reward)
	delegationsToRemove := make(map[common.Address][]common.Address, 0)
	if err := pruneStaleStakingData(&chain, header, db, delegationsToRemove); err != nil {
		t.Fatalf("Got error %v", err)
	}
	if toRemove, ok := delegationsToRemove[validator1]; ok {
		if len(toRemove) != 1 {
			t.Errorf("Unexpected # of removals for validator1 %d", len(toRemove))
		}
		if len(toRemove) > 0 {
			for _, validatorAddress := range toRemove {
				if bytes.Equal(validatorAddress.Bytes(), validator1.Bytes()) {
					t.Errorf("Found validator1 being removed from validator1's delegations")
				}
			}
		}
	}
	if toRemove, ok := delegationsToRemove[delegator1]; ok {
		if len(toRemove) != 1 {
			t.Errorf("Unexpected # of removals for delegator1 %d", len(toRemove))
		}
		if len(toRemove) > 0 {
			for _, validatorAddress := range toRemove {
				if !bytes.Equal(validatorAddress.Bytes(), validator1.Bytes()) {
					t.Errorf("Unexpected removal for delegator1; validator1 %s, validator2 %s, validatorAddress %s",
						validator1.Hex(),
						validator2.Hex(),
						validatorAddress.Hex(),
					)
				}
			}
		}
	}
	if toRemove, ok := delegationsToRemove[delegator2]; ok {
		if len(toRemove) != 2 {
			t.Errorf("Unexpected # of removals for delegator2 %d", len(toRemove))
		}
		if len(toRemove) > 0 {
			for _, validatorAddress := range toRemove {
				if !(bytes.Equal(validatorAddress.Bytes(), validator1.Bytes()) ||
					bytes.Equal(validatorAddress.Bytes(), validator2.Bytes())) {
					t.Errorf("Unexpected removal for delegator2; validator1 %s, validator2 %s, validatorAddress %s",
						validator1.Hex(),
						validator2.Hex(),
						validatorAddress.Hex(),
					)
				}
			}
		}
	}
	if toRemove, ok := delegationsToRemove[delegator3]; ok {
		if len(toRemove) != 0 {
			t.Errorf("Unexpected # of removals for delegator3 %d", len(toRemove))
		}
	}
}
