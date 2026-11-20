package docparser

import (
	"archive/zip"
	"bytes"
	"context"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func nativeTestZip(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, body := range files {
		w, err := zw.Create(name)
		require.NoError(t, err)
		_, err = w.Write([]byte(body))
		require.NoError(t, err)
	}
	require.NoError(t, zw.Close())
	return buf.Bytes()
}

func TestNativeDOCXExtractsParagraphsAndTable(t *testing.T) {
	data := nativeTestZip(t, map[string]string{
		"word/document.xml": `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
<w:body>
<w:p><w:r><w:t>Hello WeKnora</w:t></w:r></w:p>
<w:tbl>
<w:tr><w:tc><w:p><w:r><w:t>A</w:t></w:r></w:p></w:tc><w:tc><w:p><w:r><w:t>B</w:t></w:r></w:p></w:tc></w:tr>
<w:tr><w:tc><w:p><w:r><w:t>1</w:t></w:r></w:p></w:tc><w:tc><w:p><w:r><w:t>2</w:t></w:r></w:p></w:tc></w:tr>
</w:tbl>
</w:body></w:document>`,
	})
	md, err := nativeDOCXToMarkdown(data)
	require.NoError(t, err)
	assert.Contains(t, md, "Hello WeKnora")
	assert.Contains(t, md, "| A | B |")
	assert.Contains(t, md, "| 1 | 2 |")
}

func TestNativeXLSXExtractsSharedStrings(t *testing.T) {
	data := nativeTestZip(t, map[string]string{
		"xl/sharedStrings.xml": `<sst xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><si><t>Name</t></si><si><t>Value</t></si><si><t>Alpha</t></si><si><t>42</t></si></sst>`,
		"xl/worksheets/sheet1.xml": `<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData>
<row r="1"><c r="A1" t="s"><v>0</v></c><c r="B1" t="s"><v>1</v></c></row>
<row r="2"><c r="A2" t="s"><v>2</v></c><c r="B2" t="s"><v>3</v></c></row>
</sheetData></worksheet>`,
	})
	md, err := nativeXLSXToMarkdown(data)
	require.NoError(t, err)
	assert.Contains(t, md, "| Name | Value |")
	assert.Contains(t, md, "| Alpha | 42 |")
}

func TestNativePPTXExtractsSlideText(t *testing.T) {
	data := nativeTestZip(t, map[string]string{
		"ppt/slides/slide1.xml": `<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><p:cSld><p:spTree><p:sp><p:txBody><a:p><a:r><a:t>Slide title</a:t></a:r></a:p></p:txBody></p:sp></p:spTree></p:cSld></p:sld>`,
	})
	md, err := nativePPTXToMarkdown(data)
	require.NoError(t, err)
	assert.Contains(t, md, "## Slide 1")
	assert.Contains(t, md, "Slide title")
}

func TestNativeHTMLConvertsToMarkdown(t *testing.T) {
	md, err := nativeHTMLToMarkdown([]byte(`<h1>Title</h1><p>Hello <strong>world</strong></p>`))
	require.NoError(t, err)
	assert.Contains(t, md, "# Title")
	assert.Contains(t, md, "Hello **world**")
}

func TestNativeReaderDispatchesHTML(t *testing.T) {
	reader := &NativeReader{}
	result, err := reader.Read(context.Background(), &types.ReadRequest{
		FileName: "page.html",
		FileType: "html",
		FileContent: []byte(`<h1>Local</h1><p>body</p>`),
	})
	require.NoError(t, err)
	assert.Contains(t, result.MarkdownContent, "# Local")
}
