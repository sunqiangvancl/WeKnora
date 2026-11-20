package docparser

import (
	"testing"

	"github.com/Tencent/WeKnora/internal/infrastructure/docparser/anydoc"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/utils"
)

// Locally advertised parser formats must pass the shared import gate used by
// direct uploads, URL imports, workers and datasource connectors (#4598).
// Check metadata even when a converter is not linked into this test build.
func TestLocalParserFormatsPassImportValidation(t *testing.T) {
	for _, engine := range ListAllEngines(true, nil, nil) {
		t.Run(engine.Name, func(t *testing.T) {
			for _, ext := range engine.FileTypes {
				if !utils.IsSupportedImportExtension(ext) {
					t.Errorf("engine advertises %q but the shared import gate rejects it", ext)
				}
			}
		})
	}
}

func TestListAllEnginesBuiltinIncludesDocumentFormats(t *testing.T) {
	engines := ListAllEngines(true, nil, nil)
	for _, engine := range engines {
		if engine.Name != "builtin" {
			continue
		}
		if !engine.Available {
			t.Fatalf("builtin engine is unavailable: %s", engine.UnavailableReason)
		}

		fileTypes := make(map[string]bool, len(engine.FileTypes))
		for _, fileType := range engine.FileTypes {
			fileTypes[fileType] = true
		}
		for _, want := range []string{"html", "htm", "xmind", "ppt", "pptx"} {
			if !fileTypes[want] {
				t.Errorf("builtin engine file types do not include %q: %v", want, engine.FileTypes)
			}
		}
		return
	}

	t.Fatal("builtin engine not found")
}

func TestDefaultParserEnginePrefersAnydocWhenLinked(t *testing.T) {
	// The desktop build intentionally defaults some formats to the in-process
	// native parser. Derive the expectation from the same build-tagged oracle
	// the product uses, so this test holds for both desktop (native) and
	// non-desktop (official) builds without hard-coding either outcome.
	expected := func(ft, official string) string {
		if n := preferredNativeEngine(ft); n != "" {
			return n
		}
		return official
	}
	cases := []string{"pptx", "ppt", "docx"}
	if anydoc.Available() {
		for _, ft := range cases {
			want := expected(ft, AnydocEngineName)
			if got := types.DefaultParserEngine(ft); got != want {
				t.Errorf("DefaultParserEngine(%s) = %q, want %q when the binding is linked", ft, got, want)
			}
		}
		if got := types.DefaultParserEngine("csv"); got != "" {
			t.Errorf("DefaultParserEngine(csv) = %q, want empty so the Go simple reader stays default", got)
		}
		// anydoc claims pdf but cannot model one, so linking the binding must
		// not move PDFs off builtin. This branch is where that is observable.
		for _, ft := range []string{"pdf", ".PDF"} {
			if got := types.DefaultParserEngine(ft); got != "" {
				t.Errorf("DefaultParserEngine(%q) = %q, want empty so PDFs stay on builtin", ft, got)
			}
		}
		return
	}
	if got, want := types.DefaultParserEngine("pptx"), expected("pptx", "markitdown"); got != want {
		t.Fatalf("DefaultParserEngine(pptx) = %q, want %q when anydoc is unavailable", got, want)
	}
	if got, want := types.DefaultParserEngine("docx"), expected("docx", ""); got != want {
		t.Fatalf("DefaultParserEngine(docx) = %q, want %q when anydoc is unavailable", got, want)
	}
}

// The PDF exclusion only matters because anydoc still claims the type. If a
// future anydoc drops pdf, the exclusion becomes dead code and should go with
// it — this guard runs in every build, tagged or not.
func TestPreferAnydocSkipsPDF(t *testing.T) {
	if !anydoc.Supports("pdf", "") {
		t.Fatal("anydoc no longer claims pdf; drop the explicit exclusion in preferAnydocWhenAvailable")
	}
	for _, ft := range []string{"pdf", ".PDF", " Pdf "} {
		if got := preferAnydocWhenAvailable(ft); got != "" {
			t.Errorf("preferAnydocWhenAvailable(%q) = %q, want empty so PDFs stay on builtin", ft, got)
		}
	}
}

// Self-hosted MinerU parses xlsx natively since 3.1.0, so the engine must
// advertise it; the parser settings UI only offers an engine for a file type
// when the engine lists it (#4099). xls is deliberately not advertised: only
// the 4.x docs list it and the engine cannot detect the server version, so an
// older server would fail at parse time.
func TestMinerUEngineAdvertisesXlsxOnly(t *testing.T) {
	var found bool
	for _, engine := range ListAllEngines(true, nil, nil) {
		if engine.Name != MinerUEngineName {
			continue
		}
		found = true
		fileTypes := make(map[string]bool, len(engine.FileTypes))
		for _, fileType := range engine.FileTypes {
			fileTypes[fileType] = true
		}
		if !fileTypes["xlsx"] {
			t.Errorf("mineru engine file types do not include %q: %v", "xlsx", engine.FileTypes)
		}
		// Guard against re-adding xls without a way to detect the server version.
		if fileTypes["xls"] {
			t.Errorf("mineru engine must not advertise %q: %v", "xls", engine.FileTypes)
		}
		// Formats it already served must stay.
		for _, keep := range []string{"pdf", "docx", "pptx"} {
			if !fileTypes[keep] {
				t.Errorf("mineru engine dropped %q: %v", keep, engine.FileTypes)
			}
		}
	}
	if !found {
		t.Fatal("mineru engine not found")
	}
}
