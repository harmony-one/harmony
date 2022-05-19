// Copyright 2016 The go-ethereum Authors
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
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
	"strings"

	"github.com/pelletier/go-toml"
)

type RpcMethodFilter struct {
	Allow []string
	Deny  []string
}

// ExposeAll - init Allow and Deny array in a way to expose all APIs
func (rmf *RpcMethodFilter) ExposeAll() error {
	rmf.Allow = rmf.Allow[:0]
	rmf.Allow = rmf.Deny[:0]
	rmf.Allow = append(rmf.Allow, "*")
	return nil
}

// LoadRpcMethodFilters - load RPC method filters from toml file
/* ex: filters.toml
Allow = [ ... ]
Deny = [ ... ]
*/
func (rmf *RpcMethodFilter) LoadRpcMethodFiltersFromFile(file string) error {
	// check if file exist
	if _, err := os.Stat(file); err == nil {
		b, errRead := ioutil.ReadFile(file)
		if errRead != nil {
			return fmt.Errorf("rpc filter file read error - %s", errRead.Error())
		}
		return rmf.LoadRpcMethodFilters(b)
	} else if errors.Is(err, os.ErrNotExist) {
		// file path does not exist
		return fmt.Errorf("rpc filter file doesn't exist")
	} else {
		// some other errors happened
		return fmt.Errorf("rpc filter file stat error - %s", err.Error())
	}
}

// LoadRpcMethodFilters - load RPC method filters from binary array (given from toml file)
func (rmf *RpcMethodFilter) LoadRpcMethodFilters(b []byte) error {
	fTree, err := toml.LoadBytes(b)
	if err != nil {
		return fmt.Errorf("rpc filter file parse error - %s", err.Error())
	}
	if err := fTree.Unmarshal(rmf); err != nil {
		return fmt.Errorf("rpc filter parse error - %s", err.Error())
	}
	if len(rmf.Allow) == 0 {
		rmf.Allow = append(rmf.Allow, "*")
	}

	return nil
}

// Expose - checks whether specific method have to expose or not
func (rmf *RpcMethodFilter) Expose(name string) bool {
	allow := checkFilters(rmf.Allow, name)
	deny := checkFilters(rmf.Deny, name)
	return allow && !deny
}

// checkFilters - checks whether any of filters match with value
func checkFilters(filters []string, value string) bool {
	if len(filters) == 0 {
		return false
	}
	for _, filter := range filters {
		if Match(filter, value) {
			return true
		}
	}
	return false
}

// MatchRegex -  finds whether the text matches/satisfies the regex pattern string.
// pattern can include match type (ex: regex:^[a-z]bc )
func Match(pattern string, value string) bool {
	parts := strings.SplitN(pattern, ":", 2)

	// check if pattern defines match type
	if len(parts) > 1 {
		matchType := strings.Trim(strings.ToLower(parts[0]), " ")
		matchPattern := strings.Trim(parts[1], " ")
		switch matchType {
		case "exact":
			return matchPattern == value
		case "simple":
			return MatchSimple(matchPattern, value)
		case "wildcard":
			return MatchWildCard(matchPattern, value)
		case "regex":
			isAllowed, _ := regexp.MatchString(matchPattern, value)
			return isAllowed
		default:
			isAllowed, _ := regexp.MatchString(matchPattern, value)
			return isAllowed
		}
	}
	// auto detect simple checking  or wildcard
	if regexp.MustCompile(`^[a-zA-Z0-9_]+$`).MatchString(pattern) {
		return strings.EqualFold(pattern, value)
	} else if regexp.MustCompile(`^[a-zA-Z0-9_*?]+$`).MatchString(pattern) {
		return MatchWildCard(pattern, value)
	}
	// by default we use regex matching
	allowed, _ := regexp.MatchString(pattern, value)
	return allowed
}

// MatchSimple - finds whether the text matches/satisfies the pattern string.
// supports only '*' wildcard in the pattern.
func MatchSimple(pattern, name string) bool {
	if pattern == "" {
		return name == pattern
	}

	if pattern == "*" {
		return true
	}
	// Does only wildcard '*' match.
	return deepMatchRune([]rune(name), []rune(pattern), true)
}

// Match -  finds whether the text matches/satisfies the pattern string.
// supports  '*' and '?' wildcards in the pattern string.
func MatchWildCard(pattern, name string) (matched bool) {
	if pattern == "" {
		return name == pattern
	}

	if pattern == "*" {
		return true
	}
	// Does extended wildcard '*' and '?' match.
	return deepMatchRune([]rune(name), []rune(pattern), false)
}

func deepMatchRune(str, pattern []rune, simple bool) bool {
	for len(pattern) > 0 {
		switch pattern[0] {
		default:
			if len(str) == 0 || str[0] != pattern[0] {
				return false
			}
		case '?':
			if len(str) == 0 && !simple {
				return false
			}
		case '*':
			return deepMatchRune(str, pattern[1:], simple) ||
				(len(str) > 0 && deepMatchRune(str[1:], pattern, simple))
		}

		str = str[1:]
		pattern = pattern[1:]
	}

	return len(str) == 0 && len(pattern) == 0
}

// MatchRegex -  finds whether the text matches/satisfies the regex pattern string.
func MatchRegex(pattern string, value string) bool {
	result, _ := regexp.MatchString(pattern, value)
	return result
}
