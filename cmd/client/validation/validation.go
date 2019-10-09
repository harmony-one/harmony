package validation

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/ethereum/go-ethereum/common"
)

var (
	bech32AddressRegex = regexp.MustCompile("^one[a-zA-Z0-9]{39}")
	base16AddressRegex = regexp.MustCompile("^0x[a-fA-F0-9]{40}")
)

// ValidateAddress - validates that an address is in a correct bech32 or base16 format
func ValidateAddress(address string, commonAddress common.Address, addressType string) (bool, string) {
	var valid = true
	var errorMessage string

	if len(addressType) > 0 {
		addressType = fmt.Sprintf("%s ", addressType)
	}

	if strings.HasPrefix(address, "one") || strings.HasPrefix(address, "0x") {
		if strings.HasPrefix(address, "one") {
			matches := bech32AddressRegex.FindAllStringSubmatch(address, -1)
			if len(matches) == 0 || len(commonAddress) != 20 {
				valid = false
				errorMessage = "The %saddress you supplied (%s) is in an invalid format. Please provide a valid ONE address."
			}
		} else if strings.HasPrefix(address, "0x") {
			matches := base16AddressRegex.FindAllStringSubmatch(address, -1)
			if len(matches) == 0 || len(commonAddress) != 20 {
				valid = false
				errorMessage = "The %saddress you supplied (%s) is in an invalid format. Please provide a valid 0x address."
			}
		}
	} else {
		valid = false
		errorMessage = "The %saddress you supplied (%s) is in an invalid format. Please provide a valid address."
	}

	return valid, errorMessage
}

// ValidShard - validates that a specified shardID is valid and within bounds of the available shard count
func ValidShard(shardID int, shardCount int) bool {
	if shardID < 0 || shardID > (shardCount-1) {
		return false
	}

	return true
}
