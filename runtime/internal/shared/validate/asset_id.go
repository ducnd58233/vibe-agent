package validate

import "regexp"

// assetIDPattern matches graph metadata.id: lowercase letter first, then letters, digits, hyphens.
var assetIDPattern = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

// AssetID reports whether s is a valid graph asset identifier (metadata.id).
func AssetID(s string) bool {
	return assetIDPattern.MatchString(s)
}
