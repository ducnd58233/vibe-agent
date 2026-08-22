package validate

import "testing"

func TestSlug(t *testing.T) {
	for _, tc := range []struct {
		slug string
		ok   bool
	}{
		{"goal-command-sdlc", true},
		{"a", true},
		{"", false},
		{"My-Feature-2", true},
		{"myFeature2", true},
		{"has_underscore", false},
		{"-leading", false},
		{"trailing-", false},
		{"a--b", false},
	} {
		if got := Slug(tc.slug); got != tc.ok {
			t.Errorf("Slug(%q) = %v, want %v", tc.slug, got, tc.ok)
		}
	}
}
