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
	pngAfbeelding := image.NewRGBA(image.Rect(0, 0, 40, 20))
	pngAfbeelding.Set(0, 0, color.Black)
	if err := png.Encode(&pngBytes, pngAfbeelding); err != nil {
		t.Fatal(err)
	}
	document := Nieuw()
	if err := document.Vel().Diagram(pngBytes.Bytes()); err != nil {
		t.Fatal(err)
	}
	pad := filepath.Join(t.TempDir(), "diagram.pdf")
	if err := document.Schrijf(pad); err != nil {
		t.Fatal(err)
	}
	inhoud, err := os.ReadFile(pad)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(inhoud, []byte("/Subtype /Image")) {
		t.Fatal("PDF bevat geen afbeelding")
	}
}

func TestDiagramOngeldigePNGGeeftFout(t *testing.T) {
	document := Nieuw()
	if err := document.Vel().Diagram([]byte("geen PNG")); err == nil {
		t.Fatal("ongeldige PNG gaf geen fout")
	}
}

func TestSchrijfPDF(t *testing.T) {
	pad := filepath.Join(t.TempDir(), "alinea.pdf")
	document := Nieuw()
	if err := render.Teken([]markdown.Block{{Kind: markdown.Paragraph, Spans: []markdown.Span{{Text: "Een alinea."}}}}, document.Vel(), render.Opties{}); err != nil {
		t.Fatal(err)
	}
	if err := document.Schrijf(pad); err != nil {
		t.Fatal(err)
	}
	inhoud, err := os.ReadFile(pad)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(inhoud, []byte("%PDF-")) || !bytes.HasSuffix(bytes.TrimSpace(inhoud), []byte("%%EOF")) || len(inhoud) <= 500 {
		t.Fatalf("ongeldige PDF van %d bytes", len(inhoud))
	}
}

func TestCP1252StaatInInhoudsstroom(t *testing.T) {
	pad := filepath.Join(t.TempDir(), "tekens.pdf")
	document := Nieuw()
	blokken := []markdown.Block{{Kind: markdown.Paragraph, Spans: []markdown.Span{{Text: "café —"}}}}
	if err := render.Teken(blokken, document.Vel(), render.Opties{}); err != nil {
		t.Fatal(err)
	}
	if err := document.Schrijf(pad); err != nil {
		t.Fatal(err)
	}
	inhoud, err := os.ReadFile(pad)
	if err != nil {
		t.Fatal(err)
	}
	stroom := inhoudsstroom(t, inhoud)
	if !bytes.Contains(stroom, []byte{0xe9}) {
		t.Fatalf("cp1252-byte 0xe9 ontbreekt in % x", stroom)
	}
	if bytes.Contains(stroom, []byte{0xc3, 0xa9}) {
		t.Fatalf("UTF-8-bytes gevonden in % x", stroom)
	}
}

func TestLangLijstitemBlijftIngesprongen(t *testing.T) {
	pad := filepath.Join(t.TempDir(), "lijst.pdf")
	document := Nieuw()
	blokken := []markdown.Block{
		{Kind: markdown.ListItem, Depth: 1, Spans: []markdown.Span{{Text: strings.Repeat("een lang lijstitem ", 40)}}},
		{Kind: markdown.Paragraph, Spans: []markdown.Span{{Text: "einde"}}},
	}
	if err := render.Teken(blokken, document.Vel(), render.Opties{}); err != nil {
		t.Fatal(err)
	}
	if err := document.Schrijf(pad); err != nil {
		t.Fatal(err)
	}
	inhoud, err := os.ReadFile(pad)
	if err != nil {
		t.Fatal(err)
	}
	stroom := inhoudsstroom(t, inhoud)
	kolommen := tekstkolommen(t, stroom)
	if len(kolommen) < 4 {
		t.Fatalf("te weinig tekstregels om terugloop te beoordelen: %q", stroom)
	}
	// Eerste regel is de opsommingsbol, daarna de tekst van het item; alle
	// vervolgregels horen onder die tekstkolom te hangen, niet onder de bol.
	tekstkolom := kolommen[1]
	for i, kolom := range kolommen[2 : len(kolommen)-1] {
		if kolom != tekstkolom {
			t.Fatalf("vervolgregel %d staat op %.2f, wil %.2f: %q", i+1, kolom, tekstkolom, stroom)
		}
	}
	if slot := kolommen[len(kolommen)-1]; slot >= kolommen[0] {
		t.Fatalf("linkermarge is niet hersteld: alinea op %.2f, bol stond op %.2f", slot, kolommen[0])
	}
}

