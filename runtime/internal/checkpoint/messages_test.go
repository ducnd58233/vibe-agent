package checkpoint

import "testing"

func TestValidateBlockerOutcome(t *testing.T) {
	if err := ValidateBlockerOutcome("", ""); err != nil {
		t.Fatalf("empty blocker/class: %v", err)
	}
	if err := ValidateBlockerOutcome("blocked", "test"); err != nil {
		t.Fatalf("valid pair: %v", err)
	}
	if err := ValidateBlockerOutcome("", "test"); err == nil {
		t.Fatal("class without blocker should fail")
	}
	if err := ValidateBlockerOutcome("blocked", ""); err == nil {
		t.Fatal("blocker without class should fail")
	}
	if err := ValidateBlockerOutcome("blocked", "not-a-class"); err == nil {
		t.Fatal("unknown class should fail")
	}
}

func TestDuplicateReplayStringsNonEmpty(t *testing.T) {
	if DuplicateReplayCLI() == "" || DuplicateReplayNote() == "" {
		t.Fatal("duplicate replay strings must be non-empty")
	}
}
