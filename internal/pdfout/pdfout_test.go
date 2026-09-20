package pdfout

import (
	"bytes"
	"compress/zlib"
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

func inhoudsstroom(t *testing.T, pdf []byte) []byte {
	t.Helper()
	begin := bytes.Index(pdf, []byte("stream\n"))
	if begin < 0 {
		t.Fatal("inhoudsstroom ontbreekt")
	}
	begin += len("stream\n")
	einde := bytes.Index(pdf[begin:], []byte("\nendstream"))
	if einde < 0 {
		t.Fatal("einde van inhoudsstroom ontbreekt")
	}
	lezer, err := zlib.NewReader(bytes.NewReader(pdf[begin : begin+einde]))
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
	return stroom
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
