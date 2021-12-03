package common

import (
	"bytes"
	"crypto/ecdsa"
	"database/sql/driver"
	"encoding/hex"
	"fmt"
	"math/big"

	ethCommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/harmony-one/harmony/internal/bech32"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/pkg/errors"
	"golang.org/x/crypto/sha3"
)

// Lengths of addresses in bytes.
const (
	// AddressLength is the expected length of the address
	AddressLength = 20
)

// Address represents the 20 byte address of an Harmony account.
type Address [AddressLength]byte

// BytesToAddress returns Address with value b.
// If b is larger than len(h), b will be cropped from the left.
func BytesToAddress(b []byte) Address {
	var a Address
	a.SetBytes(b)
	return a
}

// BigToAddress returns Address with byte values of b.
// If b is larger than len(h), b will be cropped from the left.
func BigToAddress(b *big.Int) Address { return BytesToAddress(b.Bytes()) }

// HexToAddress returns Address with byte values of s.
// If s is larger than len(h), s will be cropped from the left.
func HexToAddress(s string) Address { return BytesToAddress(utils.FromHex(s)) }

// IsBech32Address verifies whether a string can represent a valid bech32-encoded
// Harmony address or not.
func IsBech32Address(s string) bool {
	hrp, bytes, err := bech32.DecodeAndConvert(s)
	if err != nil || (hrp != "one" && hrp != "tone") || len(bytes) != AddressLength {
		return false
	}
	return true
}

// Bytes gets the string representation of the underlying address.
func (a Address) Bytes() []byte { return a[:] }

// Big converts an address to a big integer.
func (a Address) Big() *big.Int { return new(big.Int).SetBytes(a[:]) }

// Hash converts an address to a hash by left-padding it with zeros.
func (a Address) Hash() Hash { return BytesToHash(a[:]) }

// Bech32 returns an bip0173-compliant string representation of the address.
func (a Address) Bech32() string {
	unchecksummed := hex.EncodeToString(a[:])
	sha := sha3.NewLegacyKeccak256()
	sha.Write([]byte(unchecksummed))
	hash := sha.Sum(nil)

	result := []byte(unchecksummed)
	for i := 0; i < len(result); i++ {
		hashByte := hash[i/2]
		if i%2 == 0 {
			hashByte = hashByte >> 4
		} else {
			hashByte &= 0xf
		}
		if result[i] > '9' && hashByte > 7 {
			result[i] -= 32
		}
	}
	return "0x" + string(result)
}

// String implements fmt.Stringer.
func (a Address) String() string {
	return a.Bech32()
}

// Format implements fmt.Formatter, forcing the byte slice to be formatted as is,
// without going through the stringer interface used for logging.
func (a Address) Format(s fmt.State, c rune) {
	fmt.Fprintf(s, "%"+string(c), a[:])
}

// SetBytes sets the address to the value of b.
// If b is larger than len(a) it will panic.
func (a *Address) SetBytes(b []byte) {
	if len(b) > len(a) {
		b = b[len(b)-AddressLength:]
	}
	copy(a[AddressLength-len(b):], b)
}

// MarshalText returns the hex representation of a.
func (a Address) MarshalText() ([]byte, error) {
	return hexutil.Bytes(a[:]).MarshalText()
}

// UnmarshalText parses a hash in hex syntax.
func (a *Address) UnmarshalText(input []byte) error {
	return hexutil.UnmarshalFixedText("Address", input, a[:])
}

// UnmarshalJSON parses a hash in hex syntax.
func (a *Address) UnmarshalJSON(input []byte) error {
	return hexutil.UnmarshalFixedJSON(addressT, input, a[:])
}

// Scan implements Scanner for database/sql.
func (a *Address) Scan(src interface{}) error {
	srcB, ok := src.([]byte)
	if !ok {
		return fmt.Errorf("can't scan %T into Address", src)
	}
	if len(srcB) != AddressLength {
		return fmt.Errorf("can't scan []byte of len %d into Address, want %d", len(srcB), AddressLength)
	}
	copy(a[:], srcB)
	return nil
}

