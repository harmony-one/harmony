package utils

import (
	"bytes"
	"testing"

	checker "gopkg.in/check.v1"
)

type BytesSuite struct{}

var _ = checker.Suite(&BytesSuite{})

func (s *BytesSuite) TestCopyBytes(c *checker.C) {
	data1 := []byte{1, 2, 3, 4}
	exp1 := []byte{1, 2, 3, 4}
	res1 := CopyBytes(data1)
	c.Assert(res1, checker.DeepEquals, exp1)
}

func (s *BytesSuite) TestLeftPadBytes(c *checker.C) {
	val1 := []byte{1, 2, 3, 4}
	exp1 := []byte{0, 0, 0, 0, 1, 2, 3, 4}

	res1 := LeftPadBytes(val1, 8)
	res2 := LeftPadBytes(val1, 2)

	c.Assert(res1, checker.DeepEquals, exp1)
	c.Assert(res2, checker.DeepEquals, val1)
}

func (s *BytesSuite) TestRightPadBytes(c *checker.C) {
	val := []byte{1, 2, 3, 4}
	exp := []byte{1, 2, 3, 4, 0, 0, 0, 0}

	resstd := RightPadBytes(val, 8)
	resshrt := RightPadBytes(val, 2)

	c.Assert(resstd, checker.DeepEquals, exp)
	c.Assert(resshrt, checker.DeepEquals, val)
}

func TestFromHex(t *testing.T) {
	input := "0x01"
	expected := []byte{1}
	result := FromHex(input)
	if !bytes.Equal(expected, result) {
		t.Errorf("Expected %x got %x", expected, result)
	}
}

func TestIsHex(t *testing.T) {
	tests := []struct {
		input string
		ok    bool
	}{
		{"", true},
		{"0", false},
		{"00", true},
		{"a9e67e", true},
		{"A9E67E", true},
		{"0xa9e67e", false},
		{"a9e67e001", false},
		{"0xHELLO_MY_NAME_IS_STEVEN_@#$^&*", false},
	}
	for _, test := range tests {
		if ok := IsHex(test.input); ok != test.ok {
			t.Errorf("isHex(%q) = %v, want %v", test.input, ok, test.ok)
		}
	}
}

func TestFromHexOddLength(t *testing.T) {
	input := "0x1"
	expected := []byte{1}
	result := FromHex(input)
	if !bytes.Equal(expected, result) {
		t.Errorf("Expected %x got %x", expected, result)
	}
}

func TestNoPrefixShortHexOddLength(t *testing.T) {
	input := "1"
	expected := []byte{1}
	result := FromHex(input)
	if !bytes.Equal(expected, result) {
		t.Errorf("Expected %x got %x", expected, result)
	}
}

func TestCopyBytes(t *testing.T) {
	expected := []byte{1, 2, 3}
	result := CopyBytes(expected)
	if !bytes.Equal(expected, result) {
		t.Errorf("Expected %x got %x", expected, result)
	}
	expected[0] = 0
	if result[0] == 0 {
		t.Errorf("should not be 0")
	}
}

func TestPadBytes(test *testing.T) {
	byteSuite := BytesSuite{}
	c := checker.C{}
	byteSuite.TestCopyBytes(&c)
	byteSuite.TestLeftPadBytes(&c)
	byteSuite.TestRightPadBytes(&c)
}
