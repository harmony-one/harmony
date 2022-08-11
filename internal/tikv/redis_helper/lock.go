package redis_helper

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"

	"github.com/go-redis/redis/v8"
)

type LockResult int

const (
	_ LockResult = iota
	LockResultSuccess
	LockResultRenewalSuccess
	LockResultFail
)

type RedisPreempt struct {
	key                      string
	password                 string
	lockScript, unlockScript *redis.Script
	lastLockStatus           bool
}

// CreatePreempt used to create a redis preempt instance
func CreatePreempt(key string) *RedisPreempt {
	p := &RedisPreempt{
		key: key,
	}
	p.init()

	return p
}

// init redis preempt instance and some script
func (p *RedisPreempt) init() {
	byt := make([]byte, 18)
	_, _ = rand.Read(byt)
	p.password = base64.StdEncoding.EncodeToString(byt)
	p.lockScript = redis.NewScript(`
		local val = redis.call('get', KEYS[1])
		if (val==nil or (type(val) == "boolean" and not val)) then
			redis.call('set', KEYS[1], ARGV[1], 'ex', ARGV[2])
			return 'LockResultSuccess'
		elseif (val == ARGV[1]) then
			redis.call('expire', KEYS[1], ARGV[2])
			return 'LockResultRenewalSuccess'
		else 
			return 'LockResultFail'
		end
	`)
	p.unlockScript = redis.NewScript(`
		if redis.call('get',KEYS[1]) == ARGV[1] then
			return redis.call('del', KEYS[1])
		else
			return 0
		end
	`)
}

// TryLock attempt to lock the master for ttlSecond
func (p *RedisPreempt) TryLock(ttlSecond int) (result LockResult, err error) {
	ret, err := p.lockScript.Run(context.Background(), redisInstance, []string{p.key}, p.password, ttlSecond).Text()
	if err != nil {
		return LockResultFail, err
	}
	switch ret {
	case "LockResultRenewalSuccess":
		p.lastLockStatus = true
		return LockResultRenewalSuccess, nil
	case "LockResultSuccess":
		p.lastLockStatus = true
		return LockResultSuccess, nil
	case "LockResultFail":
		return LockResultFail, nil
	default:
		return LockResultFail, errors.New("fixme: unknown return")
	}
}

// Unlock try to release the master permission
func (p *RedisPreempt) Unlock() (bool, error) {
	if p == nil {
		return false, nil
	}

	return p.unlockScript.Run(context.Background(), redisInstance, []string{p.key}, p.password).Bool()
}

// LastLockStatus get the last preempt status
func (p *RedisPreempt) LastLockStatus() bool {
	if p == nil {
		return false
	}

	return p.lastLockStatus
}
