package services

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/coinbase/rosetta-sdk-go/types"
	lru "github.com/hashicorp/golang-lru"
)

const (
	maxCacheNum  = 1000
	maxCacheTime = 5 * time.Minute
)

var rosettaCache, _ = lru.New(maxCacheNum)

type cacheFunc func(resp interface{}, err *types.Error)

type cacheItem struct {
	resp      interface{}
	cacheTime time.Time
}

func rosettaCacheHelper(typ string, req interface{}) (item *cacheItem, cf cacheFunc, err error) {
	reqText, err := json.Marshal(req)
	if err != nil {
		return nil, nil, err
	}

	md5Hash := md5.Sum(reqText)
	cacheKey := typ + hex.EncodeToString(md5Hash[:])
	if res, ok := rosettaCache.Get(cacheKey); ok {
		resItem := res.(*cacheItem)
		if time.Now().Sub(resItem.cacheTime) < maxCacheTime {
			return resItem, nil, nil
		} else {
			rosettaCache.Remove(cacheKey)
		}
	}

	cf = func(resp interface{}, err *types.Error) {
		if err != nil {
			return
		}

		rosettaCache.Add(cacheKey, &cacheItem{
			resp:      resp,
			cacheTime: time.Now(),
		})
	}

	return nil, cf, nil
}
