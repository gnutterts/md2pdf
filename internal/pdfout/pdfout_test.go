// SPDX-License-Identifier: MIT

package pdfout

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gnutterts/md2pdf/internal/markdown"
	"github.com/gnutterts/md2pdf/internal/render"
)

func TestDiagramPNG(t *testing.T) {
	var pngBytes bytes.Buffer
	pngImage := image.NewRGBA(image.Rect(0, 0, 40, 20))
	pngImage.Set(0, 0, color.Black)
	if err := png.Encode(&pngBytes, pngImage); err != nil {
		t.Fatal(err)
	}
	document := New()
	if err := document.Canvas().Diagram(pngBytes.Bytes()); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "diagram.pdf")
	if err := document.Write(path); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(content, []byte("/Subtype /Image")) {
		t.Fatal("PDF contains no image")
	}
}

func TestDiagramInvalidPNGReturnsError(t *testing.T) {
	document := New()
	if err := document.Canvas().Diagram([]byte("not PNG")); err == nil {
		t.Fatal("invalid PNG did not return an error")
	}
}

func TestWritePDF(t *testing.T) {
	path := filepath.Join(t.TempDir(), "paragraph.pdf")
	document := New()
	if err := render.Draw([]markdown.Block{{Kind: markdown.Paragraph, Spans: []markdown.Span{{Text: "A paragraph."}}}}, document.Canvas(), render.Options{}); err != nil {
		t.Fatal(err)
	}
	if err := document.Write(path); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(content, []byte("%PDF-")) || !bytes.HasSuffix(bytes.TrimSpace(content), []byte("%%EOF")) || len(content) <= 500 {
		t.Fatalf("invalide PDF van %d bytes", len(content))
	}
}

func TestCP1252IsInContentStream(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tekens.pdf")
	document := New()
	blocks := []markdown.Block{{Kind: markdown.Paragraph, Spans: []markdown.Span{{Text: "café —"}}}}
	if err := render.Draw(blocks, document.Canvas(), render.Options{}); err != nil {
		t.Fatal(err)
	}
	if err := document.Write(path); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	stream := contentStream(t, content)
	if !bytes.Contains(stream, []byte{0xe9}) {
		t.Fatalf("cp1252-byte 0xe9 is missing in % x", stream)
	}
	if bytes.Contains(stream, []byte{0xc3, 0xa9}) {
		t.Fatalf("UTF-8 bytes found in % x", stream)
	}
}

func TestLongListItemStaysIndented(t *testing.T) {
	path := filepath.Join(t.TempDir(), "list.pdf")
	document := New()
	blocks := []markdown.Block{
		{Kind: markdown.ListItem, Depth: 1, Spans: []markdown.Span{{Text: strings.Repeat("a long list item ", 40)}}},
		{Kind: markdown.Paragraph, Spans: []markdown.Span{{Text: "end"}}},
	}
	if err := render.Draw(blocks, document.Canvas(), render.Options{}); err != nil {
		t.Fatal(err)
	}
	if err := document.Write(path); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	stream := contentStream(t, content)
	columns := textColumns(t, stream)
	if len(columns) < 4 {
		t.Fatalf("te weinig textregels om terugloop te beoordelen: %q", stream)
	}
	// The first line is the bullet, followed by the item text; all
	// subsequent lines should hang below that text column, not below the bullet.
	textColumn := columns[1]
	for i, column := range columns[2 : len(columns)-1] {
		if column != textColumn {
			t.Fatalf("continuation line %d is at %.2f, want %.2f: %q", i+1, column, textColumn, stream)
		}
	}
	if final := columns[len(columns)-1]; final >= columns[0] {
		t.Fatalf("left margin was not restored: paragraph at %.2f, bullet was at %.2f", final, columns[0])
	}
}

