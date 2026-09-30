package rediscon

import (
	"context"
	"testing"
	"time"
)

func TestItemKey(t *testing.T) {
	if got := ItemKey("abc"); got != "item:one:abc" {
		t.Fatalf("unexpected key %q", got)
	}
}

func TestDisabledClientIsNoop(t *testing.T) {
	ctx := context.Background()
	c := &Client{}

	c.SetJSON(ctx, "k", map[string]int{"a": 1}, time.Minute)
	var dest map[string]int
	if c.GetJSON(ctx, "k", &dest) {
		t.Fatal("disabled client must always miss")
	}
	c.Del(ctx, "k")

	c.SetManyJSON(ctx, map[string]any{"a": 1}, time.Minute)
	got := c.MGetBytes(ctx, "a", "b")
	if len(got) != 2 || got[0] != nil || got[1] != nil {
		t.Fatalf("disabled client must return all misses, got %v", got)
	}
}

func TestAccessTokenKeyIsStableHash(t *testing.T) {
	a := AccessTokenKey("token-a")
	b := AccessTokenKey("token-a")
	c := AccessTokenKey("token-b")
	if a != b {
		t.Fatal("same token must hash to the same key")
	}
	if a == c {
		t.Fatal("different tokens must not share a key")
	}
	if len(a) < len(accessTokenPrefix)+10 {
		t.Fatalf("unexpected key %q", a)
	}
}
