package connections

import "testing"

func TestPerConnectionLimiterAllowsBurstThenBlocks(t *testing.T) {
	l := newPerConnectionLimiter(1, 3) // 1/s sustained, burst of 3

	allowed := 0
	for i := 0; i < 5; i++ {
		if l.Allow("conn-a") {
			allowed++
		}
	}
	if allowed != 3 {
		t.Fatalf("expected exactly 3 allowed (the burst), got %d", allowed)
	}
}

func TestPerConnectionLimiterIsPerConnection(t *testing.T) {
	l := newPerConnectionLimiter(1, 1)

	if !l.Allow("conn-a") {
		t.Fatal("expected first call for conn-a to be allowed")
	}
	if l.Allow("conn-a") {
		t.Fatal("expected second immediate call for conn-a to be blocked")
	}
	if !l.Allow("conn-b") {
		t.Fatal("expected conn-b to have its own independent budget")
	}
}
