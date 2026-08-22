package validate

import "regexp"

var slugPattern = regexp.MustCompile(`^[A-Za-z0-9]+(-[A-Za-z0-9]+)*$`)

// Slug reports whether s is kebab-case (letters of either case, digits,
// single hyphens between segments) suitable for run slugs.
func Slug(s string) bool {
	return slugPattern.MatchString(s)
}
