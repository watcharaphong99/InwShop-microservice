package playerHandler

import "testing"

func TestFormatPlayerId(t *testing.T) {
	if got := formatPlayerId("6a927a8535887829348adce9"); got != "player:6a927a8535887829348adce9" {
		t.Fatalf("got %q", got)
	}
	if got := formatPlayerId("player:6a927a8535887829348adce9"); got != "player:6a927a8535887829348adce9" {
		t.Fatalf("got %q", got)
	}
}
