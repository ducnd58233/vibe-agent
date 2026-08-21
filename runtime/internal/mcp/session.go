package mcp

import "sync"

// Session holds the MCP-host-scoped active run and a pending list-changed flag.
//
// tools/list narrows from the active slug's current node. A real (non-duplicate)
// checkpoint or verify sets PendingListChanged so Serve can emit
// notifications/tools/list_changed on the same turn.
type Session struct {
	mu                 sync.Mutex
	ActiveSlug         string
	PendingListChanged bool
}

// Touch records the slug the host just used.
func (s *Session) Touch(slug string) {
	if s == nil || slug == "" {
		return
	}
	s.mu.Lock()
	s.ActiveSlug = slug
	s.mu.Unlock()
}

// NoteListChanged arms a tools/list_changed notification for the next Serve write.
func (s *Session) NoteListChanged() {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.PendingListChanged = true
	s.mu.Unlock()
}

// Slug returns the active slug, or empty when none has been touched.
func (s *Session) Slug() string {
	if s == nil {
		return ""
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ActiveSlug
}

// ConsumeListChanged reports and clears the pending notification flag.
func (s *Session) ConsumeListChanged() bool {
	if s == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	pending := s.PendingListChanged
	s.PendingListChanged = false
	return pending
}
