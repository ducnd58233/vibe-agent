package domain

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestResolveHostDirRejectsRelativeAndDotDot(t *testing.T) {
	for _, dir := range []string{"relative", "..", "../outside"} {
		if _, err := ResolveHostDir(dir); err == nil {
			t.Fatalf("expected error for %q", dir)
		}
	}
}

func TestResolveHostDirAllowsAbsoluteTemp(t *testing.T) {
	root := t.TempDir()
	got, err := ResolveHostDir(root)
	if err != nil {
		t.Fatal(err)
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		t.Fatal(err)
	}
	if got != abs {
		t.Fatalf("got %q want %q", got, abs)
	}
}

func TestHostRootsIncludesHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	rows := HostRoots()
	found := false
	for _, row := range rows {
		if row.Path == filepath.ToSlash(home) || filepath.Clean(row.Path) == filepath.Clean(home) {
			found = true
			if !row.IsDir {
				t.Fatal("home must be a directory row")
			}
		}
	}
	if !found {
		t.Fatal("HostRoots must include the user home")
	}
	if runtime.GOOS == "windows" {
		hasDrive := false
		for _, row := range rows {
			if strings.HasSuffix(filepath.Clean(row.Path), `:\`) || len(filepath.VolumeName(row.Path)) == 2 {
				hasDrive = true
				break
			}
		}
		if !hasDrive {
			t.Fatal("windows HostRoots must include a drive letter")
		}
	}
}
