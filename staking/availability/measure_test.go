package availability

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	common2 "github.com/harmony-one/harmony/internal/common"
)

const (
	to0 = "one1zyxauxquys60dk824p532jjdq753pnsenrgmef"
	to2 = "one14438psd5vrjes7qm97jrj3t0s5l4qff5j5cn4h"
)

var (
	validatorS0Addr, validatorS2Addr = common.Address{}, common.Address{}
	addrs                            = []common.Address{}
)

func init() {
	validatorS0Addr, _ = common2.Bech32ToAddress(to0)
	validatorS2Addr, _ = common2.Bech32ToAddress(to2)
	addrs = []common.Address{validatorS0Addr, validatorS2Addr}
}

func TestCompute(t *testing.T) {
	t.Log("Unimplemented")
}
