//go:build darwin

package docparser

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Tencent/WeKnora/internal/types"
)

// nativePDFHelperPath locates the bundled pdf2md executable. Packaged apps
// keep it in Contents/Resources; dev runs can set WEKNORA_PDF2MD.
func nativePDFHelperPath() string {
	if p := strings.TrimSpace(os.Getenv("WEKNORA_PDF2MD")); p != "" {
		if nativeExecutableFile(p) {
			return p
		}
	}
	exe, err := os.Executable()
	if err == nil {
		const marker = ".app/Contents/MacOS"
		if i := strings.Index(exe, marker); i >= 0 {
			p := filepath.Join(exe[:i+len(".app/Contents")], "Resources", "pdf2md")
			if nativeExecutableFile(p) {
				return p
			}
		}
	}
	if p, err := exec.LookPath("pdf2md"); err == nil {
		return p
	}
	return ""
}

func nativeExecutableFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir() && info.Mode()&0111 != 0
}

func nativePDFToMarkdown(ctx context.Context, req *types.ReadRequest) (string, error) {
	helper := nativePDFHelperPath()
	if helper == "" {
		return "", fmt.Errorf("native PDF helper not found; rebuild the desktop app or set WEKNORA_PDF2MD")
	}
	if len(req.FileContent) == 0 {
		return "", fmt.Errorf("native PDF parser: empty file")
	}

	f, err := os.CreateTemp("", "weknora-native-*.pdf")
	if err != nil {
		return "", fmt.Errorf("native PDF parser: create temp file: %w", err)
	}
	name := f.Name()
	defer os.Remove(name)
	if _, err := f.Write(req.FileContent); err != nil {
		f.Close()
		return "", fmt.Errorf("native PDF parser: write temp file: %w", err)
	}
	if err := f.Close(); err != nil {
		return "", fmt.Errorf("native PDF parser: close temp file: %w", err)
	}

	cmd := exec.CommandContext(ctx, helper, name)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("native PDF parser: %s", msg)
	}
	return stdout.String(), nil
}
