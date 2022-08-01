package redis_helper

import (
	"context"

	"github.com/go-redis/redis/v8"
)

var redisInstance *redis.ClusterClient

// Init used to init redis instance in tikv mode
func Init(serverAddr []string) error {
	option := &redis.ClusterOptions{
		Addrs:    serverAddr,
		PoolSize: 2,
	}

	rdb := redis.NewClusterClient(option)
	err := rdb.Ping(context.Background()).Err()
	if err != nil {
		return err
	}

	redisInstance = rdb
	return nil
}

// Close disconnect redis instance in tikv mode
func Close() error {
	if redisInstance != nil {
		return redisInstance.Close()
	}
	return nil
}
