package rbac

import "testing"

func TestIntToBinaryTwoRoles(t *testing.T) {
	got := IntToBinary(1, 2)
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0] != 1 || got[1] != 0 {
		t.Fatalf("got %v, want [1 0]", got)
	}
}
