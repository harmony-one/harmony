package utils

import (
	"strings"

	"github.com/ethereum/go-ethereum/common"
	internal_common "github.com/harmony-one/harmony/internal/common"
)

// ConvertAddresses - converts to bech32 depending on the RPC version
func ConvertAddresses(from *common.Address, to *common.Address, convertToBech32 bool) (string, string, error) {
	fromAddr := strings.ToLower(from.String())
	toAddr := ""
	if to != nil {
		toAddr = strings.ToLower(to.String())
	}

	if convertToBech32 {
		return base16toBech32(from, to)
	}

	return fromAddr, toAddr, nil
}

func base16toBech32(from *common.Address, to *common.Address) (fromAddr string, toAddr string, err error) {
	if fromAddr, err = internal_common.AddressToBech32(*from); err != nil {
		return "", "", err
	}

	if to != nil {
		if toAddr, err = internal_common.AddressToBech32(*to); err != nil {
			return "", "", err
		}
	}

	return fromAddr, toAddr, nil
}
