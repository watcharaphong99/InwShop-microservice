package jwtauth

import "testing"

func TestApiKeyHasApiKeySubject(t *testing.T) {
	secret := "apisecret"
	token := NewApiKey(secret).SignToken()
	if token == "" {
		t.Fatal("expected signed api key token")
	}

	claims, err := ParseToken(secret, token)
	if err != nil {
		t.Fatalf("ParseToken: %v", err)
	}
	if claims.Subject != "api-key" {
		t.Fatalf("subject = %q, want api-key", claims.Subject)
	}
}

func TestParseTokenRejectsMalformed(t *testing.T) {
	_, err := ParseToken("apisecret", "")
	if err == nil {
		t.Fatal("expected error for empty token")
	}
}
