package noprogress_test

import (
	"testing"

	"github.com/ducnd58233/vibe-agent/runtime/internal/noprogress"
)

func TestCheck(t *testing.T) {
	cases := []struct {
		name              string
		previous, current string
		wantProgressed    bool
		wantProgressedWhy string
	}{
		{
			name:     "first visit is always allowed",
			previous: "", current: "anything at all\nsecond line\n",
			wantProgressed: true, wantProgressedWhy: "an empty previous (first visit) must always be allowed",
		},
		{
			name:              "identical resubmission",
			previous:          "line one\nline two\nline three\n",
			current:           "line one\nline two\nline three\n",
			wantProgressed:    false,
			wantProgressedWhy: "byte-identical content must not count as progress",
		},
		{
			name:              "whitespace-only rewrite",
			previous:          "line one\nline two\nline three\n",
			current:           "line one\n\nline two\nline three\n\n",
			wantProgressed:    false,
			wantProgressedWhy: "only blank-line differences must not count as progress",
		},
		{
			name:              "genuine revision",
			previous:          "The approach is X.\nTried A.\nTried B.\nConclusion: unclear.\n",
			current:           "The approach is Y.\nTried C.\nTried D.\nConclusion: works.\n",
			wantProgressed:    true,
			wantProgressedWhy: "a substantially different artifact must count as progress",
		},
		{
			name:              "trivial one-word edit",
			previous:          "The approach is X.\nTried A.\nTried B.\nConclusion: unclear.\n",
			current:           "The approach is X.\nTried A.\nTried B.\nConclusion: unclear!\n",
			wantProgressed:    false,
			wantProgressedWhy: "a single-line edit below the threshold must not count as progress",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			progressed, changed := noprogress.Check(c.previous, c.current)
			if progressed != c.wantProgressed {
				t.Errorf("%s: changed=%d", c.wantProgressedWhy, changed)
			}
		})
	}
}
