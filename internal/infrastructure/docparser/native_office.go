package docparser

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strconv"
	"strings"
)

// nativeDOCXToMarkdown extracts paragraphs and simple tables from a DOCX.
// It deliberately ignores styling, images and headers; the desktop native
// path is a local fallback, not a replacement for a layout-aware parser.
func nativeDOCXToMarkdown(data []byte) (string, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("native DOCX parser: %w", err)
	}
	var doc io.ReadCloser
	for _, f := range zr.File {
		if f.Name == "word/document.xml" {
			doc, err = f.Open()
			if err != nil {
				return "", fmt.Errorf("native DOCX parser: open document.xml: %w", err)
			}
			break
		}
	}
	if doc == nil {
		return "", fmt.Errorf("native DOCX parser: word/document.xml not found")
	}
	defer doc.Close()

	dec := xml.NewDecoder(doc)
	var out strings.Builder
	var para, cell strings.Builder
	var rows [][]string
	var row []string
	inTable, inCell := false, false

	flushParagraph := func() {
		text := strings.TrimSpace(para.String())
		if text != "" {
			out.WriteString(text)
			out.WriteString("\n\n")
		}
		para.Reset()
	}

	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("native DOCX parser: %w", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "tbl":
				inTable = true
				rows = nil
			case "tr":
				if inTable {
					row = nil
				}
			case "tc":
				if inTable {
					inCell = true
					cell.Reset()
				}
			case "p":
				if !inCell {
					para.Reset()
				}
			case "tab":
				if inCell {
					cell.WriteString("\t")
				} else {
					para.WriteString("\t")
				}
			case "br":
				if inCell {
					cell.WriteString("\n")
				} else {
					para.WriteString("\n")
				}
			}
		case xml.CharData:
			if inCell {
				cell.Write(t)
			} else {
				para.Write(t)
			}
		case xml.EndElement:
			switch t.Name.Local {
			case "p":
				if inTable {
					if inCell {
						cell.WriteString(" ")
					}
				} else {
					flushParagraph()
				}
			case "tc":
				if inTable {
					row = append(row, strings.TrimSpace(cell.String()))
					inCell = false
				}
			case "tr":
				if inTable && len(row) > 0 {
					rows = append(rows, row)
				}
			case "tbl":
				if inTable {
					out.WriteString(nativeRenderTable(rows))
					inTable = false
					rows = nil
				}
			}
		}
	}
	flushParagraph()
	return strings.TrimSpace(out.String()), nil
}

func nativeXLSXToMarkdown(data []byte) (string, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("native XLSX parser: %w", err)
	}
	shared := nativeXLSXSharedStrings(zr)
	files := make([]string, 0)
	for _, f := range zr.File {
		if strings.HasPrefix(f.Name, "xl/worksheets/sheet") && strings.HasSuffix(f.Name, ".xml") {
			files = append(files, f.Name)
		}
	}
	if len(files) == 0 {
		return "", fmt.Errorf("native XLSX parser: no worksheets found")
	}
	sort.Strings(files)

	var out strings.Builder
	for i, name := range files {
		md, err := nativeXLSXSheet(zr, name, shared)
		if err != nil {
			return "", err
		}
		if strings.TrimSpace(md) == "" {
			continue
		}
		fmt.Fprintf(&out, "## Sheet %d\n\n%s\n\n", i+1, md)
	}
	return strings.TrimSpace(out.String()), nil
}

func nativeXLSXSharedStrings(zr *zip.Reader) []string {
	var shared []string
	for _, f := range zr.File {
		if f.Name != "xl/sharedStrings.xml" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil
		}
		dec := xml.NewDecoder(rc)
		var text strings.Builder
		inSI, inT := false, false
		for {
			tok, err := dec.Token()
			if err != nil {
				break
			}
			switch t := tok.(type) {
			case xml.StartElement:
				switch t.Name.Local {
				case "si":
					inSI = true
					text.Reset()
				case "t":
					inT = true
				}
			case xml.CharData:
				if inSI && inT {
					text.Write(t)
				}
			case xml.EndElement:
				switch t.Name.Local {
				case "t":
					inT = false
				case "si":
					shared = append(shared, text.String())
					inSI = false
				}
			}
		}
		rc.Close()
		break
	}
	return shared
}

