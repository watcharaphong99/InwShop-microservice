package playerRepository

import (
	"testing"

	"github.com/watcharaphong99/InwzaShop/modules/player"
)

func TestSameEventTransaction(t *testing.T) {
	existing := &player.PlayerTransaction{PlayerId: "player:abc", Amount: 100}

	tests := []struct {
		name     string
		incoming *player.PlayerTransaction
		want     bool
	}{
		{name: "same player and amount", incoming: &player.PlayerTransaction{PlayerId: "player:abc", Amount: 100}, want: true},
		{name: "other player", incoming: &player.PlayerTransaction{PlayerId: "player:xyz", Amount: 100}, want: false},
		{name: "other amount", incoming: &player.PlayerTransaction{PlayerId: "player:abc", Amount: -100}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sameEventTransaction(existing, tt.incoming); got != tt.want {
				t.Fatalf("sameEventTransaction = %v, want %v", got, tt.want)
			}
		})
	}
}
