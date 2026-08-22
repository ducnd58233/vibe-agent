package app

import (
	"testing"
)

func TestDefaultPortIs1411(t *testing.T) {
	if DefaultPort != 1411 {
		t.Fatalf("DefaultPort = %d, want 1411", DefaultPort)
	}
}

func TestAddrFallsBackToDefaultPort(t *testing.T) {
	if got, want := Addr(0), "127.0.0.1:1411"; got != want {
		t.Fatalf("Addr(0) = %q, want %q", got, want)
	}
}

func TestValidateListenHostRejectsWildcard(t *testing.T) {
	for _, host := range []string{"0.0.0.0", "::", "[::]"} {
		if err := ValidateListenHost(host); err == nil {
			t.Fatalf("expected error for %q", host)
		}
	}
}

func TestValidateListenHostAllowsLoopback(t *testing.T) {
	for _, host := range []string{"127.0.0.1", "localhost"} {
		if err := ValidateListenHost(host); err != nil {
			t.Fatalf("%q: %v", host, err)
		}
	}
}
