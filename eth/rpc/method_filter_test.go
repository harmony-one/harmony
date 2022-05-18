// Copyright 2015 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package rpc

import (
	"testing"
)

func TestRpcMethodFilter(t *testing.T) {
	method_filters_toml := `
		Allow = [ 
			"hmy_method1",
			"hmyv2_method?",
			"eth*",
			"hmy_getNetworkInfo",
		]

		Deny = [ 
			"*staking*",
			"eth_get*",
			"hmy_getNetworkInfo"
		]
	`
	b := []byte(method_filters_toml)

	var rmf RpcMethodFilter
	rmf.LoadRpcMethodFilters(b)

	tests := []struct {
		name     string
		exposure bool
	}{
		0: {"hmy_method1", true},
		1: {"hmy_method2", false},
		2: {"hmyv2_method5", true},
		3: {"hmyv2_method", false},
		4: {"eth_chainID", true},
		5: {"eth_getValidator", false},
		6: {"hmy_getStakingInfo", false},
		7: {"abc", false},
		8: {"hmy_getNetworkInfo", false},
	}

	for i, test := range tests {
		mustExpose := rmf.Expose(test.name)

		if mustExpose != test.exposure {
			t.Errorf("Test %d got unexpected value, want %t, got %t", i, test.exposure, mustExpose)
		}
	}
}

func TestRpcMethodAllowAllFilter(t *testing.T) {
	method_filters_toml := `
		Allow = [ 
			"*"
		]

		Deny = [ 
			"mtd1",
			"*staking*",
			"eth_get*",
			"regex:^hmy_[a-z]+"
		]
	`
	b := []byte(method_filters_toml)

	var rmf RpcMethodFilter
	rmf.LoadRpcMethodFilters(b)

	tests := []struct {
		name     string
		exposure bool
	}{
		0: {"mtd1", false},
		1: {"hmy_method1", false},
		2: {"hmyv2_method5", true},
		3: {"hmyv2_method", true},
		4: {"eth_chainID", true},
		5: {"eth_getValidator", false},
		6: {"hmy_getStakingInfo", false},
		7: {"abc", true},
		8: {"hmy_getStakingNetworkInfo", false},
	}

	for i, test := range tests {
		mustExpose := rmf.Expose(test.name)

		if mustExpose != test.exposure {
			t.Errorf("Test %d got unexpected value, want %t, got %t", i, test.exposure, mustExpose)
		}
	}
}

func TestRpcMethodDenyAllFilter(t *testing.T) {
	method_filters_toml := `
		Allow = [ 
			"mtd1",
			"*staking*",
			"eth_get*",
			"regex:^hmy_[a-z]+"
		]

		Deny = [ 
			"*"
		]
	`
	b := []byte(method_filters_toml)

	var rmf RpcMethodFilter
	rmf.LoadRpcMethodFilters(b)

	tests := []struct {
		name     string
		exposure bool
	}{
		0: {"mtd1", false},
		1: {"hmy_method1", false},
		2: {"hmyv2_method5", false},
		3: {"hmyv2_method", false},
		4: {"eth_chainID", false},
		5: {"eth_getValidator", false},
		6: {"hmy_getStakingInfo", false},
		7: {"abc", false},
		8: {"hmy_getStakingNetworkInfo", false},
	}

	for i, test := range tests {
		mustExpose := rmf.Expose(test.name)

		if mustExpose != test.exposure {
			t.Errorf("Test %d got unexpected value, want %t, got %t", i, test.exposure, mustExpose)
		}
	}
}

func TestEmptyRpcMethodFilter(t *testing.T) {

	b := []byte("")
	var rmf RpcMethodFilter
	rmf.LoadRpcMethodFilters(b)

	tests := []struct {
		name     string
		exposure bool
	}{
		0: {"hmy_method1", true},
		1: {"hmy_method2", true},
		2: {"hmyv2_method5", true},
		3: {"hmyv2_method", true},
		4: {"eth_chainID", true},
		5: {"eth_getValidator", true},
		6: {"hmy_getStakingInfo", true},
		7: {"abc", true},
		8: {"hmy_getNetworkInfo", true},
	}

	for i, test := range tests {
		mustExpose := rmf.Expose(test.name)

		if mustExpose != test.exposure {
			t.Errorf("Test %d got unexpected value, want %t, got %t", i, test.exposure, mustExpose)
		}
	}
}

func TestFilter(t *testing.T) {
	tests := []struct {
		input             string
		pattern           string
		expectedAllowance bool
	}{
		0:  {"abc", "abc", true},
		1:  {"abcd", "*", true},         // check * to pass everything
		2:  {"abc", "simple:abc", true}, // check simple matching
		3:  {"abcd", "simple:abc", false},
		4:  {"abcd", "regex:^a([a-z]+)d$", true}, // check regex
		5:  {"abcde", "regex:^a([a-z]+)d$", false},
		6:  {"abc", "wildcard:abc*", true}, // check wild card
		7:  {"abcdef", "abc*", true},       // by default is wild card
		8:  {"dabcd", "?abc*", true},       // check * and ? for wild card
		9:  {"abc", "*abc?", false},        // ? can't be empty
		10: {"abcdef", "*a?c*", true},
		11: {"defabc", "*ab?*", true},
		12: {"defabcghi", "*abc*", true},
		13: {"ab", "*abc*", false},
		14: {"defghabc", "*abc*", true},
	}

	for i, test := range tests {
		isAllowed := Match(test.pattern, test.input)

		if isAllowed != test.expectedAllowance {
			t.Errorf("Test %d got unexpected value, want %t, got %t", i, test.expectedAllowance, isAllowed)
		}
	}
}
