package nodeconfig

import (
	"testing"

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
	if err != nil {
		t.Error(err)
	}
	pubKey1 := priKey1.PublicKey()
	keyPath2 := "../../../.hmy/ca86e551ee42adaaa6477322d7db869d3e203c00d7b86c82ebee629ad79cb6d57b8f3db28336778ec2180e56a8e07296.key"
	priKey2, err := blsgen.LoadBLSKeyWithPassPhrase(keyPath2, "")
	if err != nil {
		t.Error(err)
	}
	pubKey2 := priKey2.PublicKey()
	keys := multibls.PublicKeys{}
	keys = append(keys, pubKey1)
	keys = append(keys, pubKey2)
	if err := GetDefaultConfig().ValidateConsensusKeysForSameShard(keys, 0); err != nil {
		t.Error("expected", nil, "got", err)
	}
	// add third key in different shard and test ValidateConsensusKeysForSameShard
	keyPath3 := "../../../.hmy/68ae289d73332872ec8d04ac256ca0f5453c88ad392730c5741b6055bc3ec3d086ab03637713a29f459177aaa8340615.key"
	priKey3, err := blsgen.LoadBLSKeyWithPassPhrase(keyPath3, "")
	if err != nil {
		t.Error(err)
	}
	pubKey3 := priKey3.PublicKey()
	keys = append(keys, pubKey3)
	if err := GetDefaultConfig().ValidateConsensusKeysForSameShard(keys, 0); err == nil {
		e := errors.New("bls keys do not belong to the same shard")
		t.Error("expected", e, "got", nil)
	}
}

// func Test(t *testing.T) {

// 	files := []string{
// 		"02c8ff0b88f313717bc3a627d2f8bb172ba3ad3bb9ba3ecb8eed4b7c878653d3d4faf769876c528b73f343967f74a917.key",
// 		"16513c487a6bb76f37219f3c2927a4f281f9dd3fd6ed2e3a64e500de6545cf391dd973cc228d24f9bd01efe94912e714.key",
// 		"1c1fb28d2de96e82c3d9b4917eb54412517e2763112a3164862a6ed627ac62e87ce274bb4ea36e6a61fb66a15c263a06.key",
// 		"2d3d4347c5a7398fbfa74e01514f9d0fcd98606ea7d245d9c7cfc011d472c2edc36c738b78bd115dda9d25533ddfe301.key",
// 		"2d61379e44a772e5757e27ee2b3874254f56073e6bd226eb8b160371cc3c18b8c4977bd3dcb71fd57dc62bf0e143fd08.key",
// 		"40379eed79ed82bebfb4310894fd33b6a3f8413a78dc4d43b98d0adc9ef69f3285df05eaab9f2ce5f7227f8cb920e809.key",
// 		"4235d4ae2219093632c61db4f71ff0c32bdb56463845f8477c2086af1fe643194d3709575707148cad4f835f2fc4ea05.key",
// 		"49d15743b36334399f9985feb0753430a2b287b2d68b84495bbb15381854cbf01bca9d1d9f4c9c8f18509b2bfa6bd40f.key",
// 		"52ecce5f64db21cbe374c9268188f5d2cdd5bec1a3112276a350349860e35fb81f8cfe447a311e0550d961cf25cb988d.key",
// 		"576d3c48294e00d6be4a22b07b66a870ddee03052fe48a5abbd180222e5d5a1f8946a78d55b025de21635fd743bbad90.key",
// 		"63f479f249c59f0486fda8caa2ffb247209489dae009dfde6144ff38c370230963d360dffd318cfb26c213320e89a512.key",
// 		"65f55eb3052f9e9f632b2923be594ba77c55543f5c58ee1454b9cfd658d25e06373b0f7d42a19c84768139ea294f6204.key",
// 		"678ec9670899bf6af85b877058bea4fc1301a5a3a376987e826e3ca150b80e3eaadffedad0fedfa111576fa76ded980c.key",
// 		"68ae289d73332872ec8d04ac256ca0f5453c88ad392730c5741b6055bc3ec3d086ab03637713a29f459177aaa8340615.key",
// 		"776f3b8704f4e1092a302a60e84f81e476c212d6f458092b696df420ea19ff84a6179e8e23d090b9297dc041600bc100.key",
// 		"86dc2fdc2ceec18f6923b99fd86a68405c132e1005cf1df72dca75db0adfaeb53d201d66af37916d61f079f34f21fb96.key",
// 		"95117937cd8c09acd2dfae847d74041a67834ea88662a7cbed1e170350bc329e53db151e5a0ef3e712e35287ae954818.key",
// 		"a547a9bf6fdde4f4934cde21473748861a3cc0fe8bbb5e57225a29f483b05b72531f002f8187675743d819c955a86100.key",
// 		"b179c4fdc0bee7bd0b6698b792837dd13404d3f985b59d4a9b1cd0641a76651e271518b61abbb6fbebd4acf963358604.key",
// 		"c4e4708b6cf2a2ceeb59981677e9821eebafc5cf483fb5364a28fa604cc0ce69beeed40f3f03815c9e196fdaec5f1097.key",
// 		"ca86e551ee42adaaa6477322d7db869d3e203c00d7b86c82ebee629ad79cb6d57b8f3db28336778ec2180e56a8e07296.key",
// 		"e751ec995defe4931273aaebcb2cd14bf37e629c554a57d3f334c37881a34a6188a93e76113c55ef3481da23b7d7ab09.key",
// 		"eca09c1808b729ca56f1b5a6a287c6e1c3ae09e29ccf7efa35453471fcab07d9f73cee249e2b91f5ee44eb9618be3904.key",
// 		"ee2474f93cba9241562efc7475ac2721ab0899edf8f7f115a656c0c1f9ef8203add678064878d174bb478fa2e6630502.key",
// 		"f47238daef97d60deedbde5302d05dea5de67608f11f406576e363661f7dcbc4a1385948549b31a6c70f6fde8a391486.key",
// 		"fc4b9c535ee91f015efff3f32fbb9d32cdd9bfc8a837bb3eee89b8fff653c7af2050a4e147ebe5c7233dc2d5df06ee0a.key",
// 	}

// 	for i := 0; i < len(files); i++ {
// 		keyPath3 := "../../../.hmy/" + files[i]
// 		priKey, err := blsgen.LoadBLSKeyWithPassPhrase(keyPath3, "")
// 		if err != nil {
// 			continue
// 		}
// 		pubKey := priKey.PublicKey()

// 		fmt.Println("")
// 		fmt.Println("file", files[i])
// 		fmt.Printf("%x\n", priKey.ToBytes())
// 		fmt.Printf("%x\n", pubKey.ToBytes())
// 		fmt.Printf("%x\n", pubKey.ToBigEndianBytes())
// 	}

// }
