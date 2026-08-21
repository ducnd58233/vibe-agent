package validate

import "regexp"

// Date reports whether s is a calendar date in YYYY-MM-DD form.
// It checks shape only, not whether the day exists on the calendar.
var datePattern = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}$`)

func Date(s string) bool {
	return datePattern.MatchString(s)
}
