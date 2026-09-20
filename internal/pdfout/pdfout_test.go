package pdfout

import (
	"bytes"
	"compress/zlib"
	"fmt"
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

func TestSchrijfPDF(t *testing.T) {
	pad := filepath.Join(t.TempDir(), "alinea.pdf")
	document := Nieuw()
	if err := render.Teken([]markdown.Blok{{Soort: markdown.Alinea, Stukken: []markdown.Stuk{{Tekst: "Een alinea."}}}}, document.Vel()); err != nil {
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
	blokken := []markdown.Blok{{Soort: markdown.Alinea, Stukken: []markdown.Stuk{{Tekst: "café —"}}}}
	if err := render.Teken(blokken, document.Vel()); err != nil {
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
	blokken := []markdown.Blok{
		{Soort: markdown.Lijstitem, Diepte: 1, Stukken: []markdown.Stuk{{Tekst: strings.Repeat("een lang lijstitem ", 40)}}},
		{Soort: markdown.Alinea, Stukken: []markdown.Stuk{{Tekst: "einde"}}},
	}
	if err := render.Teken(blokken, document.Vel()); err != nil {
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
	rijen := []markdown.Rij{
		{Kop: true, Cellen: []markdown.Cel{
			{Stukken: []markdown.Stuk{{Tekst: "Naam"}}},
			{Stukken: []markdown.Stuk{{Tekst: "Waarde"}}},
		}},
		{Cellen: []markdown.Cel{
			{Stukken: []markdown.Stuk{{Tekst: "een"}}},
			{Stukken: []markdown.Stuk{{Tekst: "1"}}},
		}},
		{Cellen: []markdown.Cel{
			{Stukken: []markdown.Stuk{{Tekst: "twee"}}},
			{Stukken: []markdown.Stuk{{Tekst: "2"}}},
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
	rijen := []markdown.Rij{
		{Kop: true, Cellen: []markdown.Cel{{Stukken: []markdown.Stuk{{Tekst: strings.Repeat("kop ", 200)}}}}},
		{Cellen: []markdown.Cel{{Stukken: []markdown.Stuk{{Tekst: strings.Repeat("breed ", 200)}}}}},
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
	rijen := []markdown.Rij{
		{Kop: true, Cellen: []markdown.Cel{{Stukken: []markdown.Stuk{{Tekst: "Koptekst"}}}}},
	}
	for i := 0; i < 60; i++ {
		rijen = append(rijen, markdown.Rij{Cellen: []markdown.Cel{{Stukken: []markdown.Stuk{{Tekst: fmt.Sprintf("rij%d", i)}}}}})
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
	blokken, err := markdown.Ontleed(bron)
	if err != nil {
		t.Fatal(err)
	}
	pad := filepath.Join(t.TempDir(), "readme.pdf")
	document := Nieuw()
	if err := render.Teken(blokken, document.Vel()); err != nil {
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
