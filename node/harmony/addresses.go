package node

import (
	"math/big"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	bls_core "github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/crypto/bls"
	common2 "github.com/harmony-one/harmony/internal/common"
	"github.com/harmony-one/harmony/internal/registry"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/internal/utils/lrucache"
	"github.com/harmony-one/harmony/multibls"
)

type AddressToBLSKey struct {
	// KeysToAddrs holds the addresses of bls keys run by the node
	keysToAddrs      *lrucache.Cache[uint64, map[string]common.Address]
	keysToAddrsMutex sync.Mutex

	shardID uint32
}

// NewAddressToBLSKey creates a new AddressToBLSKey
func NewAddressToBLSKey(shardID uint32) *AddressToBLSKey {
	return &AddressToBLSKey{
		keysToAddrs: lrucache.NewCache[uint64, map[string]common.Address](100),
		shardID:     shardID,
	}
}

// GetAddressForBLSKey retrieves the ECDSA address associated with bls key for epoch
func (a *AddressToBLSKey) GetAddressForBLSKey(publicKeys multibls.PublicKeys, shardState registry.FindCommitteeByID, blskey *bls_core.PublicKey, epoch *big.Int) common.Address {
	return a.GetAddresses(publicKeys, shardState, epoch)[blskey.SerializeToHexStr()]
}

// GetAddresses retrieves all ECDSA addresses of the bls keys for epoch
func (a *AddressToBLSKey) GetAddresses(publicKeys multibls.PublicKeys, shardState registry.FindCommitteeByID, epoch *big.Int) map[string]common.Address {
	// populate if new epoch
	if rs, ok := a.keysToAddrs.Get(epoch.Uint64()); ok {
		return rs
	}
	a.keysToAddrsMutex.Lock()
	a.populateSelfAddresses(publicKeys, shardState, epoch)
	a.keysToAddrsMutex.Unlock()
	if rs, ok := a.keysToAddrs.Get(epoch.Uint64()); ok {
		return rs
	}
	return make(map[string]common.Address)
}

func (a *AddressToBLSKey) populateSelfAddresses(publicKeys multibls.PublicKeys, shardState registry.FindCommitteeByID, epoch *big.Int) {
	shardID := a.shardID

	committee, err := shardState.FindCommitteeByID(shardID)
	if err != nil {
		utils.Logger().Error().Err(err).
			Int64("epoch", epoch.Int64()).
			Uint32("shard-id", shardID).
			Msg("[PopulateSelfAddresses] failed to find shard committee")
		return
	}
	keysToAddrs := map[string]common.Address{}
	for _, blskey := range publicKeys {
		blsStr := blskey.Bytes.Hex()
		shardkey := bls.FromLibBLSPublicKeyUnsafe(blskey.Object)
		if shardkey == nil {
			utils.Logger().Error().
				Int64("epoch", epoch.Int64()).
				Uint32("shard-id", shardID).
				Str("blskey", blsStr).
				Msg("[PopulateSelfAddresses] failed to get shard key from bls key")
			return
		}
		addr, err := committee.AddressForBLSKey(*shardkey)
		if err != nil {
			utils.Logger().Error().Err(err).
				Int64("epoch", epoch.Int64()).
				Uint32("shard-id", shardID).
				Str("blskey", blsStr).
				Msg("[PopulateSelfAddresses] could not find address")
			return
		}
		keysToAddrs[blsStr] = *addr
		utils.Logger().Debug().
			Int64("epoch", epoch.Int64()).
			Uint32("shard-id", shardID).
			Str("bls-key", blsStr).
			Str("address", common2.MustAddressToBech32(*addr)).
			Msg("[PopulateSelfAddresses]")
	}
	a.keysToAddrs.Set(epoch.Uint64(), keysToAddrs)
}
