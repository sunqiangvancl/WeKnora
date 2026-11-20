package docparser

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	htmltomd "github.com/JohannesKaufmann/html-to-markdown/v2"
	"github.com/Tencent/WeKnora/internal/utils"
)

const nativeHTMLMaxBytes = 20 << 20

func nativeHTMLToMarkdown(data []byte) (string, error) {
	if len(data) == 0 {
		return "", fmt.Errorf("native HTML parser: empty file")
	}
	md, err := htmltomd.ConvertString(string(data))
	if err != nil {
		return "", fmt.Errorf("native HTML parser: %w", err)
	}
	return strings.TrimSpace(md), nil
}

func nativeURLToMarkdown(ctx context.Context, rawURL string) (string, error) {
	if err := utils.ValidateURLForSSRF(rawURL); err != nil {
		return "", fmt.Errorf("native URL parser: %w", err)
	}
	client := utils.NewSSRFSafeHTTPClient(utils.SSRFSafeHTTPClientConfig{
		Timeout:       30 * time.Second,
		MaxRedirects: 5,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", fmt.Errorf("native URL parser: %w", err)
	}
	req.Header.Set("User-Agent", "WeKnora-Lite/1.0")
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("native URL parser: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("native URL parser: HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, nativeHTMLMaxBytes+1))
	if err != nil {
		return "", fmt.Errorf("native URL parser: read body: %w", err)
	}
	if len(body) > nativeHTMLMaxBytes {
		return "", fmt.Errorf("native URL parser: response exceeds %d bytes", nativeHTMLMaxBytes)
	}
	return nativeHTMLToMarkdown(body)
}
