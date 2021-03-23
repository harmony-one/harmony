package accounts

import (
	"github.com/harmony-one/harmony/internal/bech32"
	"github.com/harmony-one/harmony/internal/common"
	"github.com/pkg/errors"
)

// MustBech32ToAddressH is a wrapper for casting ethCommon.Address to harmony's common.Address
func MustBech32ToAddressH(b32 string) common.Address {
	return common.Address(common.MustBech32ToAddress(b32))
}

// Bech32ToAddressH decodes the given bech32 address.
func Bech32ToAddressH(b32 string) (addr common.Address, err error) {
	var hrp string
	err = ParseBech32AddrH(b32, &hrp, &addr)
	if err == nil && hrp != common.Bech32AddressHRP {
		err = errors.Errorf("%#v is not a %#v address", b32, common.Bech32AddressHRP)
	}
	return
}

// ParseBech32AddrH is another wrapper
func ParseBech32AddrH(b32 string, hrp *string, addr *common.Address) error {
	h, b, err := bech32.DecodeAndConvert(b32)
	if err != nil {
		return errors.Wrapf(err, "cannot decode %#v as bech32 address", b32)
	}
	if len(b) != common.AddressLength {
		return errors.Errorf("decoded bech32 %#v has invalid length %d",
			b32, len(b))
	}
	*hrp = h
	addr.SetBytes(b)
	return nil
}
