package slash

import (
	"testing"

	"github.com/harmony-one/harmony/consensus/quorum"
)

func TestDidAnyoneDoubleSign(t *testing.T) {
	d := quorum.NewDecider(quorum.SuperMajorityStake)
	t.Log("Unimplemented", d)
}
