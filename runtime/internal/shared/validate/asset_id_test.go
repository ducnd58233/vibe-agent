package validate

import "testing"

func TestAssetID(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"goal-delivery", true},
		{"a", true},
		{"1bad", false},
		{"UPPER", false},
		{"", false},
	}
	for _, tc := range cases {
		if got := AssetID(tc.in); got != tc.want {
			t.Fatalf("AssetID(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}
