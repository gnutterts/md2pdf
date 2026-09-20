package markdown

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseElements(t *testing.T) {
	tests := []struct {
		name, source string
		check        func(*testing.T, []Block)
	}{
		{"kop 1", "# Een", func(t *testing.T, b []Block) {
			if b[0].Level != 1 {
				t.Fatal(b)
			}
		}},
		{"kop 2", "## Twee", func(t *testing.T, b []Block) {
			if b[0].Level != 2 {
				t.Fatal(b)
			}
		}},
		{"kop 3", "### Drie", func(t *testing.T, b []Block) {
			if b[0].Level != 3 {
				t.Fatal(b)
			}
		}},
		{"kop 4", "#### Vier", func(t *testing.T, b []Block) {
			if b[0].Level != 4 {
				t.Fatal(b)
			}
		}},
		{"kop 5", "##### Vijf", func(t *testing.T, b []Block) {
			if b[0].Level != 5 {
				t.Fatal(b)
			}
		}},
		{"kop 6", "###### Zes", func(t *testing.T, b []Block) {
			if b[0].Level != 6 {
				t.Fatal(b)
			}
		}},
		{"vet", "**sterk**", func(t *testing.T, b []Block) {
			if !b[0].Spans[0].Bold {
				t.Fatal(b)
			}
		}},
		{"cursief", "*schuin*", func(t *testing.T, b []Block) {
			if !b[0].Spans[0].Italic {
				t.Fatal(b)
			}
		}},
		{"vet en cursief", "***sterk schuin***", func(t *testing.T, b []Block) {
			span := b[0].Spans[0]
			if !span.Bold || !span.Italic {
				t.Fatal(b)
			}
		}},
		{"inline code", "`x()`", func(t *testing.T, b []Block) {
			if !b[0].Spans[0].Code {
				t.Fatal(b)
			}
		}},
		{"link", "[site](https://example.org)", func(t *testing.T, b []Block) {
			if b[0].Spans[0].URL != "https://example.org" {
				t.Fatal(b)
			}
		}},
		{"geneste lijst", "- buiten\n  - binnen", func(t *testing.T, b []Block) {
			if len(b) != 2 || b[0].Depth != 0 || b[1].Depth != 1 {
				t.Fatal(b)
			}
		}},
		{"genummerde lijst", "3. drie", func(t *testing.T, b []Block) {
			if !b[0].Ordered || b[0].Number != 3 {
				t.Fatal(b)
			}
		}},
		{"genummerde lijst vanaf nul", "0. nul", func(t *testing.T, b []Block) {
			if !b[0].Ordered || b[0].Number != 0 {
				t.Fatal(b)
			}
		}},
		{"lijstitem met twee alineas", "- eerste alinea\n\n  tweede alinea", func(t *testing.T, b []Block) {
			if len(b) != 2 || b[0].Depth != 0 || b[1].Depth != 0 {
				t.Fatal(b)
			}
		}},
		{"codeblok met taal", "```go\nfmt.Println()\n```", func(t *testing.T, b []Block) {
			if b[0].Kind != CodeBlock || b[0].Language != "go" {
				t.Fatal(b)
			}
		}},
		{"mermaid", "```mermaid\ngraph TD\n```", func(t *testing.T, b []Block) {
			if b[0].Language != "mermaid" {
				t.Fatal(b)
			}
		}},
		{"citaat", "> woorden", func(t *testing.T, b []Block) {
			if b[0].Kind != Quote {
				t.Fatal(b)
			}
		}},
		{"breuk", "---", func(t *testing.T, b []Block) {
			if b[0].Kind != Rule {
				t.Fatal(b)
			}
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			blocks, err := Parse([]byte(test.source))
			if err != nil {
				t.Fatal(err)
			}
			test.check(t, blocks)
		})
	}
}

func TestParseREADME(t *testing.T) {
	source, err := os.ReadFile(filepath.Join("..", "..", "testdata", "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	blocks, err := Parse(source)
	if err != nil || len(blocks) == 0 {
		t.Fatalf("Ontleed = %d blokken, %v", len(blocks), err)
	}
}

func TestParseTable(t *testing.T) {
	source := []byte("| Naam | Waarde |\n|------|--------|\n| een  | 1      |\n| twee | 2      |\n")
	blocks, err := Parse(source)
	if err != nil {
		t.Fatal(err)
	}
	if len(blocks) != 1 || blocks[0].Kind != Table {
		t.Fatalf("Ontleed leverde geen tabelblok: %v", blocks)
	}
	rows := blocks[0].Rows
	if len(rows) != 3 {
		t.Fatalf("tabel heeft %d rijen, wil 3: %v", len(rows), rows)
	}
	if !rows[0].Header || rows[1].Header || rows[2].Header {
		t.Fatalf("alleen de eerste rij hoort Kop te zijn: %v", rows)
	}
	want := [][]string{{"Naam", "Waarde"}, {"een", "1"}, {"twee", "2"}}
	for i, row := range rows {
		if len(row.Cells) != 2 {
			t.Fatalf("rij %d heeft %d cellen, wil 2: %v", i, len(row.Cells), row)
		}
		for column, cell := range row.Cells {
			if len(cell.Spans) != 1 || cell.Spans[0].Text != want[i][column] {
				t.Fatalf("cel %d,%d is %v, wil %q", i, column, cell.Spans, want[i][column])
			}
		}
	}
}

func TestParseTableAlignment(t *testing.T) {
	source := []byte("| L | C | R |\n|---|:--:|---:|\n| a | b | c |\n")
	blocks, err := Parse(source)
	if err != nil {
		t.Fatal(err)
	}
	rows := blocks[0].Rows
	if len(rows) == 0 || len(rows[0].Cells) != 3 {
		t.Fatalf("verwachte drie uitgelijnde cellen: %v", rows)
	}
	want := []Alignment{AlignLeft, AlignCenter, AlignRight}
	for column, alignment := range want {
		if rows[0].Cells[column].Alignment != alignment {
			t.Fatalf("kolom %d heeft uitlijning %v, wil %v", column, rows[0].Cells[column].Alignment, alignment)
		}
	}
}