func TestTableTextIsInContentStream(t *testing.T) {
	path := filepath.Join(t.TempDir(), "table.pdf")
	document := New()
	rows := []markdown.Row{
		{Header: true, Cells: []markdown.Cell{
			{Spans: []markdown.Span{{Text: "Name"}}},
			{Spans: []markdown.Span{{Text: "Value"}}},
		}},
		{Cells: []markdown.Cell{
			{Spans: []markdown.Span{{Text: "one"}}},
			{Spans: []markdown.Span{{Text: "1"}}},
		}},
		{Cells: []markdown.Cell{
			{Spans: []markdown.Span{{Text: "two"}}},
			{Spans: []markdown.Span{{Text: "2"}}},
		}},
	}
	document.Canvas().Table(rows)
	if err := document.Write(path); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	stream := contentStream(t, content)
	if !bytes.Contains(stream, []byte("Name")) {
		t.Fatalf("header row is missing in %q", stream)
	}
	if !bytes.Contains(stream, []byte("two")) {
		t.Fatalf("last data row is missing in %q", stream)
	}
}

func TestTableTooWideWritesWithoutError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "breed.pdf")
	document := New()
	rows := []markdown.Row{
		{Header: true, Cells: []markdown.Cell{{Spans: []markdown.Span{{Text: strings.Repeat("header ", 200)}}}}},
		{Cells: []markdown.Cell{{Spans: []markdown.Span{{Text: strings.Repeat("breed ", 200)}}}}},
	}
	document.Canvas().Table(rows)
	if err := document.Canvas().Err(); err != nil {
		t.Fatalf("Err() after drawing: %v", err)
	}
	if err := document.Write(path); err != nil {
		t.Fatal(err)
	}
	if err := document.Canvas().Err(); err != nil {
		t.Fatalf("Err() after writing: %v", err)
	}
}

func TestTableRepeatsHeaderAcrossPages(t *testing.T) {
	path := filepath.Join(t.TempDir(), "paginas.pdf")
	document := New()
	rows := []markdown.Row{
		{Header: true, Cells: []markdown.Cell{{Spans: []markdown.Span{{Text: "Header"}}}}},
	}
	for i := 0; i < 60; i++ {
		rows = append(rows, markdown.Row{Cells: []markdown.Cell{{Spans: []markdown.Span{{Text: fmt.Sprintf("row%d", i)}}}}})
	}
	document.Canvas().Table(rows)
	if err := document.Write(path); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if count := pageCount(content); count <= 1 {
		t.Fatalf("PDF has %d pages, want meer dan één", count)
	}
	headers := 0
	for _, stream := range contentStreams(t, content) {
		headers += bytes.Count(stream, []byte("Header"))
	}
	if headers <= 1 {
		t.Fatalf("header row appears %d times, want vaker dan één", headers)
	}
}

// textColumns reads the x position of every text placement from a content stream.
func textColumns(t *testing.T, stream []byte) []float64 {
	t.Helper()
	var columns []float64
	for _, match := range regexp.MustCompile(`BT (\d+\.\d+) \d+\.\d+ Td`).FindAllSubmatch(stream, -1) {
		x, err := strconv.ParseFloat(string(match[1]), 64)
		if err != nil {
			t.Fatal(err)
		}
		columns = append(columns, x)
	}
	return columns
}

// contentStream extracts the first content stream from a PDF.
func contentStream(t *testing.T, pdf []byte) []byte {
	t.Helper()
	stromen := contentStreams(t, pdf)
	return stromen[0]
}

// contentStreams extracts all zlib content streams from a PDF.
func contentStreams(t *testing.T, pdf []byte) [][]byte {
	t.Helper()
	var out [][]byte
	remaining := pdf
	for {
		start := bytes.Index(remaining, []byte("stream\n"))
		if start < 0 {
			break
		}
		start += len("stream\n")
		end := bytes.Index(remaining[start:], []byte("\nendstream"))
		if end < 0 {
			break
		}
		reader, err := zlib.NewReader(bytes.NewReader(remaining[start : start+end]))
		if err != nil {
			t.Fatal(err)
		}
		stream, err := io.ReadAll(reader)
		if err != nil {
			t.Fatal(err)
		}
		if err := reader.Close(); err != nil {
			t.Fatal(err)
		}
		out = append(out, stream)
		remaining = remaining[start+end+len("\nendstream"):]
	}
	if len(out) == 0 {
		t.Fatal("no content streams found")
	}
	return out
}

