package nodeconfig

import (
	"testing"

	"github.com/harmony-one/harmony/crypto/bls"

	"github.com/harmony-one/harmony/internal/blsgen"
	shardingconfig "github.com/harmony-one/harmony/internal/configs/sharding"
	"github.com/harmony-one/harmony/multibls"
	"github.com/pkg/errors"
)

func TestNodeConfigSingleton(t *testing.T) {
	// init 3 configs
	_ = GetShardConfig(2)
	// get the singleton variable
	c := GetShardConfig(Global)
	c.SetBeaconGroupID(GroupIDBeacon)
	d := GetShardConfig(Global)
	g := d.GetBeaconGroupID()
	if g != GroupIDBeacon {
		t.Errorf("GetBeaconGroupID = %v, expected = %v", g, GroupIDBeacon)
	}
}

func TestNodeConfigMultiple(t *testing.T) {
	// init 3 configs
	d := GetShardConfig(1)
	e := GetShardConfig(0)
	f := GetShardConfig(42)

	if f != nil {
		t.Errorf("expecting nil, got: %v", f)
	}

	d.SetShardGroupID("abcd")
	if d.GetShardGroupID() != "abcd" {
		t.Errorf("expecting abcd, got: %v", d.GetShardGroupID())
	}

	e.SetClientGroupID("client")
	if e.GetClientGroupID() != "client" {
		t.Errorf("expecting client, got: %v", d.GetClientGroupID())
	}
}

func TestValidateConsensusKeysForSameShard(t *testing.T) {
	// set localnet config
	networkType := "localnet"
	schedule := shardingconfig.LocalnetSchedule
	netType := NetworkType(networkType)
	SetNetworkType(netType)
	SetShardingSchedule(schedule)

	// import two keys that belong to same shard and test ValidateConsensusKeysForSameShard
	keyPath1 := "../../../.hmy/65f55eb3052f9e9f632b2923be594ba77c55543f5c58ee1454b9cfd658d25e06373b0f7d42a19c84768139ea294f6204.key"
	priKey1, err := blsgen.LoadBLSKeyWithPassPhrase(keyPath1, "")
	pubKey1 := priKey1.GetPublicKey()
	if err != nil {
		t.Error(err)
	}
	keyPath2 := "../../../.hmy/ca86e551ee42adaaa6477322d7db869d3e203c00d7b86c82ebee629ad79cb6d57b8f3db28336778ec2180e56a8e07296.key"
	priKey2, err := blsgen.LoadBLSKeyWithPassPhrase(keyPath2, "")
	pubKey2 := priKey2.GetPublicKey()
	if err != nil {
		t.Error(err)
	}
	keys := multibls.PublicKeys{}
	dummyKey := bls.SerializedPublicKey{}
	dummyKey.FromLibBLSPublicKey(pubKey1)
	keys = append(keys, bls.PublicKeyWrapper{Object: pubKey1, Bytes: dummyKey})
	dummyKey = bls.SerializedPublicKey{}
	dummyKey.FromLibBLSPublicKey(pubKey2)
	keys = append(keys, bls.PublicKeyWrapper{Object: pubKey2, Bytes: dummyKey})
	if err := GetDefaultConfig().ValidateConsensusKeysForSameShard(keys, 0); err != nil {
		t.Error("expected", nil, "got", err)
	}
	// add third key in different shard and test ValidateConsensusKeysForSameShard
	keyPath3 := "../../../.hmy/68ae289d73332872ec8d04ac256ca0f5453c88ad392730c5741b6055bc3ec3d086ab03637713a29f459177aaa8340615.key"
	priKey3, err := blsgen.LoadBLSKeyWithPassPhrase(keyPath3, "")
	pubKey3 := priKey3.GetPublicKey()
	if err != nil {
		t.Error(err)
	}
	dummyKey = bls.SerializedPublicKey{}
	dummyKey.FromLibBLSPublicKey(pubKey3)
	keys = append(keys, bls.PublicKeyWrapper{Object: pubKey3, Bytes: dummyKey})
	if err := GetDefaultConfig().ValidateConsensusKeysForSameShard(keys, 0); err == nil {
		e := errors.New("bls keys do not belong to the same shard")
		t.Error("expected", e, "got", nil)
	}
}
