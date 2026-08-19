package validate

import "regexp"

var slugPattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// Slug reports whether s is lowercase kebab-case suitable for run slugs.
func Slug(s string) bool {
	return slugPattern.MatchString(s)
}