// Value implements valuer for database/sql.
func (a Address) Value() (driver.Value, error) {
	return a[:], nil
}

// UnprefixedAddress allows marshaling an Address without 0x prefix.
type UnprefixedAddress Address

// UnmarshalText decodes the address from hex. The 0x prefix is optional.
func (a *UnprefixedAddress) UnmarshalText(input []byte) error {
	return hexutil.UnmarshalFixedUnprefixedText("UnprefixedAddress", input, a[:])
}

// MarshalText encodes the address as hex.
func (a UnprefixedAddress) MarshalText() ([]byte, error) {
	return []byte(hex.EncodeToString(a[:])), nil
}

// TODO ek – the following functions use Ethereum addresses until we have a
//  proper abstraction set in place.

// ParseBech32Addr decodes the given bech32 address and populates the given
// human-readable-part string and address with the decoded result.
func ParseBech32Addr(b32 string, hrp *string, addr *ethCommon.Address) error {
	h, b, err := bech32.DecodeAndConvert(b32)
	if err != nil {
		return errors.Wrapf(err, "cannot decode %#v as bech32 address", b32)
	}
	if len(b) != ethCommon.AddressLength {
		return errors.Errorf("decoded bech32 %#v has invalid length %d",
			b32, len(b))
	}
	*hrp = h
	addr.SetBytes(b)
	return nil
}

// BuildBech32Addr encodes the given human-readable-part string and address
// into a bech32 address.
func BuildBech32Addr(hrp string, addr ethCommon.Address) (string, error) {
	return bech32.ConvertAndEncode(hrp, addr.Bytes())
}

// MustBuildBech32Addr encodes the given human-readable-part string and
// address into a bech32 address.  It panics on error.
func MustBuildBech32Addr(hrp string, addr ethCommon.Address) string {
	b32, err := BuildBech32Addr(hrp, addr)
	if err != nil {
		panic(err)
	}
	return b32
}

// Bech32AddressHRP is the human-readable part of the Harmony address used by
// this process.
var Bech32AddressHRP = "one"

// Bech32ToAddress decodes the given bech32 address.
func Bech32ToAddress(b32 string) (addr ethCommon.Address, err error) {
	var hrp string
	err = ParseBech32Addr(b32, &hrp, &addr)
	if err == nil && hrp != Bech32AddressHRP {
		err = errors.Errorf("%#v is not a %#v address", b32, Bech32AddressHRP)
	}
	return
}

// MustBech32ToAddress decodes the given bech32 address.  It panics on error.
func MustBech32ToAddress(b32 string) ethCommon.Address {
	addr, err := Bech32ToAddress(b32)
	if err != nil {
		panic(err)
	}
	return addr
}

// AddressToBech32 encodes the given address into bech32 format.
func AddressToBech32(addr ethCommon.Address) (string, error) {
	return BuildBech32Addr(Bech32AddressHRP, addr)
}

// MustAddressToBech32 encodes the given address into bech32 format.
// It panics on error.
func MustAddressToBech32(addr ethCommon.Address) string {
	b32, err := BuildBech32Addr(Bech32AddressHRP, addr)
	if err != nil {
		panic(err)
	}
	return b32
}

// ParseAddr parses the given address, either as bech32 or as hex.
// Return error if the address is invalid.
func ParseAddr(s string) (ethCommon.Address, error) {
	// empty address in 0x format is still a valid address
	if s == "0x0000000000000000000000000000000000000000" {
		return ethCommon.Address{}, nil
	}

	if addr, err := Bech32ToAddress(s); err == nil {
		return addr, nil
	}
	// The result can be 0x00...00 if the passing param is not a correct address.
	hex := ethCommon.HexToAddress(s)
	emptyAddr := ethCommon.Address{}
	if bytes.Compare(hex[:], emptyAddr[:]) == 0 {
		return hex, errors.Errorf("invalid address: %s", s)
	}
	return hex, nil
}

// MustGeneratePrivateKey generates a random private key for an address. It panics on error.
func MustGeneratePrivateKey() *ecdsa.PrivateKey {
	key, err := crypto.GenerateKey()
	if err != nil {
		panic(err)
	}
	return key
}
