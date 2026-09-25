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

func TestTaskCheckboxWritesBox(t *testing.T) {
	tests := []struct {
		name      string
		checked   bool
		wantLines int
	}{
		{"open", false, 0},
		{"done", true, 2},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "checkbox.pdf")
			document := New()
			document.Canvas().Checkbox("", test.checked)
			if err := document.Write(path); err != nil {
				t.Fatal(err)
			}
			content, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			stream := contentStream(t, content)
			if !bytes.Contains(stream, []byte(" re S")) {
				t.Fatalf("checkbox has no rect operator in %q", stream)
			}
			if got := bytes.Count(stream, []byte(" l S")); got != test.wantLines {
				t.Fatalf("checkbox has %d line segments, want %d: %q", got, test.wantLines, stream)
			}
		})
	}
}

func TestQuoteDrawsLineAtSixty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "quote.pdf")
	document := New()
	canvas := document.Canvas()
	canvas.Quote(1, func() {
		canvas.LineBreak(15)
	})
	if err := document.Write(path); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	stream := contentStream(t, content)
	line := regexp.MustCompile(`60\.00 [0-9]+\.[0-9]+ m 60\.00 [0-9]+\.[0-9]+ l S`)
	if !line.Match(stream) {
		t.Fatalf("quote has no vertical line at x = 60: %q", stream)
	}
}

