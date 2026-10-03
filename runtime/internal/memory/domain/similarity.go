package domain

import (
	"strings"
	"unicode"
)

// nearDuplicateThreshold is the Jaccard overlap of two claims' word sets at or
// above which they are treated as one claim said twice. High on purpose: a
// merge keeps the existing wording and only adds evidence, so a false merge
// quietly loses the candidate's claim while a missed one only costs a row.
const nearDuplicateThreshold = 0.9

// minNearDuplicateWords keeps short claims out of fuzzy matching. A single
// changed word in a ten-word claim is a 0.82 overlap, so below the threshold; the
// guard exists because a changed value ("frankfurt" for "virginia") carries no
// number or negation for the guards below to catch, and only length keeps one
// word from looking like noise.
const minNearDuplicateWords = 8

var negations = map[string]bool{
	"not": true, "no": true, "never": true, "without": true, "cannot": true,
	"cant": true, "dont": true, "doesnt": true, "isnt": true, "wont": true,
	"none": true, "disabled": true,
}

// NearDuplicate reports whether two claims say the same thing in slightly
// different words. Curating by Jaccard similarity is how agent-memory stores
// keep paraphrases from piling up and each taking a retrieval slot.
//
// Overlap alone would merge claims a single token reverses, so two guards run
// first: every number must match, and so must the set of negations. "retries are
// capped at 3" and "retries are capped at 5" differ by one word of seven; they
// are different facts, and so are "is enabled" and "is not enabled".
func NearDuplicate(a, b string) bool {
	left, right := wordSet(a), wordSet(b)
	if len(left) < minNearDuplicateWords || len(right) < minNearDuplicateWords {
		return false
	}
	if !sameMatching(left, right, hasDigit) || !sameMatching(left, right, func(w string) bool { return negations[w] }) {
		return false
	}
	shared := 0
	for word := range left {
		if right[word] {
			shared++
		}
	}
	union := len(left) + len(right) - shared
	return float64(shared)/float64(union) >= nearDuplicateThreshold
}

func wordSet(text string) map[string]bool {
	words := map[string]bool{}
	for _, field := range strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	}) {
		words[field] = true
	}
	return words
}

func hasDigit(word string) bool {
	for _, r := range word {
		if unicode.IsDigit(r) {
			return true
		}
	}
	return false
}

// sameMatching reports whether both sets hold exactly the same words among those
// the predicate selects.
func sameMatching(a, b map[string]bool, selects func(string) bool) bool {
	for word := range a {
		if selects(word) && !b[word] {
			return false
		}
	}
	for word := range b {
		if selects(word) && !a[word] {
			return false
		}
	}
	return true
}
