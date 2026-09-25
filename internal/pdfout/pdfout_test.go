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
	"unicode/utf16"

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
	if err := document.Canvas().Diagram(pngBytes.Bytes(), 1); err != nil {
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
	if err := document.Canvas().Diagram([]byte("not PNG"), 1); err == nil {
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

func TestQuoteBarSitsInTheGutter(t *testing.T) {
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
	bar := fmt.Sprintf("%.2f", DefaultMargin+4) // the bar sits 4 points right of the margin
	line := regexp.MustCompile(bar + ` [0-9]+\.[0-9]+ m ` + bar + ` [0-9]+\.[0-9]+ l S`)
	if !line.Match(stream) {
		t.Fatalf("quote has no vertical line at x = %s: %q", bar, stream)
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
	bar := fmt.Sprintf("%.2f", DefaultMargin+4)
	line := regexp.MustCompile(bar + ` [0-9]+\.[0-9]+ m ` + bar + ` [0-9]+\.[0-9]+ l S`)
	for i, stream := range streams {
		if !line.Match(stream) {
			t.Fatalf("page %d has no quote line: %q", i+1, stream)
		}
	}
}

func TestCharactersOutsideCP1252AreKept(t *testing.T) {
	path := filepath.Join(t.TempDir(), "characters.pdf")
	document := New()
	text := "café — Привет, Γειά → ✓"
	blocks := []markdown.Block{{Kind: markdown.Paragraph, Spans: []markdown.Span{{Text: text}}}}
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
	if !bytes.Contains(contentStream(t, content), []byte("("+text+")")) {
		t.Fatalf("the text is not kept: %s", contentStream(t, content))
	}
	if !bytes.Contains(content, []byte("/FontFile2")) {
		t.Fatal("no embedded TrueType font")
	}
	if len(content) > 100_000 {
		t.Fatalf("the PDF is %d bytes: the embedded font is not subset", len(content))
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
	if !bytes.Contains(content, []byte("/BaseFont /utf8dejavusansB")) || !bytes.Contains(content, []byte("/BaseFont /utf8dejavusansmono")) {
		t.Fatal("styled table does not embed bold DejaVu Sans and DejaVu Sans Mono")
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

// contentStreams returns the content of every page, in page order, with its
// text decoded (see decodeText).
func contentStreams(t *testing.T, pdf []byte) [][]byte {
	t.Helper()
	var out [][]byte
	for _, page := range regexp.MustCompile(`(?s)\d+ 0 obj\n<</Type /Page\n.*?/Contents (\d+) 0 R`).FindAllSubmatch(pdf, -1) {
		body := regexp.MustCompile(`(?s)\n` + string(page[1]) + ` 0 obj\n<<[^>]*>>\nstream\n(.*?)\nendstream`).FindSubmatch(pdf)
		if body == nil {
			t.Fatalf("no content stream %s", page[1])
		}
		reader, err := zlib.NewReader(bytes.NewReader(body[1]))
		if err != nil {
			t.Fatal(err)
		}
		stream, err := io.ReadAll(reader)
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, decodeText(stream))
	}
	if len(out) == 0 {
		t.Fatal("no content streams found")
	}
	return out
}

// decodeText rewrites the text strings of a content stream, which fpdf writes
// as escaped UTF-16BE for the embedded fonts, into plain UTF-8, so that tests
// can look for "(word)".
func decodeText(stream []byte) []byte {
	var out bytes.Buffer
	for i := 0; i < len(stream); i++ {
		if stream[i] != '(' {
			out.WriteByte(stream[i])
			continue
		}
		var raw []byte
		j := i + 1
		for ; j < len(stream) && stream[j] != ')'; j++ {
			if stream[j] == '\\' && j+1 < len(stream) {
				j++
				switch stream[j] {
				case 'n':
					raw = append(raw, '\n')
				case 'r':
					raw = append(raw, '\r')
				default:
					raw = append(raw, stream[j])
				}
				continue
			}
			raw = append(raw, stream[j])
		}
		units := make([]uint16, 0, len(raw)/2)
		for k := 0; k+1 < len(raw); k += 2 {
			units = append(units, uint16(raw[k])<<8|uint16(raw[k+1]))
		}
		out.WriteByte('(')
		out.WriteString(string(utf16.Decode(units)))
		out.WriteByte(')')
		i = j
	}
	return out.Bytes()
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
	if err := document.Canvas().Diagram(pngBytes.Bytes(), 1); err != nil {
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
		if value < 8 || value > DefaultMargin/2+4 {
			t.Errorf("page %d: footer baseline at y=%v, want between 8 points and half the margin above the bottom edge, below the text area", i+1, value)
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

// outlineTitles decodes the UTF-16 titles of the outline entries in document order.
func outlineTitles(content []byte) []string {
	var titles []string
	marker := []byte("/Title (\xfe\xff")
	for rest := content; ; {
		at := bytes.Index(rest, marker)
		if at < 0 {
			return titles
		}
		raw := rest[at+len(marker):]
		raw = raw[:bytes.IndexByte(raw, ')')]
		var units []uint16
		for i := 0; i+1 < len(raw); i += 2 {
			units = append(units, uint16(raw[i])<<8|uint16(raw[i+1]))
		}
		titles = append(titles, string(utf16.Decode(units)))
		rest = rest[at+len(marker):]
	}
}

func TestBookmarksFormAContiguousOutline(t *testing.T) {
	document := New()
	canvas := document.Canvas()
	canvas.Style("Helvetica", true, false, 20)
	for _, heading := range []struct {
		title string
		level int
	}{{"First", 3}, {"Deep", 5}, {"Back", 1}, {"Again", 3}} {
		canvas.Bookmark(heading.title, heading.level)
		canvas.Text(heading.title)
		canvas.LineBreak(20)
	}
	target := filepath.Join(t.TempDir(), "outline.pdf")
	if err := document.Write(target); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(content, []byte("/Outlines")) {
		t.Fatal("no outline in the PDF")
	}
	// Levels 3, 5, 1, 3 become depths 0, 1, 0, 1: two top entries with one child each.
	if got, want := outlineTitles(content), []string{"First", "Deep", "Back", "Again"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("outline titles = %v, want %v", got, want)
	}
	// fpdf writes /First for the outline root and for every entry that has children.
	if n := bytes.Count(content, []byte("/First ")); n != 3 {
		t.Fatalf("%d /First references, want 3 (the root and two parents)", n)
	}
}

func TestBookmarkTitlesKeepTheirCharacters(t *testing.T) {
	document := New()
	canvas := document.Canvas()
	canvas.Style("Helvetica", true, false, 20)
	canvas.Bookmark("Cost: \u20ac5 \u2014 \u201cquoted\u201d \u2122 caf\u00e9 \u0416", 1)
	target := filepath.Join(t.TempDir(), "unicode.pdf")
	if err := document.Write(target); err != nil {
		t.Fatal(err)
	}
	content, _ := os.ReadFile(target)
	want := "Cost: \u20ac5 \u2014 \u201cquoted\u201d \u2122 caf\u00e9 \u0416"
	if got := outlineTitles(content); len(got) != 1 || got[0] != want {
		t.Fatalf("outline titles = %q, want %q", got, want)
	}
}

func TestBookmarkAtTheBottomStartsOnTheNextPage(t *testing.T) {
	document := New()
	canvas := document.Canvas()
	canvas.Style("Helvetica", true, false, 20)
	for document.pdf.GetY() < 841.89-DefaultMargin-20 { // stop just above the bottom margin
		canvas.Text("filler")
		canvas.LineBreak(27)
	}
	canvas.Bookmark("Late", 1)
	canvas.Text("Late")
	target := filepath.Join(t.TempDir(), "late.pdf")
	if err := document.Write(target); err != nil {
		t.Fatal(err)
	}
	content, _ := os.ReadFile(target)
	dests := regexp.MustCompile(`/Dest \[(\d+) 0 R /XYZ`).FindSubmatch(content)
	if dests == nil {
		t.Fatal("no bookmark destination")
	}
	// The heading text is on the last page; the destination must be that page object.
	last := regexp.MustCompile(`(\d+) 0 obj\n<</Type /Page\n`).FindAllSubmatch(content, -1)
	if len(last) < 2 || string(dests[1]) != string(last[len(last)-1][1]) {
		t.Fatalf("bookmark points at page object %s, the heading is on %s", dests[1], last[len(last)-1][1])
	}
}

func TestBookmarkOnSecondPageTargetsThatPage(t *testing.T) {
	document := New()
	canvas := document.Canvas()
	canvas.Style("Helvetica", false, false, 11)
	canvas.Bookmark("Early", 1)
	canvas.NewPage()
	canvas.Bookmark("Late", 1)
	target := filepath.Join(t.TempDir(), "dest.pdf")
	if err := document.Write(target); err != nil {
		t.Fatal(err)
	}
	content, _ := os.ReadFile(target)
	dests := regexp.MustCompile(`/Dest \[(\d+) 0 R /XYZ`).FindAllSubmatch(content, -1)
	if len(dests) != 2 || string(dests[0][1]) == string(dests[1][1]) {
		t.Fatalf("both bookmarks point at the same page: %q", dests)
	}
}

func TestLayoutSetsPaperAndMargin(t *testing.T) {
	document := NewWithLayout(Layout{Paper: "Letter", Margin: 36})
	canvas := document.Canvas()
	canvas.Style("Helvetica", false, false, 11)
	canvas.Text("left edge")
	canvas.LineBreak(15)
	canvas.Rule()
	target := filepath.Join(t.TempDir(), "letter.pdf")
	if err := document.Write(target); err != nil {
		t.Fatal(err)
	}
	content, _ := os.ReadFile(target)
	if !bytes.Contains(content, []byte("/MediaBox [0 0 612.00 792.00]")) {
		t.Fatalf("page is not Letter: %s", regexp.MustCompile(`/MediaBox \[[^\]]*\]`).Find(content))
	}
	stream := contentStream(t, content)
	columns := textColumns(t, stream)
	if len(columns) == 0 || columns[0] < 38.7 || columns[0] > 39 {
		t.Fatalf("text starts at x=%v, want the 36 point margin plus fpdf's 2.83 point cell margin", columns)
	}
	// The rule runs from the left margin to the right margin: 36 .. 612-36.
	if !regexp.MustCompile(`36\.00 [\d.]+ m 576\.00 [\d.]+ l S`).Match(stream) {
		t.Fatalf("rule does not run from 36 to 576:\n%s", stream)
	}
}

func TestDefaultLayoutIsA4WithTheDefaultMargin(t *testing.T) {
	document := New()
	canvas := document.Canvas()
	canvas.Style("Helvetica", false, false, 11)
	canvas.Text("x")
	target := filepath.Join(t.TempDir(), "a4.pdf")
	if err := document.Write(target); err != nil {
		t.Fatal(err)
	}
	content, _ := os.ReadFile(target)
	if !bytes.Contains(content, []byte("/MediaBox [0 0 595.28 841.89]")) {
		t.Fatal("default page is not A4")
	}
	if columns := textColumns(t, contentStream(t, content)); len(columns) == 0 || columns[0] < DefaultMargin+2.7 || columns[0] > DefaultMargin+3 {
		t.Fatalf("text starts at %v, want %v plus fpdf's 2.83 point cell margin", columns, DefaultMargin)
	}
}

// lineBaselines reads the y position of every text placement in a content stream.
func lineBaselines(t *testing.T, stream []byte) []float64 {
	t.Helper()
	var ys []float64
	for _, match := range regexp.MustCompile(`BT [\d.]+ ([\d.]+) Td \(`).FindAllSubmatch(stream, -1) {
		y, err := strconv.ParseFloat(string(match[1]), 64)
		if err != nil {
			t.Fatal(err)
		}
		ys = append(ys, y)
	}
	return ys
}

func TestWrappedLinesFollowTheFontSize(t *testing.T) {
	for _, test := range []struct {
		size float64
		want float64
	}{{20, 27}, {16, 21.5}, {11, 15}} {
		document := New()
		canvas := document.Canvas()
		canvas.Style("Helvetica", true, false, test.size)
		canvas.Text(strings.Repeat("wrapping words ", 30)) // several lines
		target := filepath.Join(t.TempDir(), "wrap.pdf")
		if err := document.Write(target); err != nil {
			t.Fatal(err)
		}
		content, _ := os.ReadFile(target)
		ys := lineBaselines(t, contentStream(t, content))
		if len(ys) < 3 {
			t.Fatalf("size %v: %d lines, want several", test.size, len(ys))
		}
		for i := 1; i < len(ys); i++ {
			if gap := ys[i-1] - ys[i]; gap < test.want-0.01 || gap > test.want+0.01 {
				t.Errorf("size %v: lines %d and %d are %.2f apart, want %.2f", test.size, i, i+1, gap, test.want)
			}
		}
	}
}

func TestInlineCodeDoesNotShrinkTheLineHeightOfAHeading(t *testing.T) {
	document := New()
	canvas := document.Canvas()
	canvas.Style("Helvetica", true, false, 20)
	canvas.Text(strings.Repeat("heading words ", 12))
	canvas.Style("Courier", true, false, 17)
	canvas.Text(strings.Repeat("code words ", 12))
	target := filepath.Join(t.TempDir(), "mixed.pdf")
	if err := document.Write(target); err != nil {
		t.Fatal(err)
	}
	content, _ := os.ReadFile(target)
	ys := lineBaselines(t, contentStream(t, content))
	for i := 1; i < len(ys); i++ {
		// fpdf puts the baseline at 0.5*height + 0.3*size, so a smaller run on the same
		// line sits up to 0.3*(20-17) = 0.9 points higher; the line spacing itself is 27.
		gap := ys[i-1] - ys[i]
		if gap > -1 && gap < 1 {
			continue // two runs on the same line
		}
		if gap < 26 || gap > 28 {
			t.Errorf("lines %d and %d are %.2f apart, want about 27 for a size 20 heading", i, i+1, gap)
		}
	}
}

func TestLineHeightSurvivesALineBreakAndANewPage(t *testing.T) {
	for _, reset := range []struct {
		name string
		do   func(render.Canvas)
	}{
		{"line break", func(c render.Canvas) { c.LineBreak(27) }},
		{"new page", func(c render.Canvas) { c.NewPage() }},
	} {
		document := New()
		canvas := document.Canvas()
		canvas.Style("Helvetica", true, false, 20)
		canvas.Text("first")
		reset.do(canvas)
		canvas.Text(strings.Repeat("wrapping words ", 30)) // no new Style call: still size 20
		target := filepath.Join(t.TempDir(), "reset.pdf")
		if err := document.Write(target); err != nil {
			t.Fatal(err)
		}
		content, _ := os.ReadFile(target)
		streams := contentStreams(t, content)
		ys := lineBaselines(t, streams[len(streams)-1])
		if len(ys) < 3 {
			t.Fatalf("%s: %d lines, want several", reset.name, len(ys))
		}
		if gap := ys[len(ys)-2] - ys[len(ys)-1]; gap < 26.99 || gap > 27.01 {
			t.Errorf("%s: lines are %.2f apart, want 27", reset.name, gap)
		}
	}
}

func TestParagraphAfterAHeadingKeepsItsOwnLineHeight(t *testing.T) {
	document := New()
	canvas := document.Canvas()
	canvas.Style("Helvetica", true, false, 20)
	canvas.Text("Heading")
	canvas.LineBreak(27)
	canvas.Style("Helvetica", false, false, 11)
	canvas.Text(strings.Repeat("paragraph words ", 30))
	target := filepath.Join(t.TempDir(), "after.pdf")
	if err := document.Write(target); err != nil {
		t.Fatal(err)
	}
	content, _ := os.ReadFile(target)
	ys := lineBaselines(t, contentStream(t, content))
	for i := 2; i < len(ys); i++ {
		if gap := ys[i-1] - ys[i]; gap < 14.99 || gap > 15.01 {
			t.Errorf("paragraph lines %d and %d are %.2f apart, want 15", i, i+1, gap)
		}
	}
}

func TestHeadingKeepsItsLineHeightWhenTheFirstSpanIsCode(t *testing.T) {
	document := New()
	canvas := document.Canvas()
	canvas.Style("Helvetica", false, false, 11)
	canvas.Text("before")
	canvas.LineBreak(15)
	canvas.Style("Helvetica", true, false, 20) // the heading base style
	canvas.Style("Courier", true, false, 17)   // its first span is inline code
	canvas.Text(strings.Repeat("code words ", 12))
	target := filepath.Join(t.TempDir(), "code.pdf")
	if err := document.Write(target); err != nil {
		t.Fatal(err)
	}
	content, _ := os.ReadFile(target)
	ys := lineBaselines(t, contentStream(t, content))
	if len(ys) < 3 {
		t.Fatalf("%d lines", len(ys))
	}
	if gap := ys[len(ys)-2] - ys[len(ys)-1]; gap < 26.99 || gap > 27.01 {
		t.Errorf("heading lines are %.2f apart, want 27", gap)
	}
}

func TestHeadingPushedToANewPageKeepsItsLineHeight(t *testing.T) {
	document := New()
	canvas := document.Canvas()
	canvas.Style("Helvetica", false, false, 11)
	for document.pdf.GetY() < 841.89-DefaultMargin-10 {
		canvas.Text("filler")
		canvas.LineBreak(15)
	}
	canvas.Style("Helvetica", true, false, 20) // heading base style
	canvas.Bookmark("Heading", 1)              // does not fit: starts a new page
	canvas.Style("Courier", true, false, 17)   // the first span is inline code
	canvas.Text(strings.Repeat("code words ", 12))
	target := filepath.Join(t.TempDir(), "pushed.pdf")
	if err := document.Write(target); err != nil {
		t.Fatal(err)
	}
	content, _ := os.ReadFile(target)
	streams := contentStreams(t, content)
	if len(streams) < 2 {
		t.Fatalf("%d pages, want the heading on a second page", len(streams))
	}
	ys := lineBaselines(t, streams[len(streams)-1])
	if len(ys) < 3 {
		t.Fatalf("%d lines on the last page", len(ys))
	}
	if gap := ys[len(ys)-2] - ys[len(ys)-1]; gap < 26.99 || gap > 27.01 {
		t.Errorf("heading lines are %.2f apart, want 27", gap)
	}
}

func TestDeepNestingKeepsAReadableTextWidth(t *testing.T) {
	const levels = 40
	var list, quote []markdown.Block
	for depth := 0; depth < levels; depth++ {
		list = append(list, markdown.Block{Kind: markdown.ListItem, Depth: depth, Spans: []markdown.Span{{Text: "item " + strings.Repeat("word ", 12)}}})
	}
	for level := 1; level <= levels; level++ {
		quote = append(quote, markdown.Block{Kind: markdown.Paragraph, Quote: level, Spans: []markdown.Span{{Text: "quote " + strings.Repeat("word ", 12)}}})
	}
	code := markdown.Block{Kind: markdown.CodeBlock, Depth: levels, InItem: true, Lines: []string{strings.Repeat("x", 300)}}
	table := markdown.Block{Kind: markdown.Table, Depth: levels, InItem: true, Rows: []markdown.Row{{Header: true, Cells: []markdown.Cell{{Spans: []markdown.Span{{Text: "head"}}}}}, {Cells: []markdown.Cell{{Spans: []markdown.Span{{Text: strings.Repeat("cell ", 10)}}}}}}}
	limit := 595.28 - DefaultMargin - minTextWidth + 4.5 // the widest left edge of text plus the table cell padding
	for name, blocks := range map[string][]markdown.Block{"list": list, "quote": quote, "code in a list": {code}, "table in a list": {table}} {
		document := New()
		if err := render.Draw(blocks, document.Canvas(), render.Options{}); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		target := filepath.Join(t.TempDir(), "deep.pdf")
		if err := document.Write(target); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		content, _ := os.ReadFile(target)
		columns := textColumns(t, contentStream(t, content))
		if len(columns) == 0 {
			t.Fatalf("%s: no text", name)
		}
		for _, x := range columns {
			if x > limit {
				t.Errorf("%s: text starts at x=%.1f, beyond %.1f", name, x, limit)
				break
			}
		}
	}
}

func TestQuoteBarsStayLeftOfTheTextUnderDeepNesting(t *testing.T) {
	var blocks []markdown.Block
	for level := 1; level <= 40; level++ {
		blocks = append(blocks, markdown.Block{Kind: markdown.Paragraph, Quote: level, Spans: []markdown.Span{{Text: "quote text"}}})
	}
	document := New()
	if err := render.Draw(blocks, document.Canvas(), render.Options{}); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "bars.pdf")
	if err := document.Write(target); err != nil {
		t.Fatal(err)
	}
	content, _ := os.ReadFile(target)
	stream := contentStream(t, content)
	bars := regexp.MustCompile(`([\d.]+) [\d.]+ m ([\d.]+) [\d.]+ l S`).FindAllSubmatch(stream, -1)
	if len(bars) == 0 {
		t.Fatal("no quote bars")
	}
	deepest := 0.0
	for _, bar := range bars {
		x, _ := strconv.ParseFloat(string(bar[1]), 64)
		deepest = max(deepest, x)
	}
	// The text of the deepest levels starts at the cap; a bar there would run through it.
	capX := DefaultMargin + New().maxIndent()
	if deepest >= capX {
		t.Errorf("a quote bar at x=%.1f reaches the text column at the cap (x=%.1f)", deepest, capX)
	}
}

func TestIndentingBackIsExactAfterCapping(t *testing.T) {
	document := New()
	canvas := document.Canvas()
	for i := 0; i < 50; i++ {
		canvas.Indent(14)
	}
	for i := 0; i < 50; i++ {
		canvas.Indent(-14)
	}
	canvas.Style("Helvetica", false, false, 11)
	canvas.Text("back at the margin")
	target := filepath.Join(t.TempDir(), "back.pdf")
	if err := document.Write(target); err != nil {
		t.Fatal(err)
	}
	content, _ := os.ReadFile(target)
	if columns := textColumns(t, contentStream(t, content)); len(columns) == 0 || columns[0] > DefaultMargin+3 {
		t.Fatalf("text starts at %v, want the margin %v", columns, DefaultMargin)
	}
}

func TestDiagramScaleKeepsThePrintedSize(t *testing.T) {
	size := func(width, height int, scale float64) (float64, float64) {
		var pngBytes bytes.Buffer
		if err := png.Encode(&pngBytes, image.NewRGBA(image.Rect(0, 0, width, height))); err != nil {
			t.Fatal(err)
		}
		document := New()
		if err := document.Canvas().Diagram(pngBytes.Bytes(), scale); err != nil {
			t.Fatal(err)
		}
		target := filepath.Join(t.TempDir(), "scale.pdf")
		if err := document.Write(target); err != nil {
			t.Fatal(err)
		}
		content, _ := os.ReadFile(target)
		match := regexp.MustCompile(`([\d.]+) 0 0 ([\d.]+) [\d.]+ [\d.]+ cm`).FindSubmatch(contentStream(t, content))
		if match == nil {
			t.Fatal("no image placement")
		}
		w, _ := strconv.ParseFloat(string(match[1]), 64)
		h, _ := strconv.ParseFloat(string(match[2]), 64)
		return w, h
	}
	w1, h1 := size(200, 100, 1)
	w2, h2 := size(400, 200, 2)
	if w1 != w2 || h1 != h2 || w1 != 200 || h1 != 100 {
		t.Fatalf("scale 1 draws %vx%v, scale 2 draws %vx%v; both should be 200x100", w1, h1, w2, h2)
	}
	if w, _ := size(200, 100, 0); w != 200 {
		t.Fatalf("scale 0 draws %v wide, want 200 (treated as 1)", w)
	}
}

// headingPage fills the first page until spare points remain above the bottom
// margin, draws a level 2 heading and a paragraph, and returns the page number
// of the heading and the number of pages.
func headingPage(t *testing.T, spare float64) (int, int) {
	t.Helper()
	document := New()
	canvas := document.Canvas()
	canvas.Style("Helvetica", false, false, 11)
	bottom := 841.89 - DefaultMargin
	for document.pdf.GetY()+15 < bottom-spare {
		canvas.Text("filler")
		canvas.LineBreak(15)
	}
	blocks := []markdown.Block{
		{Kind: markdown.Heading, Level: 2, Spans: []markdown.Span{{Text: "Kept"}}},
		{Kind: markdown.Paragraph, Spans: []markdown.Span{{Text: "body"}}},
	}
	if err := render.Draw(blocks, canvas, render.Options{}); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "keep.pdf")
	if err := document.Write(target); err != nil {
		t.Fatal(err)
	}
	content, _ := os.ReadFile(target)
	streams := contentStreams(t, content)
	for i, stream := range streams {
		if bytes.Contains(stream, []byte("(Kept)")) {
			return i + 1, len(streams)
		}
	}
	t.Fatal("heading not found")
	return 0, 0
}

func TestAHeadingWithoutRoomBelowStartsTheNextPage(t *testing.T) {
	if page, _ := headingPage(t, 40); page != 2 {
		t.Fatalf("heading on page %d with 40 points left, want page 2", page)
	}
	if page, pages := headingPage(t, 120); page != 1 || pages != 1 {
		t.Fatalf("heading on page %d of %d with 120 points left, want page 1 of 1", page, pages)
	}
}

func TestAHeadingAtTheTopOfAPageAddsNoEmptyPage(t *testing.T) {
	document := New()
	blocks := []markdown.Block{{Kind: markdown.Heading, Level: 1, Spans: []markdown.Span{{Text: "Top"}}}}
	if err := render.Draw(blocks, document.Canvas(), render.Options{}); err != nil {
		t.Fatal(err)
	}
	if document.pdf.PageCount() != 1 {
		t.Fatalf("%d pages, want 1", document.pdf.PageCount())
	}
}

func TestAQuoteOpeningWithAHeadingLeavesNoEmptyBar(t *testing.T) {
	document := New()
	canvas := document.Canvas()
	canvas.Style("Helvetica", false, false, 11)
	for document.pdf.GetY()+15 < 841.89-DefaultMargin-40 {
		canvas.Text("filler")
		canvas.LineBreak(15)
	}
	blocks := []markdown.Block{
		{Kind: markdown.Heading, Level: 2, Quote: 1, Spans: []markdown.Span{{Text: "Quoted"}}},
		{Kind: markdown.Paragraph, Quote: 1, Spans: []markdown.Span{{Text: "body"}}},
	}
	if err := render.Draw(blocks, canvas, render.Options{}); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "quote.pdf")
	if err := document.Write(target); err != nil {
		t.Fatal(err)
	}
	content, _ := os.ReadFile(target)
	streams := contentStreams(t, content)
	if len(streams) != 2 || !bytes.Contains(streams[1], []byte("(Quoted)")) {
		t.Fatalf("%d pages; the heading must open page 2", len(streams))
	}
	bar := regexp.MustCompile(`[\d.]+ [\d.]+ m [\d.]+ [\d.]+ l S`)
	if bar.Match(streams[0]) {
		t.Fatalf("page 1 has a quote bar although the quote starts on page 2: %s", bar.Find(streams[0]))
	}
	if !bar.Match(streams[1]) {
		t.Fatal("page 2 has no quote bar")
	}
}
