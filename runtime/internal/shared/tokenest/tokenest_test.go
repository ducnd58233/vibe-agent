package tokenest

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestEstimateRoundsUp(t *testing.T) {
	for text, want := range map[string]int{"": 0, "a": 1, "abcd": 1, "abcde": 2} {
		if got := Estimate(text); got != want {
			t.Errorf("Estimate(%q) = %d, want %d", text, got, want)
		}
	}
}

func TestTruncateWordsCutsAtABoundary(t *testing.T) {
	text := "the integration suite requires redis on localhost before it will pass"
	got := TruncateWords(text, 40)
	if len(got) > 43 || !strings.HasSuffix(got, "...") {
		t.Fatalf("got %q", got)
	}
	if strings.Contains(got, "loc...") {
		t.Errorf("cut mid-word: %q", got)
	}
	if TruncateWords("short", 40) != "short" {
		t.Error("text within the limit was altered")
	}
}

func TestTruncateAndHeadTailNeverSplitARune(t *testing.T) {
	text := strings.Repeat("héllo wörld ", 80)
	for _, got := range []string{TruncateWords(text, 101), HeadTail(text, 200)} {
		if !utf8.ValidString(got) {
			t.Fatalf("invalid utf-8: %q", got)
		}
	}
}

func TestHeadTailKeepsBothEndsAndSaysWhatItDropped(t *testing.T) {
	text := "QUESTION " + strings.Repeat("filler ", 300) + " CONCLUSION"
	got := HeadTail(text, 300)
	if len(got) > 300 {
		t.Errorf("len = %d, over the cap", len(got))
	}
	if !strings.HasPrefix(got, "QUESTION") || !strings.HasSuffix(got, "CONCLUSION") {
		t.Errorf("an end was lost: %q", got)
	}
	if !strings.Contains(got, "chars omitted") {
		t.Errorf("the drop was silent: %q", got)
	}
	if HeadTail("short", 300) != "short" {
		t.Error("text within the cap was altered")
	}
}
