package p2ptypes

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

var testAddrs = []string{
	"/ip4/54.86.126.90/tcp/9850/p2p/Qmdfjtk6hPoyrH1zVD9PEH4zfWLo38dP2mDvvKXfh3tnEv",
	"/ip4/52.40.84.2/tcp/9850/p2p/QmbPVwrqWsTYXq1RxGWcxx9SWaTUCfoo1wA6wmdbduWe29",
}

func TestMultiAddressParsing(t *testing.T) {
	multiAddresses, err := StringsToMultiAddrs(testAddrs)
	assert.NoError(t, err)
	assert.Equal(t, len(testAddrs), len(multiAddresses))

	for index, multiAddress := range multiAddresses {
		assert.Equal(t, multiAddress.String(), testAddrs[index])
	}
}

func TestAddressListConversionToString(t *testing.T) {
	multiAddresses, err := StringsToMultiAddrs(testAddrs)
	assert.NoError(t, err)
	assert.Equal(t, len(testAddrs), len(multiAddresses))

	expected := strings.Join(testAddrs[:], ",")
	var addressList AddrList = multiAddresses
	assert.Equal(t, expected, addressList.String())
}

func TestAddressListConversionFromString(t *testing.T) {
	multiAddresses, err := StringsToMultiAddrs(testAddrs)
	assert.NoError(t, err)
	assert.Equal(t, len(testAddrs), len(multiAddresses))

	addressString := strings.Join(testAddrs[:], ",")
	var addressList AddrList = multiAddresses
	addressList.Set(addressString)
	assert.Equal(t, len(addressList), len(multiAddresses))
	assert.Equal(t, addressList[0], multiAddresses[0])
}
