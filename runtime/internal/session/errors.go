package session

import (
	"errors"
	"io/fs"
)

// ErrSessionLogNotFound means the session NDJSON log file does not exist yet.
var ErrSessionLogNotFound = errors.New("session log not found")

// IsNotFound reports whether err is a missing session log or filesystem not-exist.
func IsNotFound(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, ErrSessionLogNotFound) || errors.Is(err, fs.ErrNotExist)
}