func nativeXLSXSheet(zr *zip.Reader, name string, shared []string) (string, error) {
	var rc io.ReadCloser
	for _, f := range zr.File {
		if f.Name == name {
			var err error
			rc, err = f.Open()
			if err != nil {
				return "", fmt.Errorf("native XLSX parser: open %s: %w", name, err)
			}
			break
		}
	}
	if rc == nil {
		return "", fmt.Errorf("native XLSX parser: %s not found", name)
	}
	defer rc.Close()

	rows := map[int]map[int]string{}
	maxRow, maxCol := 0, 0
	curRow, curCol := 0, 0
	var ref, cellType string
	var value strings.Builder
	inValue, inInline := false, false

	for {
		dec := xml.NewDecoder(rc)
		for {
			tok, err := dec.Token()
			if err == io.EOF {
				break
			}
			if err != nil {
				return "", fmt.Errorf("native XLSX parser: %s: %w", name, err)
			}
			switch t := tok.(type) {
			case xml.StartElement:
				switch t.Name.Local {
				case "row":
					if v := nativeXMLAttr(t, "r"); v != "" {
						if n, err := strconv.Atoi(v); err == nil {
							curRow = n
						}
					}
					curCol = 0
				case "c":
					ref = nativeXMLAttr(t, "r")
					cellType = nativeXMLAttr(t, "t")
					value.Reset()
				case "v":
					inValue = true
				case "is":
					inInline = true
				}
			case xml.CharData:
				if inValue || inInline {
					value.Write(t)
				}
			case xml.EndElement:
				switch t.Name.Local {
				case "v":
					inValue = false
				case "is":
					inInline = false
				case "c":
					r, c := nativeCellPosition(ref, curRow, curCol+1)
					curRow, curCol = r, c
					s := value.String()
					if cellType == "s" {
						if idx, err := strconv.Atoi(strings.TrimSpace(s)); err == nil && idx >= 0 && idx < len(shared) {
							s = shared[idx]
						}
					}
					if strings.TrimSpace(s) != "" {
						if rows[r] == nil {
							rows[r] = map[int]string{}
						}
						rows[r][c] = s
					}
					if r > maxRow {
						maxRow = r
					}
					if c > maxCol {
						maxCol = c
					}
				}
			}
		}
		break
	}
	if maxRow == 0 || maxCol == 0 {
		return "", nil
	}
	if maxRow > 5000 || maxCol > 200 {
		return "", fmt.Errorf("native XLSX parser: sheet too large (%d rows x %d columns)", maxRow, maxCol)
	}

	table := make([][]string, 0, maxRow)
	for r := 1; r <= maxRow; r++ {
		line := make([]string, maxCol)
		for c := 1; c <= maxCol; c++ {
			line[c-1] = rows[r][c]
		}
		table = append(table, line)
	}
	return nativeRenderTable(table), nil
}

func nativeXMLAttr(e xml.StartElement, name string) string {
	for _, a := range e.Attr {
		if a.Name.Local == name {
			return a.Value
		}
	}
	return ""
}

func nativeCellPosition(ref string, fallbackRow, fallbackCol int) (int, int) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return fallbackRow, fallbackCol
	}
	i := 0
	col := 0
	for i < len(ref) {
		ch := ref[i]
		if ch >= 'A' && ch <= 'Z' {
			col = col*26 + int(ch-'A'+1)
			i++
			continue
		}
		if ch >= 'a' && ch <= 'z' {
			col = col*26 + int(ch-'a'+1)
			i++
			continue
		}
		break
	}
	row, err := strconv.Atoi(ref[i:])
	if err != nil || row <= 0 || col <= 0 {
		return fallbackRow, fallbackCol
	}
	return row, col
}

