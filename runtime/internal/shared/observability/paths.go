package observability

import (
	"os"
	"path/filepath"
	"strings"
)

const defaultLogDirName = "logs"

// ResolveLogDir returns where service log files are written.
//
// When the binary lives in bin/ (for example ~/.local/bin), logs go in a sibling
// logs/ directory (for example ~/.local/logs). VIBE_LOG_DIR overrides that.
func ResolveLogDir() (string, error) {
	if dir := strings.TrimSpace(os.Getenv("VIBE_LOG_DIR")); dir != "" {
		return filepath.Clean(dir), nil
	}
	exec, _ := os.Executable()
	if exec == "" {
		return defaultLogDirName, nil
	}
	if resolved, symErr := filepath.EvalSymlinks(exec); symErr == nil {
		exec = resolved
	}
	return logDirFromExecutable(exec), nil
}

func logDirFromExecutable(execPath string) string {
	binDir := filepath.Dir(execPath)
	if filepath.Base(binDir) == "bin" {
		return filepath.Join(filepath.Dir(binDir), defaultLogDirName)
	}
	return filepath.Join(binDir, defaultLogDirName)
}
