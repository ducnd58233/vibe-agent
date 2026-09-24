package docmeta

import (
	"strings"
	"unicode"
)

// LooksTransliterated is a best-effort, non-blocking net for a slug built by
// mechanically transliterating non-English text rather than by choosing an
// English gloss of the objective (AGENTS.md "Generated docs location").
//
// It cannot detect the general case: auto.Slugify keeps only [a-z0-9], so a
// diacritic on a non-English letter is a word break, and what survives from
// each broken word is often still a real (if short) English-looking token -
// "cho-phep-slug-dung" passes this heuristic clean. What it does catch is the
// narrower, observed failure shape in this repo's own run history: several
// short fragments in a row with no vowel at all ("l-m-th-n", "m-r-ng-repo"),
// which a diacritic stripped down to a bare consonant cluster. Two or more
// such fragments is the signal; one is treated as an ordinary short English
// word ("by", "my", "in").
func LooksTransliterated(slug string) bool {
	segments := strings.Split(slug, "-")
	bareConsonantCount := 0
	for _, seg := range segments {
		if len(seg) > 0 && len(seg) <= 4 && !hasVowel(seg) && !hasDigit(seg) {
			bareConsonantCount++
		}
	}
	return bareConsonantCount >= 2
}

func hasVowel(s string) bool {
	return strings.ContainsAny(s, "aeiouy")
}

// hasDigit reports a segment like "t4" or "s3": a technical abbreviation or
// version marker, not a diacritic-stripped word fragment.
func hasDigit(s string) bool {
	for _, r := range s {
		if unicode.IsDigit(r) {
			return true
		}
	}
	return false
}
