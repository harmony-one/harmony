// Copyright (c) 2014, Suryandaru Triandana <syndtr@gmail.com>
// All rights reserved.
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package ida errors provides common error types used throughout leveldb.
package ida

import (
	"errors"
)

// Common errors.
var (
	ErrRaptorImpNotFound = New("raptor implementation: not found")
	ErrTimeOut           = New("timeout: time's up now")
)

// New returns an error that formats as the given text.
func New(text string) error {
	return errors.New(text)
}
