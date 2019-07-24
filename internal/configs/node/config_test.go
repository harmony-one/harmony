package nodeconfig

import (
	"testing"

	"github.com/harmony-one/harmony/p2p"
)

func TestNodeConfigSingleton(t *testing.T) {
	// init 3 configs
	_ = GetShardConfig(2)

	// get the singleton variable
	c := GetShardConfig(Global)

	c.SetBeaconGroupID(p2p.GroupIDBeacon)

	d := GetShardConfig(Global)

	g := d.GetBeaconGroupID()

	if g != p2p.GroupIDBeacon {
		t.Errorf("GetBeaconGroupID = %v, expected = %v", g, p2p.GroupIDBeacon)
	}
}

func TestNodeConfigMultiple(t *testing.T) {
	// init 3 configs
	c := GetShardConfig(2)
	d := GetShardConfig(1)
	e := GetShardConfig(0)
	f := GetShardConfig(42)

	if f != nil {
		t.Errorf("expecting nil, got: %v", f)
	}

	if c.IsBeacon() != false {
		t.Errorf("expecting the node to not be beacon yet, got: %v", c.IsBeacon())
	}

	c.SetIsBeacon(true)

	if c.IsBeacon() != true {
		t.Errorf("expecting the node to be beacon, got: %v", c.IsBeacon())
	}

	d.SetShardGroupID("abcd")
	if d.GetShardGroupID() != "abcd" {
		t.Errorf("expecting abcd, got: %v", d.GetShardGroupID())
	}

	e.SetClientGroupID("client")
	if e.GetClientGroupID() != "client" {
		t.Errorf("expecting client, got: %v", d.GetClientGroupID())
	}

	e.SetIsClient(false)
	if e.IsClient() != false {
		t.Errorf("expecting false, got: %v", e.IsClient())
	}

	c.SetRole(NewNode)
	if c.Role() != NewNode {
		t.Errorf("expecting NewNode, got: %s", c.Role())
	}
	if c.Role().String() != "NewNode" {
		t.Errorf("expecting NewNode, got: %s", c.Role().String())
	}
}
