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
		{"heading 1", "# One", func(t *testing.T, b []Block) {
			if b[0].Level != 1 {
				t.Fatal(b)
			}
		}},
		{"heading 2", "## Two", func(t *testing.T, b []Block) {
			if b[0].Level != 2 {
				t.Fatal(b)
			}
		}},
		{"heading 3", "### Three", func(t *testing.T, b []Block) {
			if b[0].Level != 3 {
				t.Fatal(b)
			}
		}},
		{"heading 4", "#### Four", func(t *testing.T, b []Block) {
			if b[0].Level != 4 {
				t.Fatal(b)
			}
		}},
		{"heading 5", "##### Five", func(t *testing.T, b []Block) {
			if b[0].Level != 5 {
				t.Fatal(b)
			}
		}},
		{"heading 6", "###### Six", func(t *testing.T, b []Block) {
			if b[0].Level != 6 {
				t.Fatal(b)
			}
		}},
		{"bold", "**sterk**", func(t *testing.T, b []Block) {
			if !b[0].Spans[0].Bold {
				t.Fatal(b)
			}
		}},
		{"italic", "*italic*", func(t *testing.T, b []Block) {
			if !b[0].Spans[0].Italic {
				t.Fatal(b)
			}
		}},
		{"bold and italic", "***sterk italic***", func(t *testing.T, b []Block) {
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
		{"nested list", "- outside\n  - inside", func(t *testing.T, b []Block) {
			if len(b) != 2 || b[0].Depth != 0 || b[1].Depth != 1 {
				t.Fatal(b)
			}
		}},
		{"numbered list", "3. three", func(t *testing.T, b []Block) {
			if !b[0].Ordered || b[0].Number != 3 {
				t.Fatal(b)
			}
		}},
		{"numbered list from zero", "0. nul", func(t *testing.T, b []Block) {
			if !b[0].Ordered || b[0].Number != 0 {
				t.Fatal(b)
			}
		}},
		{"list item with two paragraphs", "- first paragraph\n\n  second paragraph", func(t *testing.T, b []Block) {
			if len(b) != 2 || b[0].Depth != 0 || b[1].Depth != 0 {
				t.Fatal(b)
			}
		}},
		{"code block with language", "```go\nfmt.Println()\n```", func(t *testing.T, b []Block) {
			if b[0].Kind != CodeBlock || b[0].Language != "go" {
				t.Fatal(b)
			}
		}},
		{"mermaid", "```mermaid\ngraph TD\n```", func(t *testing.T, b []Block) {
			if b[0].Language != "mermaid" {
				t.Fatal(b)
			}
		}},
		{"quote", "> woorden", func(t *testing.T, b []Block) {
			if b[0].Kind != Quote {
				t.Fatal(b)
			}
		}},
		{"rule", "---", func(t *testing.T, b []Block) {
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
		t.Fatalf("Parse = %d blokken, %v", len(blocks), err)
	}
}

func TestParseTable(t *testing.T) {
	source := []byte("| Name | Value |\n|------|--------|\n| one  | 1      |\n| two  | 2      |\n")
	blocks, err := Parse(source)
	if err != nil {
		t.Fatal(err)
	}
	if len(blocks) != 1 || blocks[0].Kind != Table {
		t.Fatalf("Parse returned no table block: %v", blocks)
	}
	rows := blocks[0].Rows
	if len(rows) != 3 {
		t.Fatalf("table heeft %d rows, want 3: %v", len(rows), rows)
	}
	if !rows[0].Header || rows[1].Header || rows[2].Header {
		t.Fatalf("only the first row should be a header: %v", rows)
	}
	want := [][]string{{"Name", "Value"}, {"one", "1"}, {"two", "2"}}
	for i, row := range rows {
		if len(row.Cells) != 2 {
			t.Fatalf("row %d has %d cells, want 2: %v", i, len(row.Cells), row)
		}
		for column, celll := range row.Cells {
			if len(celll.Spans) != 1 || celll.Spans[0].Text != want[i][column] {
				t.Fatalf("cell %d,%d is %v, want %q", i, column, celll.Spans, want[i][column])
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
		t.Fatalf("expected three aligned cells: %v", rows)
	}
	want := []Alignment{AlignLeft, AlignCenter, AlignRight}
	for column, alignment := range want {
		if rows[0].Cells[column].Alignment != alignment {
			t.Fatalf("column %d has alignment %v, want %v", column, rows[0].Cells[column].Alignment, alignment)
		}
	}
}
