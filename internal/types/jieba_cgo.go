//go:build cgo

package types

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"

	"github.com/yanyiwu/gojieba"
)

// gojieba resolves its default dictionaries relative to the build machine's
// module cache path (baked into the binary via runtime.Caller), so a binary
// moved to any other machine panics at init. Embed the dictionaries instead
// and materialize them under the user config directory on first start.
//
//go:embed jiebadata/jieba.dict.utf8 jiebadata/hmm_model.utf8 jiebadata/user.dict.utf8 jiebadata/idf.utf8 jiebadata/stop_words.utf8
var jiebaDictFS embed.FS

var jiebaDictNames = []string{
	"jieba.dict.utf8",
	"hmm_model.utf8",
	"user.dict.utf8",
	"idf.utf8",
	"stop_words.utf8",
}

// Jieba is a global instance of Chinese text segmentation tool
var Jieba *gojieba.Jieba = newJieba()

func newJieba() *gojieba.Jieba {
	if dictDir := os.Getenv("JIEBA_DICT_DIR"); dictDir != "" {
		return gojieba.NewJieba(dictPaths(dictDir)...)
	}

	dir, err := extractJiebaDicts()
	if err != nil {
		// 无法写用户目录时退回系统临时目录；两者都不可写属于环境级故障
		if dir2, err2 := extractJiebaDictsTo(os.TempDir()); err2 == nil {
			return gojieba.NewJieba(dictPaths(dir2)...)
		}
		panic(fmt.Sprintf("initialize jieba dictionaries: %v", err))
	}
	return gojieba.NewJieba(dictPaths(dir)...)
}

func dictPaths(dir string) []string {
	paths := make([]string, 0, len(jiebaDictNames))
	for _, name := range jiebaDictNames {
		paths = append(paths, filepath.Join(dir, name))
	}
	return paths
}

// extractJiebaDicts materializes the embedded dictionaries under the user
// config directory and returns that directory. Idempotent: existing files
// are left untouched so dictionaries can be customized in place.
func extractJiebaDicts() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return extractJiebaDictsTo(filepath.Join(base, "WeKnora Lite", "dict"))
}

func extractJiebaDictsTo(dir string) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	for _, name := range jiebaDictNames {
		target := filepath.Join(dir, name)
		if _, err := os.Stat(target); err == nil {
			continue
		}
		data, err := jiebaDictFS.ReadFile("jiebadata/" + name)
		if err != nil {
			return "", err
		}
		if err := os.WriteFile(target, data, 0o644); err != nil {
			return "", err
		}
	}
	return dir, nil
}
