package markdown

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOntleedElementen(t *testing.T) {
	tests := []struct {
		naam, bron string
		controle   func(*testing.T, []Blok)
	}{
		{"kop 1", "# Een", func(t *testing.T, b []Blok) {
			if b[0].Niveau != 1 {
				t.Fatal(b)
			}
		}},
		{"kop 2", "## Twee", func(t *testing.T, b []Blok) {
			if b[0].Niveau != 2 {
				t.Fatal(b)
			}
		}},
		{"kop 3", "### Drie", func(t *testing.T, b []Blok) {
			if b[0].Niveau != 3 {
				t.Fatal(b)
			}
		}},
		{"kop 4", "#### Vier", func(t *testing.T, b []Blok) {
			if b[0].Niveau != 4 {
				t.Fatal(b)
			}
		}},
		{"kop 5", "##### Vijf", func(t *testing.T, b []Blok) {
			if b[0].Niveau != 5 {
				t.Fatal(b)
			}
		}},
		{"kop 6", "###### Zes", func(t *testing.T, b []Blok) {
			if b[0].Niveau != 6 {
				t.Fatal(b)
			}
		}},
		{"vet", "**sterk**", func(t *testing.T, b []Blok) {
			if !b[0].Stukken[0].Vet {
				t.Fatal(b)
			}
		}},
		{"cursief", "*schuin*", func(t *testing.T, b []Blok) {
			if !b[0].Stukken[0].Cursief {
				t.Fatal(b)
			}
		}},
		{"vet en cursief", "***sterk schuin***", func(t *testing.T, b []Blok) {
			stuk := b[0].Stukken[0]
			if !stuk.Vet || !stuk.Cursief {
				t.Fatal(b)
			}
		}},
		{"inline code", "`x()`", func(t *testing.T, b []Blok) {
			if !b[0].Stukken[0].Code {
				t.Fatal(b)
			}
		}},
		{"link", "[site](https://example.org)", func(t *testing.T, b []Blok) {
			if b[0].Stukken[0].URL != "https://example.org" {
				t.Fatal(b)
			}
		}},
		{"geneste lijst", "- buiten\n  - binnen", func(t *testing.T, b []Blok) {
			if len(b) != 2 || b[0].Diepte != 0 || b[1].Diepte != 1 {
				t.Fatal(b)
			}
		}},
		{"genummerde lijst", "3. drie", func(t *testing.T, b []Blok) {
			if !b[0].Genummerd || b[0].Nummer != 3 {
				t.Fatal(b)
			}
		}},
		{"genummerde lijst vanaf nul", "0. nul", func(t *testing.T, b []Blok) {
			if !b[0].Genummerd || b[0].Nummer != 0 {
				t.Fatal(b)
			}
		}},
		{"lijstitem met twee alineas", "- eerste alinea\n\n  tweede alinea", func(t *testing.T, b []Blok) {
			if len(b) != 2 || b[0].Diepte != 0 || b[1].Diepte != 0 {
				t.Fatal(b)
			}
		}},
		{"codeblok met taal", "```go\nfmt.Println()\n```", func(t *testing.T, b []Blok) {
			if b[0].Soort != Codeblok || b[0].Taal != "go" {
				t.Fatal(b)
			}
		}},
		{"mermaid", "```mermaid\ngraph TD\n```", func(t *testing.T, b []Blok) {
			if b[0].Taal != "mermaid" {
				t.Fatal(b)
			}
		}},
		{"citaat", "> woorden", func(t *testing.T, b []Blok) {
			if b[0].Soort != Citaat {
				t.Fatal(b)
			}
		}},
		{"breuk", "---", func(t *testing.T, b []Blok) {
			if b[0].Soort != Streep {
				t.Fatal(b)
			}
		}},
	}
	for _, test := range tests {
		t.Run(test.naam, func(t *testing.T) {
			blokken, err := Ontleed([]byte(test.bron))
			if err != nil {
				t.Fatal(err)
			}
			test.controle(t, blokken)
		})
	}
}

func TestOntleedREADME(t *testing.T) {
	bron, err := os.ReadFile(filepath.Join("..", "..", "testdata", "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	blokken, err := Ontleed(bron)
	if err != nil || len(blokken) == 0 {
		t.Fatalf("Ontleed = %d blokken, %v", len(blokken), err)
	}
}

func TestOntleedTabel(t *testing.T) {
	bron := []byte("| Naam | Waarde |\n|------|--------|\n| een  | 1      |\n| twee | 2      |\n")
	blokken, err := Ontleed(bron)
	if err != nil {
		t.Fatal(err)
	}
	if len(blokken) != 1 || blokken[0].Soort != Tabel {
		t.Fatalf("Ontleed leverde geen tabelblok: %v", blokken)
	}
	rijen := blokken[0].Rijen
	if len(rijen) != 3 {
		t.Fatalf("tabel heeft %d rijen, wil 3: %v", len(rijen), rijen)
	}
	if !rijen[0].Kop || rijen[1].Kop || rijen[2].Kop {
		t.Fatalf("alleen de eerste rij hoort Kop te zijn: %v", rijen)
	}
	wil := [][]string{{"Naam", "Waarde"}, {"een", "1"}, {"twee", "2"}}
	for i, rij := range rijen {
		if len(rij.Cellen) != 2 {
			t.Fatalf("rij %d heeft %d cellen, wil 2: %v", i, len(rij.Cellen), rij)
		}
		for kolom, cel := range rij.Cellen {
			if len(cel.Stukken) != 1 || cel.Stukken[0].Tekst != wil[i][kolom] {
				t.Fatalf("cel %d,%d is %v, wil %q", i, kolom, cel.Stukken, wil[i][kolom])
			}
		}
	}
}

func TestOntleedTabelUitlijning(t *testing.T) {
	bron := []byte("| L | C | R |\n|---|:--:|---:|\n| a | b | c |\n")
	blokken, err := Ontleed(bron)
	if err != nil {
		t.Fatal(err)
	}
	rijen := blokken[0].Rijen
	if len(rijen) == 0 || len(rijen[0].Cellen) != 3 {
		t.Fatalf("verwachte drie uitgelijnde cellen: %v", rijen)
	}
	wil := []Uitlijning{UitlijnLinks, UitlijnMidden, UitlijnRechts}
	for kolom, uitlijning := range wil {
		if rijen[0].Cellen[kolom].Uitlijning != uitlijning {
			t.Fatalf("kolom %d heeft uitlijning %v, wil %v", kolom, rijen[0].Cellen[kolom].Uitlijning, uitlijning)
		}
	}
}
