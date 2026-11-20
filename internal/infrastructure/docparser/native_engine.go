package docparser

import (
	"context"

	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// NativeEngineName is the in-process parser used by the Lite desktop app.
const NativeEngineName = "native"

func init() {
	RegisterEngine(&nativeEngine{})
}

type nativeEngine struct{}

func (e *nativeEngine) Name() string { return NativeEngineName }

func (e *nativeEngine) Description() string {
	return "Built-in local parser (PDF/Office/HTML, no external service)"
}

func (e *nativeEngine) FileTypes(_ bool) []string {
	return []string{"pdf", "doc", "docx", "xlsx", "pptx", "html", "htm"}
}

func (e *nativeEngine) CheckAvailable(_ bool, _ map[string]string) (bool, string) {
	return true, ""
}

func (e *nativeEngine) NewReader(_ context.Context, _ ReaderDeps) (interfaces.DocReader, error) {
	return &NativeReader{}, nil
}
