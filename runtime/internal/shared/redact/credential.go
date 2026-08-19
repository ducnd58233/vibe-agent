package redact

// ContainsCredential reports whether text carries credential-shaped material.
// Detection matches Text: gitleaks default rules first, then regex groups from
// MemoryRejectPatterns. Callers at storage boundaries use this instead of
// duplicating pattern lists.
func ContainsCredential(s string) bool {
	if s == "" {
		return false
	}
	if hasGitleaksFinding(s) {
		return true
	}
	for _, pattern := range MemoryRejectPatterns() {
		if pattern.MatchString(s) {
			return true
		}
	}
	return false
}
