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
	"regexp"
	"strconv"
	"strings"
	"testing"

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
		t.Fatal("PDF bevat geen afbeelding")
	}
}

func TestDiagramInvalidPNGReturnsError(t *testing.T) {
	document := New()
	if err := document.Canvas().Diagram([]byte("geen PNG")); err == nil {
		t.Fatal("ongeldige PNG gaf geen fout")
	}
}

func TestWritePDF(t *testing.T) {
	path := filepath.Join(t.TempDir(), "alinea.pdf")
	document := New()
	if err := render.Draw([]markdown.Block{{Kind: markdown.Paragraph, Spans: []markdown.Span{{Text: "Een alinea."}}}}, document.Canvas(), render.Options{}); err != nil {
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
		t.Fatalf("ongeldige PDF van %d bytes", len(content))
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
		t.Fatalf("cp1252-byte 0xe9 ontbreekt in % x", stream)
	}
	if bytes.Contains(stream, []byte{0xc3, 0xa9}) {
		t.Fatalf("UTF-8-bytes gevonden in % x", stream)
	}
}

func TestLongListItemStaysIndented(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lijst.pdf")
	document := New()
	blocks := []markdown.Block{
		{Kind: markdown.ListItem, Depth: 1, Spans: []markdown.Span{{Text: strings.Repeat("een lang lijstitem ", 40)}}},
		{Kind: markdown.Paragraph, Spans: []markdown.Span{{Text: "einde"}}},
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
		t.Fatalf("te weinig tekstregels om terugloop te beoordelen: %q", stream)
	}
	// The first line is the bullet, followed by the item text; all
	// subsequent lines should hang below that text column, not below the bullet.
	textColumn := columns[1]
	for i, column := range columns[2 : len(columns)-1] {
		if column != textColumn {
			t.Fatalf("vervolgregel %d staat op %.2f, wil %.2f: %q", i+1, column, textColumn, stream)
		}
	}
	if final := columns[len(columns)-1]; final >= columns[0] {
		t.Fatalf("linkermarge is niet hersteld: alinea op %.2f, bol stond op %.2f", final, columns[0])
	}
}

func TestTableTextIsInContentStream(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tabel.pdf")
	document := New()
	rows := []markdown.Row{
		{Header: true, Cells: []markdown.Cell{
			{Spans: []markdown.Span{{Text: "Naam"}}},
			{Spans: []markdown.Span{{Text: "Waarde"}}},
		}},
		{Cells: []markdown.Cell{
			{Spans: []markdown.Span{{Text: "een"}}},
			{Spans: []markdown.Span{{Text: "1"}}},
		}},
		{Cells: []markdown.Cell{
			{Spans: []markdown.Span{{Text: "twee"}}},
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
	if !bytes.Contains(stream, []byte("Naam")) {
		t.Fatalf("kopregel ontbreekt in %q", stream)
	}
	if !bytes.Contains(stream, []byte("twee")) {
		t.Fatalf("laatste datarij ontbreekt in %q", stream)
	}
}

func TestTableTooWideWritesWithoutError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "breed.pdf")
	document := New()
	rows := []markdown.Row{
		{Header: true, Cells: []markdown.Cell{{Spans: []markdown.Span{{Text: strings.Repeat("kop ", 200)}}}}},
		{Cells: []markdown.Cell{{Spans: []markdown.Span{{Text: strings.Repeat("breed ", 200)}}}}},
	}
	document.Canvas().Table(rows)
	if err := document.Canvas().Err(); err != nil {
		t.Fatalf("Fout() na tekenen: %v", err)
	}
	if err := document.Write(path); err != nil {
		t.Fatal(err)
	}
	if err := document.Canvas().Err(); err != nil {
		t.Fatalf("Fout() na schrijven: %v", err)
	}
}

func TestTableRepeatsHeaderAcrossPages(t *testing.T) {
	path := filepath.Join(t.TempDir(), "paginas.pdf")
	document := New()
	rows := []markdown.Row{
		{Header: true, Cells: []markdown.Cell{{Spans: []markdown.Span{{Text: "Koptekst"}}}}},
	}
	for i := 0; i < 60; i++ {
		rows = append(rows, markdown.Row{Cells: []markdown.Cell{{Spans: []markdown.Span{{Text: fmt.Sprintf("rij%d", i)}}}}})
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
		t.Fatalf("PDF telt %d pagina's, wil meer dan één", count)
	}
	headers := 0
	for _, stream := range contentStreams(t, content) {
		headers += bytes.Count(stream, []byte("Koptekst"))
	}
	if headers <= 1 {
		t.Fatalf("kopregel komt %d keer voor, wil vaker dan één", headers)
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
		t.Fatal("geen inhoudsstromen gevonden")
	}
	return out
}

// pageCount counts the page objects in a PDF.
func pageCount(pdf []byte) int {
	text := string(pdf)
	return strings.Count(text, "/Type /Page") - strings.Count(text, "/Type /Pages")
}

func TestRenderREADMEToPDF(t *testing.T) {
	source, err := os.ReadFile(filepath.Join("..", "..", "testdata", "README.md"))
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
		t.Fatalf("uitvoer = %v, %v", info, err)
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
		t.Fatalf("diagram beslaat %d pagina's, wil er één", pageCount(content))
	}
}
