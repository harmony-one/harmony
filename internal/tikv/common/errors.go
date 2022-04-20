package common

import (
	"errors"

	"github.com/syndtr/goleveldb/leveldb"
)

var ErrEmptyKey = errors.New("empty key is not supported")
var ErrNotFound = leveldb.ErrNotFound
