//go:build !darwin

package docparser

import (
	"context"
	"fmt"

	"github.com/Tencent/WeKnora/internal/types"
)

func nativePDFHelperPath() string { return "" }

func nativePDFToMarkdown(_ context.Context, _ *types.ReadRequest) (string, error) {
	return "", fmt.Errorf("native PDF parsing is only available on macOS")
}