func nativePPTXToMarkdown(data []byte) (string, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("native PPTX parser: %w", err)
	}
	type slideFile struct {
		name string
		num  int
	}
	var slides []slideFile
	for _, f := range zr.File {
		if !strings.HasPrefix(f.Name, "ppt/slides/slide") || !strings.HasSuffix(f.Name, ".xml") {
			continue
		}
		numText := strings.TrimSuffix(strings.TrimPrefix(f.Name, "ppt/slides/slide"), ".xml")
		n, err := strconv.Atoi(numText)
		if err != nil {
			continue
		}
		slides = append(slides, slideFile{name: f.Name, num: n})
	}
	if len(slides) == 0 {
		return "", fmt.Errorf("native PPTX parser: no slides found")
	}
	sort.Slice(slides, func(i, j int) bool { return slides[i].num < slides[j].num })

	var out strings.Builder
	for _, slide := range slides {
		rc, err := func() (io.ReadCloser, error) {
			for _, f := range zr.File {
				if f.Name == slide.name {
					return f.Open()
				}
			}
			return nil, fmt.Errorf("slide %s not found", slide.name)
		}()
		if err != nil {
			return "", fmt.Errorf("native PPTX parser: %w", err)
		}
		dec := xml.NewDecoder(rc)
		var text strings.Builder
		inText := false
		for {
			tok, err := dec.Token()
			if err == io.EOF {
				break
			}
			if err != nil {
				rc.Close()
				return "", fmt.Errorf("native PPTX parser: %s: %w", slide.name, err)
			}
			switch t := tok.(type) {
			case xml.StartElement:
				if t.Name.Local == "t" {
					inText = true
				}
			case xml.CharData:
				if inText {
					text.Write(t)
				}
			case xml.EndElement:
				switch t.Name.Local {
				case "t":
					inText = false
					text.WriteString("\n")
				case "p":
					text.WriteString("\n")
				}
			}
		}
		rc.Close()
		body := strings.TrimSpace(text.String())
		if body == "" {
			continue
		}
		fmt.Fprintf(&out, "## Slide %d\n\n%s\n\n", slide.num, body)
	}
	return strings.TrimSpace(out.String()), nil
}

func nativeDOCToMarkdown(ctx context.Context, data []byte) (string, error) {
	if runtime.GOOS != "darwin" {
		return "", fmt.Errorf("native DOC parser: only available on macOS")
	}
	f, err := os.CreateTemp("", "weknora-native-*.doc")
	if err != nil {
		return "", fmt.Errorf("native DOC parser: %w", err)
	}
	name := f.Name()
	defer os.Remove(name)
	if _, err := f.Write(data); err != nil {
		f.Close()
		return "", fmt.Errorf("native DOC parser: write temp file: %w", err)
	}
	if err := f.Close(); err != nil {
		return "", fmt.Errorf("native DOC parser: close temp file: %w", err)
	}
	cmd := exec.CommandContext(ctx, "textutil", "-convert", "txt", "-stdout", "-encoding", "UTF-8", name)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("native DOC parser: textutil: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func nativeRenderTable(rows [][]string) string {
	if len(rows) == 0 {
		return ""
	}
	cols := 0
	for _, row := range rows {
		if len(row) > cols {
			cols = len(row)
		}
	}
	if cols == 0 {
		return ""
	}
	var b strings.Builder
	for i, row := range rows {
		b.WriteString("|")
		for c := 0; c < cols; c++ {
			cell := ""
			if c < len(row) {
				cell = row[c]
			}
			b.WriteString(" ")
			b.WriteString(strings.ReplaceAll(strings.ReplaceAll(cell, "|", `\|`), "\n", "<br>"))
			b.WriteString(" |")
		}
		b.WriteString("\n")
		if i == 0 {
			b.WriteString("|")
			for c := 0; c < cols; c++ {
				b.WriteString(" --- |")
			}
			b.WriteString("\n")
		}
	}
	return b.String()
}