func TestQuoteLineSpansPageBreak(t *testing.T) {
	path := filepath.Join(t.TempDir(), "quote-pages.pdf")
	document := New()
	blocks := make([]markdown.Block, 40)
	for i := range blocks {
		blocks[i] = markdown.Block{Kind: markdown.Paragraph, Quote: 1, Spans: []markdown.Span{{Text: "quote"}}}
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
	if count := pageCount(content); count != 2 {
		t.Fatalf("quote spans %d pages, want 2", count)
	}
	streams := contentStreams(t, content)
	if len(streams) != 2 {
		t.Fatalf("quote has %d content streams, want 2", len(streams))
	}
	line := regexp.MustCompile(`60\.00 [0-9]+\.[0-9]+ m 60\.00 [0-9]+\.[0-9]+ l S`)
	for i, stream := range streams {
		if !line.Match(stream) {
			t.Fatalf("page %d has no quote line: %q", i+1, stream)
		}
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

func TestLinkIsBlueAndUnderlined(t *testing.T) {
	path := filepath.Join(t.TempDir(), "link.pdf")
	document := New()
	blocks := []markdown.Block{{Kind: markdown.Paragraph, Spans: []markdown.Span{
		{Text: "before "},
		{Text: "site", URL: "https://x.example"},
		{Text: " after"},
	}}}
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
	if !bytes.Contains(stream, []byte("0.000 0.275 0.627 rg")) {
		t.Fatalf("link color missing in %q", stream)
	}
	if !bytes.Contains(stream, []byte("re f")) {
		t.Fatalf("link underline missing in %q", stream)
	}
	if got := bytes.Count(stream, []byte("0.000 0.275 0.627 rg")); got != 1 {
		t.Fatalf("link color appears %d times, want 1 so text after the link is black again: %q", got, stream)
	}
}

func TestStrikethroughDrawsLine(t *testing.T) {
	tests := []struct {
		name string
		span markdown.Span
		line bool
	}{
		{"struck", markdown.Span{Text: "weg", Strike: true}, true},
		{"plain", markdown.Span{Text: "gewoon"}, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "strike.pdf")
			document := New()
			blocks := []markdown.Block{{Kind: markdown.Paragraph, Spans: []markdown.Span{test.span}}}
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
			if has := bytes.Contains(stream, []byte("re f")); has != test.line {
				t.Fatalf("strikeout line present = %v, want %v: %q", has, test.line, stream)
			}
		})
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

func TestListItemContinuationSharesTextColumn(t *testing.T) {
	path := filepath.Join(t.TempDir(), "continuation.pdf")
	document := New()
	blocks := []markdown.Block{
		{Kind: markdown.ListItem, Depth: 0, Spans: []markdown.Span{{Text: "first"}}},
		{Kind: markdown.ListItem, Depth: 0, Continued: true, InItem: true, Spans: []markdown.Span{{Text: "second"}}},
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
	if len(columns) < 3 {
		t.Fatalf("too few text columns to compare paragraphs: %q", stream)
	}
	if columns[1] != columns[2] {
		t.Fatalf("second paragraph is at %.2f, want %.2f: %q", columns[2], columns[1], stream)
	}
}

func TestCodeBlockInListItemIndentsPastMarker(t *testing.T) {
	path := filepath.Join(t.TempDir(), "code.pdf")
	document := New()
	blocks := []markdown.Block{
		{Kind: markdown.ListItem, Depth: 0, Spans: []markdown.Span{{Text: "item"}}},
		{Kind: markdown.CodeBlock, Depth: 0, InItem: true, Lines: []string{"code"}},
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
	if len(columns) < 3 {
		t.Fatalf("too few text columns to compare code and marker: %q", stream)
	}
	if columns[2] <= columns[0] {
		t.Fatalf("code block is at %.2f, want right of marker at %.2f: %q", columns[2], columns[0], stream)
	}
}

func TestQuotedListItemIndentsPastPlainListItem(t *testing.T) {
	path := filepath.Join(t.TempDir(), "quote-list.pdf")
	document := New()
	blocks := []markdown.Block{
		{Kind: markdown.ListItem, Depth: 0, Spans: []markdown.Span{{Text: "plain"}}},
		{Kind: markdown.ListItem, Depth: 0, Quote: 1, Spans: []markdown.Span{{Text: "quoted"}}},
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
		t.Fatalf("too few text columns to compare quoted and plain items: %q", stream)
	}
	if columns[3] <= columns[1] {
		t.Fatalf("quoted item text is at %.2f, want right of plain item text at %.2f: %q", columns[3], columns[1], stream)
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

func TestCellLinesPreserveStyledText(t *testing.T) {
	document := New()
	canvas := document.Canvas().(documentCanvas)
	tests := []struct {
		name  string
		cell  markdown.Cell
		width float64
		lines int
	}{
		{"one run", markdown.Cell{Spans: []markdown.Span{{Text: "plain"}}}, 100, 1},
		{"mixed runs", markdown.Cell{Spans: []markdown.Span{{Text: "bold", Bold: true}, {Text: " code", Code: true}}}, 100, 1},
		{"breaks between runs", markdown.Cell{Spans: []markdown.Span{{Text: "first "}, {Text: "second", Bold: true}}}, 50, 2},
		{"wide word", markdown.Cell{Spans: []markdown.Span{{Text: "toolong"}}}, 40, 2},
		{"hard newline", markdown.Cell{Spans: []markdown.Span{{Text: "one\ntwo"}}}, 100, 2},
		{"empty cell", markdown.Cell{}, 100, 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			lines := canvas.cellLines(test.cell, test.width, false)
			if len(lines) != test.lines {
				t.Fatalf("got %d lines, want %d", len(lines), test.lines)
			}
			var got, want strings.Builder
			for _, line := range lines {
				for _, run := range line {
					got.WriteString(run.text)
				}
			}
			for _, span := range test.cell.Spans {
				want.WriteString(span.Text)
			}
			if strings.ReplaceAll(strings.ReplaceAll(got.String(), " ", ""), "\n", "") != strings.ReplaceAll(strings.ReplaceAll(want.String(), " ", ""), "\n", "") {
				t.Fatalf("run text %q differs from span text %q", got.String(), want.String())
			}
		})
	}
}

func TestTableStyledRunsAndLinks(t *testing.T) {
	path := filepath.Join(t.TempDir(), "styled-table.pdf")
	document := New()
	document.Canvas().Table([]markdown.Row{{Cells: []markdown.Cell{{Spans: []markdown.Span{
		{Text: "bold", Bold: true}, {Text: " code", Code: true}, {Text: " link", URL: "https://table.example"},
	}}}}})
	if err := document.Write(path); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(content, []byte("/BaseFont /Helvetica-Bold")) || !bytes.Contains(content, []byte("/BaseFont /Courier")) {
		t.Fatal("styled table does not embed bold Helvetica and Courier")
	}
	if !bytes.Contains(content, []byte("/URI (https://table.example)")) {
		t.Fatal("table link annotation is missing")
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

func TestCheckboxSitsOnItsTextLine(t *testing.T) {
	document := New()
	canvas := document.Canvas().(documentCanvas)
	canvas.Indent(14)
	lineTop := document.pdf.GetY()
	canvas.Checkbox("", true)
	canvas.Indent(-14)
	path := filepath.Join(t.TempDir(), "box.pdf")
	if err := document.Write(path); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	match := regexp.MustCompile(`([0-9.]+) ([0-9.]+) ([0-9.]+) (-[0-9.]+) re`).FindSubmatch(contentStream(t, content))
	if match == nil {
		t.Fatal("no rectangle for the checkbox")
	}
	_, pageHeight := document.pdf.GetPageSize()
	pdfY, _ := strconv.ParseFloat(string(match[2]), 64)
	boxTop := pageHeight - pdfY
	if boxTop < lineTop || boxTop+8 > lineTop+15 {
		t.Fatalf("checkbox spans %.2f to %.2f, want inside the line %.2f to %.2f", boxTop, boxTop+8, lineTop, lineTop+15)
	}
}

// infoString reads a string from the PDF information dictionary; fpdf writes
// UTF-8 strings as UTF-16BE with a byte order mark.
func infoString(t *testing.T, pdf []byte, key string) (string, bool) {
	t.Helper()
	marker := []byte("/" + key + " (\xfe\xff")
	at := bytes.Index(pdf, marker)
	if at < 0 {
		return "", false
	}
	raw := pdf[at+len(marker):]
	raw = raw[:bytes.IndexByte(raw, ')')]
	var out []rune
	for i := 0; i+1 < len(raw); i += 2 {
		out = append(out, rune(raw[i])<<8|rune(raw[i+1]))
	}
	return string(out), true
}

func TestSetInfoWritesTitleAuthorAndCreator(t *testing.T) {
	document := New()
	document.SetInfo("Café notes", "", "md2pdf test")
	target := filepath.Join(t.TempDir(), "info.pdf")
	if err := document.Write(target); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if got, ok := infoString(t, content, "Title"); !ok || got != "Café notes" {
		t.Fatalf("Title = %q, %v", got, ok)
	}
	if got, ok := infoString(t, content, "Creator"); !ok || got != "md2pdf test" {
		t.Fatalf("Creator = %q, %v", got, ok)
	}
	if _, ok := infoString(t, content, "Author"); ok {
		t.Fatal("an empty author must not be written")
	}
}

func TestPageNumbersAppearInTheFooter(t *testing.T) {
	document := New()
	document.SetPageNumbers(true)
	canvas := document.Canvas()
	canvas.Style("Helvetica", false, false, 11)
	canvas.Text("first")
	canvas.NewPage()
	canvas.Text("second")
	canvas.NewPage()
	canvas.Text("third")
	target := filepath.Join(t.TempDir(), "numbers.pdf")
	if err := document.Write(target); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	streams := contentStreams(t, content)
	if len(streams) != 3 {
		t.Fatalf("%d content streams, want 3", len(streams))
	}
	for i, stream := range streams {
		want := fmt.Sprintf("(%d / 3)", i+1)
		if !bytes.Contains(stream, []byte(want)) {
			t.Errorf("page %d has no %s", i+1, want)
		}
		// The footer sits in the bottom margin, below the text area (which ends 56 points above the edge).
		y := regexp.MustCompile(`BT [\d.]+ ([\d.]+) Td \(\d / 3\)`).FindSubmatch(stream)
		if y == nil {
			t.Fatalf("page %d: no footer position", i+1)
		}
		value, _ := strconv.ParseFloat(string(y[1]), 64)
		if value < 8 || value > 28 {
			t.Errorf("page %d: footer baseline at y=%v, want between 8 and 28 points above the bottom edge, below the text area", i+1, value)
		}
	}
}

func TestPageNumbersOffLeavesNoFooter(t *testing.T) {
	document := New()
	document.SetPageNumbers(false)
	document.Canvas().NewPage()
	target := filepath.Join(t.TempDir(), "plain.pdf")
	if err := document.Write(target); err != nil {
		t.Fatal(err)
	}
	content, _ := os.ReadFile(target)
	for _, stream := range contentStreams(t, content) {
		if bytes.Contains(stream, []byte(" / 2)")) {
			t.Fatal("footer present although page numbers are off")
		}
	}
}

func TestFooterDoesNotBreakTablesAcrossPages(t *testing.T) {
	rows := []markdown.Row{{Header: true, Cells: []markdown.Cell{{Spans: []markdown.Span{{Text: "Head"}}}}}}
	for i := 0; i < 120; i++ {
		rows = append(rows, markdown.Row{Cells: []markdown.Cell{{Spans: []markdown.Span{{Text: "cell"}}}}})
	}
	document := New()
	document.SetPageNumbers(true)
	document.Canvas().Table(rows)
	target := filepath.Join(t.TempDir(), "table.pdf")
	if err := document.Write(target); err != nil {
		t.Fatal(err)
	}
	content, _ := os.ReadFile(target)
	pages := pageCount(content)
	if pages < 2 {
		t.Fatalf("table fits on %d page", pages)
	}
	for i, stream := range contentStreams(t, content) {
		if !bytes.Contains(stream, []byte(fmt.Sprintf("(%d / %d)", i+1, pages))) {
			t.Errorf("page %d has no page number", i+1)
		}
	}
}

func TestPageBreakAfterFooterKeepsFontAndColour(t *testing.T) {
	document := New()
	document.SetPageNumbers(true)
	canvas := document.Canvas()
	canvas.Style("Courier", true, false, 14)
	canvas.Text("before")
	canvas.NewPage()
	canvas.Text("after")
	target := filepath.Join(t.TempDir(), "font.pdf")
	if err := document.Write(target); err != nil {
		t.Fatal(err)
	}
	content, _ := os.ReadFile(target)
	streams := contentStreams(t, content)
	if !strings.Contains(string(streams[0]), "0.502 g") {
		t.Fatal("the footer of page 1 is not grey; this test no longer checks anything")
	}
	second := string(streams[1])
	at := strings.Index(second, "(after)")
	if at < 0 {
		t.Fatal("second page has no text")
	}
	before := second[:at]
	if strings.Contains(before, "0.502 g") {
		t.Fatal("body text after the footer is still grey")
	}
}
