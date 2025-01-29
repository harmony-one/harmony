package nodeconfig

import (
	"testing"

	bls_core "github.com/harmony-one/bls/ffi/go/bls"
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

	// Init localnet configs
	shardingconfig.InitLocalnetConfig(16, 16)

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
func TestShardIDFromKey(t *testing.T) {
	// set mainnet config
	networkType := "mainnet"
	schedule := shardingconfig.MainnetSchedule
	netType := NetworkType(networkType)
	SetNetworkType(netType)
	SetShardingSchedule(schedule)

	// define a static public key that should belong to shard 0 after mainnet HIP30
	pubKeyHex := "ee2474f93cba9241562efc7475ac2721ab0899edf8f7f115a656c0c1f9ef8203add678064878d174bb478fa2e6630502"
	pubKey := &bls_core.PublicKey{}
	err := pubKey.DeserializeHexStr(pubKeyHex)
	if err != nil {
		t.Error(err)
	}

	shardID, err := GetDefaultConfig().ShardIDFromKey(pubKey)
	if err != nil {
		t.Error("expected", nil, "got", err)
	}
	expectedShardID := uint32(0)
	if shardID != expectedShardID {
		t.Errorf("expected shard ID %d, got %d", expectedShardID, shardID)
	}

	// define 2 static public key that should be equal
	// and belongs to shard 1 after mainnet HIP30
	pubKeyHex1 := "76e43d5a0593235d883ed8b1a834a08107da29c64ac5388958351694dce8f183690f29672939e37548753236105ee715"
	pubKey1 := &bls_core.PublicKey{}
	err = pubKey1.DeserializeHexStr(pubKeyHex1)
	if err != nil {
		t.Error(err)
	}
	pubKeyHex2 := "c54958e723531fba4665007b74e48c5feae8f73485f97120f0aa0e4879c4677032179d6af216fd07d9786d74c92ff40f"
	pubKey2 := &bls_core.PublicKey{}
	err = pubKey2.DeserializeHexStr(pubKeyHex2)
	if err != nil {
		t.Error(err)
	}
	shardID_1, err := GetDefaultConfig().ShardIDFromKey(pubKey1)
	if err != nil {
		t.Error("expected", nil, "got", err)
	}

	shardID_2, err := GetDefaultConfig().ShardIDFromKey(pubKey2)
	if err != nil {
		t.Error("expected", nil, "got", err)
	}

	if shardID_1 != shardID_2 {
		t.Errorf("expected same shard ID, got %d and %d", shardID_1, shardID_2)
	}
	if shardID_2 != 1 {
		t.Errorf("expected shard ID 1, got %d", shardID_2)
	}
}
