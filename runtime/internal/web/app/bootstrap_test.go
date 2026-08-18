package app

import (
	"testing"
)

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
