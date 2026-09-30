package rediscon

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	RolesCountKey     = "auth:roles:count"
	RolesCountTTL     = 1 * time.Hour
	accessTokenPrefix = "auth:access:"
	ItemTTL           = 10 * time.Minute
	itemPrefix        = "item:one:"
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

func ItemKey(itemId string) string {
	return itemPrefix + itemId
}

func (c *Client) enabled() bool {
	return c != nil && c.rdb != nil
}

func (c *Client) GetJSON(ctx context.Context, key string, dest any) bool {
	if !c.enabled() {
		return false
	}
	val, err := c.rdb.Get(ctx, key).Bytes()
	if err != nil {
		if err != redis.Nil {
			log.Printf("redis: get %s failed: %s", key, err.Error())
		}
		return false
	}
	if err := json.Unmarshal(val, dest); err != nil {
		log.Printf("redis: decode %s failed: %s", key, err.Error())
		return false
	}
	return true
}

func (c *Client) SetJSON(ctx context.Context, key string, value any, ttl time.Duration) {
	if !c.enabled() || ttl <= 0 {
		return
	}
	val, err := json.Marshal(value)
	if err != nil {
		log.Printf("redis: encode %s failed: %s", key, err.Error())
		return
	}
	if err := c.rdb.Set(ctx, key, val, ttl).Err(); err != nil {
		log.Printf("redis: set %s failed: %s", key, err.Error())
	}
}

// MGetBytes returns one entry per key, nil for a miss.
func (c *Client) MGetBytes(ctx context.Context, keys ...string) [][]byte {
	out := make([][]byte, len(keys))
	if !c.enabled() || len(keys) == 0 {
		return out
	}
	vals, err := c.rdb.MGet(ctx, keys...).Result()
	if err != nil {
		log.Printf("redis: mget failed: %s", err.Error())
		return out
	}
	for i, v := range vals {
		if s, ok := v.(string); ok {
			out[i] = []byte(s)
		}
	}
	return out
}

func (c *Client) SetManyJSON(ctx context.Context, values map[string]any, ttl time.Duration) {
	if !c.enabled() || ttl <= 0 || len(values) == 0 {
		return
	}
	pipe := c.rdb.Pipeline()
	for key, value := range values {
		val, err := json.Marshal(value)
		if err != nil {
			log.Printf("redis: encode %s failed: %s", key, err.Error())
			continue
		}
		pipe.Set(ctx, key, val, ttl)
	}
	if _, err := pipe.Exec(ctx); err != nil {
		log.Printf("redis: set many failed: %s", err.Error())
	}
}

func (c *Client) Del(ctx context.Context, keys ...string) {
	if !c.enabled() || len(keys) == 0 {
		return
	}
	if err := c.rdb.Del(ctx, keys...).Err(); err != nil {
		log.Printf("redis: del %v failed: %s", keys, err.Error())
	}
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