// pageCount counts the page objects in a PDF.
func pageCount(pdf []byte) int {
	text := string(pdf)
	return strings.Count(text, "/Type /Page") - strings.Count(text, "/Type /Pages")
}

func TestRenderREADMEToPDF(t *testing.T) {
	source, err := os.ReadFile(filepath.Join("..", "..", "testdata", "sample.md"))
	if err != nil {
		t.Fatal(err)
	}
	blocks, err := markdown.Parse(source)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "readme.pdf")
	document := New()
	if err := render.Draw(blocks, document.Canvas(), render.Options{}); err != nil {
		t.Fatal(err)
	}
	if err := document.Write(path); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Size() == 0 {
		t.Fatalf("output = %v, %v", info, err)
	}
}

func TestDiagramTallerThanOnePageIsScaled(t *testing.T) {
	document := New()
	// A narrow and very tall image: at the full text width it would become well
	// taller than a page.
	narrowAndTall := image.NewRGBA(image.Rect(0, 0, 100, 900))
	var pngBytes bytes.Buffer
	if err := png.Encode(&pngBytes, narrowAndTall); err != nil {
		t.Fatal(err)
	}
	if err := document.Canvas().Diagram(pngBytes.Bytes()); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "diagram.pdf")
	if err := document.Write(path); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if pageCount(content) != 1 {
		t.Fatalf("diagram spans %d pages, want one", pageCount(content))
	}
}

func TestWrapCode(t *testing.T) {
	tests := []struct {
		name string
		line string
		max  int
		want []string
	}{
		{"short line unchanged", "abc", 5, []string{"abc"}},
		{"empty line", "", 5, []string{""}},
		{"exact max", "abcde", 5, []string{"abcde"}},
		{"break on space", "aaa bbb ccc", 6, []string{"aaa ", "bbb ", "ccc"}},
		{"no space in second half hard breaks", "aa bbbbbb", 6, []string{"aa bbb", "bbb"}},
		{"max zero", "abcdef", 0, []string{"a", "b", "c", "d", "e", "f"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := wrapCode(test.line, test.max)
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("wrapCode(%q, %d) = %q, want %q", test.line, test.max, got, test.want)
			}
			limit := test.max
			if limit < 1 {
				limit = 1
			}
			if strings.Join(got, "") != test.line {
				t.Fatalf("wrapCode(%q, %d) pieces join to %q", test.line, test.max, strings.Join(got, ""))
			}
			for _, piece := range got {
				if len(piece) > limit {
					t.Fatalf("wrapCode(%q, %d) piece %q exceeds %d bytes", test.line, test.max, piece, limit)
				}
			}
		})
	}
}

func TestCodeBlockLongLineIsNotTruncated(t *testing.T) {
	document := New()
	var lineBuilder strings.Builder
	for i := 0; lineBuilder.Len() < 300; i++ {
		lineBuilder.WriteString(strconv.Itoa(i))
	}
	line := lineBuilder.String()[:300]

	blocks := []markdown.Block{{Kind: markdown.CodeBlock, Lines: []string{line}}}
	if err := render.Draw(blocks, document.Canvas(), render.Options{}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "wrapped.pdf")
	if err := document.Write(path); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	stream := contentStream(t, content)
	shown := regexp.MustCompile(`\(([0-9]+)\) ?Tj`).FindAllSubmatch(stream, -1)
	if len(shown) < 2 {
		t.Fatalf("want the line wrapped over several rows, got %d", len(shown))
	}
	var joined []byte
	for _, match := range shown {
		joined = append(joined, match[1]...)
	}
	if string(joined) != line {
		t.Fatalf("drawn text differs from the source line:\n got %s\nwant %s", joined, line)
	}
}

func TestCodeBlockLongLineDrawsQuickly(t *testing.T) {
	document := New()
	line := strings.Repeat("x", 200000)
	blocks := []markdown.Block{{Kind: markdown.CodeBlock, Lines: []string{line}}}
	start := time.Now()
	if err := render.Draw(blocks, document.Canvas(), render.Options{}); err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("drawing a 200000 character code line took %v, want at most 2s", elapsed)
	}
}
