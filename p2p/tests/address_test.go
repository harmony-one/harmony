package p2ptests

import (
	"strings"
	"testing"

	"github.com/harmony-one/harmony/p2p"
	"github.com/stretchr/testify/assert"
)

func TestMultiAddressParsing(t *testing.T) {
	t.Parallel()

	multiAddresses, err := p2p.StringsToAddrs(bootnodes)
	assert.NoError(t, err)
	assert.Equal(t, len(bootnodes), len(multiAddresses))

	for index, multiAddress := range multiAddresses {
		assert.Equal(t, multiAddress.String(), bootnodes[index])
	}
}

func TestAddressListConversionToString(t *testing.T) {
	t.Parallel()

	multiAddresses, err := p2p.StringsToAddrs(bootnodes)
	assert.NoError(t, err)
	assert.Equal(t, len(bootnodes), len(multiAddresses))

	expected := strings.Join(bootnodes[:], ",")
	var addressList p2p.AddrList = multiAddresses
	assert.Equal(t, expected, addressList.String())
}

func TestAddressListConversionFromString(t *testing.T) {
	t.Parallel()

	multiAddresses, err := p2p.StringsToAddrs(bootnodes)
	assert.NoError(t, err)
	assert.Equal(t, len(bootnodes), len(multiAddresses))

	addressString := strings.Join(bootnodes[:], ",")
	var addressList p2p.AddrList = multiAddresses
	addressList.Set(addressString)
	assert.Equal(t, len(addressList), len(multiAddresses))
	assert.Equal(t, addressList[0], multiAddresses[0])
}
