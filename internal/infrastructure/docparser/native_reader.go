package docparser

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/Tencent/WeKnora/internal/types"
)

// NativeReader handles PDF, Office and HTML files in-process on macOS/Lite.
// Office and HTML conversion is pure Go; PDF delegates to the bundled pdf2md
// helper (PDFKit + Vision OCR), so a desktop install needs no Python service.
type NativeReader struct{}

func (r *NativeReader) Read(ctx context.Context, req *types.ReadRequest) (*types.ReadResult, error) {
	if req == nil {
		return nil, fmt.Errorf("native parser: empty request")
	}
	if strings.TrimSpace(req.URL) != "" {
		md, err := nativeURLToMarkdown(ctx, req.URL)
		if err != nil {
			return nil, err
		}
		return &types.ReadResult{MarkdownContent: md}, nil
	}

	ft := normalizeNativeType(req.FileType)
	if ft == "" {
		ft = strings.TrimPrefix(strings.ToLower(filepath.Ext(req.FileName)), ".")
	}

	var (
		md  string
		err error
	)
	switch ft {
	case "pdf":
		md, err = nativePDFToMarkdown(ctx, req)
	case "doc":
		md, err = nativeDOCToMarkdown(ctx, req.FileContent)
	case "docx":
		md, err = nativeDOCXToMarkdown(req.FileContent)
	case "xlsx":
		md, err = nativeXLSXToMarkdown(req.FileContent)
	case "pptx":
		md, err = nativePPTXToMarkdown(req.FileContent)
	case "html", "htm":
		md, err = nativeHTMLToMarkdown(req.FileContent)
	default:
		return nil, fmt.Errorf("native parser: unsupported file type %q", ft)
	}
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(md) == "" {
		return nil, fmt.Errorf("native parser: no text extracted from %s", ft)
	}
	return &types.ReadResult{MarkdownContent: md}, nil
}

func normalizeNativeType(fileType string) string {
	return strings.TrimPrefix(strings.ToLower(strings.TrimSpace(fileType)), ".")
}
