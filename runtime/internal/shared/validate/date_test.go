package validate

import "testing"

func TestDate(t *testing.T) {
	for _, testCase := range []struct {
		in   string
		want bool
	}{
		{"2026-08-21", true},
		{"2026-8-21", false},
		{"26-08-21", false},
		{"2026/08/21", false},
		{"", false},
		{"tmp", false},
	} {
		if got := Date(testCase.in); got != testCase.want {
			t.Errorf("Date(%q) = %v, want %v", testCase.in, got, testCase.want)
		}
	}
}