func TestTabelTekstStaatInInhoudsstroom(t *testing.T) {
	pad := filepath.Join(t.TempDir(), "tabel.pdf")
	document := Nieuw()
	rijen := []markdown.Row{
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
	document.Vel().Tabel(rijen)
	if err := document.Schrijf(pad); err != nil {
		t.Fatal(err)
	}
	inhoud, err := os.ReadFile(pad)
	if err != nil {
		t.Fatal(err)
	}
	stroom := inhoudsstroom(t, inhoud)
	if !bytes.Contains(stroom, []byte("Naam")) {
		t.Fatalf("kopregel ontbreekt in %q", stroom)
	}
	if !bytes.Contains(stroom, []byte("twee")) {
		t.Fatalf("laatste datarij ontbreekt in %q", stroom)
	}
}

func TestTabelTeBreedSchrijftZonderFout(t *testing.T) {
	pad := filepath.Join(t.TempDir(), "breed.pdf")
	document := Nieuw()
	rijen := []markdown.Row{
		{Header: true, Cells: []markdown.Cell{{Spans: []markdown.Span{{Text: strings.Repeat("kop ", 200)}}}}},
		{Cells: []markdown.Cell{{Spans: []markdown.Span{{Text: strings.Repeat("breed ", 200)}}}}},
	}
	document.Vel().Tabel(rijen)
	if fout := document.Vel().Fout(); fout != nil {
		t.Fatalf("Fout() na tekenen: %v", fout)
	}
	if err := document.Schrijf(pad); err != nil {
		t.Fatal(err)
	}
	if fout := document.Vel().Fout(); fout != nil {
		t.Fatalf("Fout() na schrijven: %v", fout)
	}
}

func TestTabelOverPaginasHerhaaltKopregel(t *testing.T) {
	pad := filepath.Join(t.TempDir(), "paginas.pdf")
	document := Nieuw()
	rijen := []markdown.Row{
		{Header: true, Cells: []markdown.Cell{{Spans: []markdown.Span{{Text: "Koptekst"}}}}},
	}
	for i := 0; i < 60; i++ {
		rijen = append(rijen, markdown.Row{Cells: []markdown.Cell{{Spans: []markdown.Span{{Text: fmt.Sprintf("rij%d", i)}}}}})
	}
	document.Vel().Tabel(rijen)
	if err := document.Schrijf(pad); err != nil {
		t.Fatal(err)
	}
	inhoud, err := os.ReadFile(pad)
	if err != nil {
		t.Fatal(err)
	}
	if aantal := paginaAantal(inhoud); aantal <= 1 {
		t.Fatalf("PDF telt %d pagina's, wil meer dan één", aantal)
	}
	kopteksten := 0
	for _, stroom := range inhoudsstromen(t, inhoud) {
		kopteksten += bytes.Count(stroom, []byte("Koptekst"))
	}
	if kopteksten <= 1 {
		t.Fatalf("kopregel komt %d keer voor, wil vaker dan één", kopteksten)
	}
}

// tekstkolommen leest de x-positie van elke tekstplaatsing uit een inhoudsstroom.
func tekstkolommen(t *testing.T, stroom []byte) []float64 {
	t.Helper()
	var kolommen []float64
	for _, treffer := range regexp.MustCompile(`BT (\d+\.\d+) \d+\.\d+ Td`).FindAllSubmatch(stroom, -1) {
		x, err := strconv.ParseFloat(string(treffer[1]), 64)
		if err != nil {
			t.Fatal(err)
		}
		kolommen = append(kolommen, x)
	}
	return kolommen
}

// inhoudsstroom pakt de eerste inhoudsstroom van een PDF uit.
func inhoudsstroom(t *testing.T, pdf []byte) []byte {
	t.Helper()
	stromen := inhoudsstromen(t, pdf)
	return stromen[0]
}

// inhoudsstromen pakt alle zlib-inhoudsstromen van een PDF uit.
func inhoudsstromen(t *testing.T, pdf []byte) [][]byte {
	t.Helper()
	var uit [][]byte
	rest := pdf
	for {
		begin := bytes.Index(rest, []byte("stream\n"))
		if begin < 0 {
			break
		}
		begin += len("stream\n")
		einde := bytes.Index(rest[begin:], []byte("\nendstream"))
		if einde < 0 {
			break
		}
		lezer, err := zlib.NewReader(bytes.NewReader(rest[begin : begin+einde]))
		if err != nil {
			t.Fatal(err)
		}
		stroom, err := io.ReadAll(lezer)
		if err != nil {
			t.Fatal(err)
		}
		if err := lezer.Close(); err != nil {
			t.Fatal(err)
		}
		uit = append(uit, stroom)
		rest = rest[begin+einde+len("\nendstream"):]
	}
	if len(uit) == 0 {
		t.Fatal("geen inhoudsstromen gevonden")
	}
	return uit
}

// paginaAantal telt het aantal pagina-objecten in een PDF.
func paginaAantal(pdf []byte) int {
	tekst := string(pdf)
	return strings.Count(tekst, "/Type /Page") - strings.Count(tekst, "/Type /Pages")
}

func TestRenderREADMENaarPDF(t *testing.T) {
	bron, err := os.ReadFile(filepath.Join("..", "..", "testdata", "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	blokken, err := markdown.Parse(bron)
	if err != nil {
		t.Fatal(err)
	}
	pad := filepath.Join(t.TempDir(), "readme.pdf")
	document := Nieuw()
	if err := render.Teken(blokken, document.Vel(), render.Opties{}); err != nil {
		t.Fatal(err)
	}
	if err := document.Schrijf(pad); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(pad)
	if err != nil || info.Size() == 0 {
		t.Fatalf("uitvoer = %v, %v", info, err)
	}
}

func TestDiagramHogerDanEenPaginaWordtGeschaald(t *testing.T) {
	document := Nieuw()
	// Een smal en zeer hoog plaatje: op de volle tekstbreedte zou het ruim
	// hoger worden dan een pagina.
	smalEnHoog := image.NewRGBA(image.Rect(0, 0, 100, 900))
	var pngBytes bytes.Buffer
	if err := png.Encode(&pngBytes, smalEnHoog); err != nil {
		t.Fatal(err)
	}
	if err := document.Vel().Diagram(pngBytes.Bytes()); err != nil {
		t.Fatal(err)
	}
	pad := filepath.Join(t.TempDir(), "diagram.pdf")
	if err := document.Schrijf(pad); err != nil {
		t.Fatal(err)
	}
	inhoud, err := os.ReadFile(pad)
	if err != nil {
		t.Fatal(err)
	}
	if paginaAantal(inhoud) != 1 {
		t.Fatalf("diagram beslaat %d pagina's, wil er één", paginaAantal(inhoud))
	}
}
