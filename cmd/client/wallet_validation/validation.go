package validation

import (
	"github.com/ethereum/go-ethereum/common"
	"regexp"
	"strings"
)

var (
	bech32AddressRegex = regexp.MustCompile("^one[a-zA-Z0-9]{39}")
	base16AddressRegex = regexp.MustCompile("^0x[a-fA-F0-9]{40}")
)

func ValidateAddress(addressType string, address string, commonAddress common.Address) (bool, string) {
	var valid bool = true
	var errorMessage string

	if strings.HasPrefix(address, "one") || strings.HasPrefix(address, "0x") {
		if strings.HasPrefix(address, "one") {
			matches := bech32AddressRegex.FindAllStringSubmatch(address, -1)
			if len(matches) == 0 || len(commonAddress) != 20 {
				valid = false
				errorMessage = "The %s address you supplied (%s) is in an invalid format. Please provide a valid ONE address."
			}
		} else if strings.HasPrefix(address, "0x") {
			matches := base16AddressRegex.FindAllStringSubmatch(address, -1)
			if len(matches) == 0 || len(commonAddress) != 20 {
				valid = false
				errorMessage = "The %s address you supplied (%s) is in an invalid format. Please provide a valid 0x address."
			}
		}
	} else {
		valid = false
		errorMessage = "The %s address you supplied (%s) is in an invalid format. Please provide a valid address."
	}

	return valid, errorMessage
}

func ValidShard(shardID int, shardCount int) bool {
	if shardID < 0 || shardID > (shardCount-1) {
		return false
	} else {
		return true
	}
}
