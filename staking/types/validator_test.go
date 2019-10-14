package types

import (
	"reflect"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/assert"
)

// Constants of the key into Storage field of Object in database
var (
	AddressKey              = common.BytesToHash(crypto.Keccak256([]byte("Address")))
	BlsPubKey               = common.BytesToHash(crypto.Keccak256([]byte("ValidatingPubKey")))
	StakeKey                = common.BytesToHash(crypto.Keccak256([]byte("Stake")))
	UnboundingHeightKey     = common.BytesToHash(crypto.Keccak256([]byte("UnbondingHeight")))
	MinSelfDeleKey          = common.BytesToHash(crypto.Keccak256([]byte("MinSelfDelegation")))
	ActiveKey               = common.BytesToHash(crypto.Keccak256([]byte("IsActive")))
	UpdateHeightKey         = common.BytesToHash(crypto.Keccak256([]byte("UpdateHeight")))
	DescriptionNameKey      = common.BytesToHash(crypto.Keccak256([]byte("Name")))
	DescriptionIdentityKey  = common.BytesToHash(crypto.Keccak256([]byte("Identity")))
	DescriptionWebsiteKey   = common.BytesToHash(crypto.Keccak256([]byte("Website")))
	DescriptionContactKey   = common.BytesToHash(crypto.Keccak256([]byte("SecurityContact")))
	DescriptionDetailsKey   = common.BytesToHash(crypto.Keccak256([]byte("Details")))
	CommissionRateKey       = common.BytesToHash(crypto.Keccak256([]byte("Rate")))
	CommissionMaxRateKey    = common.BytesToHash(crypto.Keccak256([]byte("MaxRate")))
	CommissionChangeRateKey = common.BytesToHash(crypto.Keccak256([]byte("MaxChangeRate")))
)

func TestKeyMatch(t *testing.T) {
	names := []common.Hash{
		AddressKey,
		BlsPubKey,
		StakeKey,
		UnboundingHeightKey,
		MinSelfDeleKey,
		ActiveKey,
		UpdateHeightKey,
		DescriptionNameKey,
		DescriptionIdentityKey,
		DescriptionWebsiteKey,
		DescriptionContactKey,
		DescriptionDetailsKey,
		CommissionRateKey,
		CommissionMaxRateKey,
		CommissionChangeRateKey,
	}

	v := reflect.TypeOf((*Validator)(nil)).Elem()

	res := []common.Hash{}
	queue := []reflect.StructField{}
	for i := 0; i < v.NumField(); i++ {
		queue = append(queue, v.Field(i))
	}

	for len(queue) > 0 {
		item := queue[0]
		queue = queue[1:]
		if item.Anonymous {
			for j := 0; j < item.Type.NumField(); j++ {
				queue = append(queue, item.Type.Field(j))
			}
		} else {
			hash := common.BytesToHash(crypto.Keccak256([]byte(item.Name)))
			res = append(res, hash)
		}
	}
	assert.Equal(t, len(res), len(names))
	for i := 0; i < len(res); i++ {
		assert.Equal(t, res[i], names[i])
	}
}
