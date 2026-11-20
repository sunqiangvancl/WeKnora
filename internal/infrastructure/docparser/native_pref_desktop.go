//go:build desktop

package docparser

import "runtime"

// preferredNativeEngine routes the desktop app's default parser to the local
// engine. It is intentionally gated by the "desktop" build tag: the Lite CLI
// and full server keep their existing docreader/default behavior.
func preferredNativeEngine(fileType string) string {
	switch normalizeNativeType(fileType) {
	case "pdf":
		if nativePDFHelperPath() != "" {
			return NativeEngineName
		}
	case "doc":
		if runtime.GOOS == "darwin" {
			return NativeEngineName
		}
	case "docx", "xlsx", "pptx", "html", "htm", "url":
		return NativeEngineName
	}
	return ""
}
