package sessionread

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/ducnd58233/vibe-agent/runtime/internal/session"
)

// FS reads session logs from disk via session path helpers.
type FS struct{}

// NewFS returns a filesystem-backed session reader.
func NewFS() FS {
	return FS{}
}

type hostLine struct {
	Payload struct {
		Client string `json:"client"`
	} `json:"payload"`
}

func logPath(workspaceRoot, slug string) string {
	if slug == "ambient" {
		return session.AmbientLogPath(workspaceRoot)
	}
	return session.LogPath(workspaceRoot, slug)
}

// Replay loads session gestures for slug ("ambient" uses the ambient journal).
func (FS) Replay(workspaceRoot, slug string) ([]session.Event, error) {
	events, err := session.Replay(logPath(workspaceRoot, slug))
	if err != nil && session.IsNotFound(err) {
		return nil, nil
	}
	return events, err
}

// AmbientStat reports whether the ambient journal exists and is non-empty.
func (FS) AmbientStat(workspaceRoot string) AmbientStat {
	path := session.AmbientLogPath(workspaceRoot)
	info, err := os.Stat(path)
	if err != nil || info.IsDir() || info.Size() == 0 {
		return AmbientStat{}
	}
	return AmbientStat{
		Present: true,
		Size:    info.Size(),
		ModTime: info.ModTime(),
	}
}

// PeekHost returns payload.client from the first matching lines of the log.
func (FS) PeekHost(logPath string) string {
	file, err := os.Open(filepath.Clean(logPath))
	if err != nil {
		return ""
	}
	defer func() { _ = file.Close() }()
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for i := 0; i < 8 && scanner.Scan(); i++ {
		var line hostLine
		if err := json.Unmarshal(scanner.Bytes(), &line); err != nil {
			continue
		}
		if line.Payload.Client != "" {
			return line.Payload.Client
		}
	}
	_ = scanner.Err()
	return ""
}
