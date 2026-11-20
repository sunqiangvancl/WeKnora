//go:build !desktop

package docparser

// preferredNativeEngine is disabled outside the desktop build so the server
// and Lite CLI keep using the documented docreader/default routing.
func preferredNativeEngine(_ string) string { return "" }
