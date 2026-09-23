package rediscon

import "testing"

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
