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
