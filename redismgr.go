package main

import (
	"context"
	"encoding/json"
	"os"
	"strconv"

	"github.com/redis/go-redis/v9"
)

type SondeSession struct {
	Time     int64  `json:"time"`
	Webhook  string `json:"webhook"`
	FromText string `json:"fromText"`
}

// NewRedisClient creates a RedisMgr using environment variables, defaults to localhost:6379 if not set.
func NewRedisClient() *RedisMgr {
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		addr = "localhost:6379"
	}
	password := os.Getenv("REDIS_PASSWORD")
	db := 0
	if dbStr := os.Getenv("REDIS_DB"); dbStr != "" {
		// Optionally parse db from env
		if parsed, err := strconv.Atoi(dbStr); err == nil {
			db = parsed
		}
	}
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password, // empty string means no password
		DB:       db,       // use default DB
	})
	return &RedisMgr{Client: client}
}

// RedisMgr wraps a redis.Client to allow custom methods.
type RedisMgr struct {
	Client *redis.Client
}

// GetSondeSession checks for a SondeSession, and provides one if it exists.
func (mgr *RedisMgr) GetSondeSession(serial string) (*SondeSession, error) {
	ctx := context.Background()
	data, err := mgr.Client.Get(ctx, serial).Result()
	if err == redis.Nil {
		// Key does not exist
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	var session SondeSession
	if err := json.Unmarshal([]byte(data), &session); err != nil {
		return nil, err
	}
	return &session, nil
}

// SaveSondeSession saves a SondeSession to Redis with a TTL of 8 hours.
func (mgr *RedisMgr) SaveSondeSession(serial string, session *SondeSession) error {
	ctx := context.Background()
	data, err := json.Marshal(session)
	if err != nil {
		return err
	}
	return mgr.Client.Set(ctx, serial, data, 8*60*60*1e9).Err() // 8 hours in nanoseconds
}

// Ping checks if the Redis connection is alive.
func (mgr *RedisMgr) Ping() error {
	ctx := context.Background()
	return mgr.Client.Ping(ctx).Err()
}
