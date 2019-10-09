package main

import (
	"testing"

	"github.com/harmony-one/harmony/internal/common"
)

func TestIsValidAddress(t *testing.T) {
	tests := []struct {
		str string
		exp bool
	}{
		{"one1ay37rp2pc3kjarg7a322vu3sa8j9puahg679z3", true},
		{"0x7c41E0668B551f4f902cFaec05B5Bdca68b124CE", true},
		{"onefoofoo", false},
		{"0xbarbar", false},
		{"dsasdadsasaadsas", false},
		{"32312123213213212321", false},
	}

	for _, test := range tests {
		valid, _ := validateAddress(test.str, common.ParseAddr(test.str), "sender")

		if valid != test.exp {
			t.Errorf("validateAddress(\"%s\") returned %v, expected %v", test.str, valid, test.exp)
		}
	}
}

func TestIsValidShard(t *testing.T) {
	readProfile("local")

	tests := []struct {
		shardID int
		exp     bool
	}{
		{0, true},
		{1, true},
		{-1, false},
		{99, false},
	}

	for _, test := range tests {
		valid := validShard(test.shardID, walletProfile.Shards)

		if valid != test.exp {
			t.Errorf("validShard(%d) returned %v, expected %v", test.shardID, valid, test.exp)
		}
	}
}
