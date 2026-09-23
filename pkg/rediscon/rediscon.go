package rediscon

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"log"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	RolesCountKey     = "auth:roles:count"
	RolesCountTTL     = 1 * time.Hour
	accessTokenPrefix = "auth:access:"
)

type Client struct {
	rdb *redis.Client
}

func NewClient(addr string) *Client {
	if addr == "" {
		log.Println("redis: REDIS_URL is empty, cache disabled")
		return &Client{}
	}

	rdb := redis.NewClient(&redis.Options{
		Addr: addr,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Printf("redis: ping failed, cache disabled: %s", err.Error())
		_ = rdb.Close()
		return &Client{}
	}

	log.Printf("redis: connected %s", addr)
	return &Client{rdb: rdb}
}

func (c *Client) Close() error {
	if c == nil || c.rdb == nil {
		return nil
	}
	return c.rdb.Close()
}

func AccessTokenKey(accessToken string) string {
	sum := sha256.Sum256([]byte(accessToken))
	return accessTokenPrefix + hex.EncodeToString(sum[:])
}

func (c *Client) enabled() bool {
	return c != nil && c.rdb != nil
}

func (c *Client) HasAccessToken(ctx context.Context, accessToken string) bool {
	if !c.enabled() || accessToken == "" {
		return false
	}
	n, err := c.rdb.Exists(ctx, AccessTokenKey(accessToken)).Result()
	if err != nil {
		log.Printf("redis: has access token failed: %s", err.Error())
		return false
	}
	return n == 1
}

func (c *Client) SetAccessToken(ctx context.Context, accessToken string, ttl time.Duration) {
	if !c.enabled() || accessToken == "" || ttl <= 0 {
		return
	}
	if err := c.rdb.Set(ctx, AccessTokenKey(accessToken), "1", ttl).Err(); err != nil {
		log.Printf("redis: set access token failed: %s", err.Error())
	}
}

func (c *Client) DelAccessToken(ctx context.Context, accessToken string) {
	if !c.enabled() || accessToken == "" {
		return
	}
	if err := c.rdb.Del(ctx, AccessTokenKey(accessToken)).Err(); err != nil {
		log.Printf("redis: del access token failed: %s", err.Error())
	}
}

func (c *Client) GetRolesCount(ctx context.Context) (int64, bool) {
	if !c.enabled() {
		return 0, false
	}
	val, err := c.rdb.Get(ctx, RolesCountKey).Result()
	if err != nil {
		if err != redis.Nil {
			log.Printf("redis: get roles count failed: %s", err.Error())
		}
		return 0, false
	}
	count, err := strconv.ParseInt(val, 10, 64)
	if err != nil {
		return 0, false
	}
	return count, true
}

func (c *Client) SetRolesCount(ctx context.Context, count int64) {
	if !c.enabled() {
		return
	}
	if err := c.rdb.Set(ctx, RolesCountKey, strconv.FormatInt(count, 10), RolesCountTTL).Err(); err != nil {
		log.Printf("redis: set roles count failed: %s", err.Error())
	}
}
