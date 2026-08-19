//go:build windows

package hostpick

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"strings"
	"unicode/utf16"

	"github.com/ducnd58233/vibe-agent/runtime/internal/safexec"
	"github.com/ducnd58233/vibe-agent/runtime/internal/web/domain"
)

const pickFilePS = `
Add-Type -AssemblyName System.Windows.Forms
$d = New-Object System.Windows.Forms.OpenFileDialog
$d.Multiselect = $false
$d.CheckFileExists = $true
if ($d.ShowDialog() -ne [System.Windows.Forms.DialogResult]::OK) { exit 2 }
[Console]::OutputEncoding = [System.Text.UTF8Encoding]::new($false)
[Console]::Out.Write($d.FileName)
`

const pickFolderPS = `
Add-Type -AssemblyName System.Windows.Forms
$d = New-Object System.Windows.Forms.FolderBrowserDialog
$d.ShowNewFolderButton = $false
if ($d.ShowDialog() -ne [System.Windows.Forms.DialogResult]::OK) { exit 2 }
[Console]::OutputEncoding = [System.Text.UTF8Encoding]::new($false)
[Console]::Out.Write($d.SelectedPath)
`

func pick(ctx context.Context, kind domain.PickKind) (string, error) {
	script := pickFilePS
	if kind == domain.PickFolder {
		script = pickFolderPS
	}
	cmd, err := safexec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-STA", "-WindowStyle", "Hidden", "-EncodedCommand", utf16LEBase64(script))
	if err != nil {
		return "", domain.ErrPickUnavailable
	}
	out, err := cmd.Output()
	if err != nil {
		var exitErr *safexec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 2 {
			return "", domain.ErrPickCancelled
		}
		return "", domain.ErrPickUnavailable
	}
	path := strings.TrimSpace(string(out))
	if path == "" {
		return "", domain.ErrPickCancelled
	}
	return path, nil
}

func utf16LEBase64(script string) string {
	u := utf16.Encode([]rune(script))
	b := make([]byte, len(u)*2)
	for i, v := range u {
		binary.LittleEndian.PutUint16(b[i*2:], v)
	}
	return base64.StdEncoding.EncodeToString(b)
}
