// SPDX-License-Identifier: MIT

package markdown

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
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
	source, err := os.ReadFile(filepath.Join("..", "..", "testdata", "sample.md"))
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

func TestParseAutoLinks(t *testing.T) {
	tests := []struct {
		name, source, text, url string
	}{
		{"angle", "<https://a.example>", "https://a.example", "https://a.example"},
		{"www", "www.b.example", "www.b.example", "http://www.b.example"},
		{"email", "c@d.example", "c@d.example", "mailto:c@d.example"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			blocks, err := Parse([]byte(test.source))
			if err != nil {
				t.Fatal(err)
			}
			if len(blocks) != 1 || len(blocks[0].Spans) != 1 {
				t.Fatalf("Parse returned %v, want one span", blocks)
			}
			span := blocks[0].Spans[0]
			if span.Text != test.text || span.URL != test.url {
				t.Fatalf("span = %v, want text %q and url %q", span, test.text, test.url)
			}
		})
	}
}

func TestParseAutoLinkInTable(t *testing.T) {
	source := []byte("| Link |\n|---|\n| <https://a.example> |\n")
	blocks, err := Parse(source)
	if err != nil {
		t.Fatal(err)
	}
	if len(blocks) != 1 || blocks[0].Kind != Table || len(blocks[0].Rows) != 2 {
		t.Fatalf("Parse returned no table: %v", blocks)
	}
	cell := blocks[0].Rows[1].Cells[0]
	if len(cell.Spans) != 1 {
		t.Fatalf("cell spans = %v, want one span", cell.Spans)
	}
	span := cell.Spans[0]
	if span.Text != "https://a.example" || span.URL != "https://a.example" {
		t.Fatalf("span = %v, want autolink text and url", span)
	}
}

func TestParseEscapes(t *testing.T) {
	tests := []struct {
		name, source, want string
	}{
		{"star", "\\*niet\\*", "*niet*"},
		{"greater", "5 \\> 3", "5 > 3"},
		{"underscore", "a\\_b", "a_b"},
		{"escaped entity stays literal", "\\&copy; and \\&#65;", "&copy; and &#65;"},
		{"backslash before letter stays", "a\\b", "a\\b"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			blocks, err := Parse([]byte(test.source))
			if err != nil {
				t.Fatal(err)
			}
			if len(blocks) != 1 || len(blocks[0].Spans) != 1 || blocks[0].Spans[0].Text != test.want {
				t.Fatalf("spans = %v, want %q", blocks, test.want)
			}
		})
	}
}

func TestParseEntities(t *testing.T) {
	tests := []struct {
		name, source, want string
	}{
		{"copy", "&copy;", "©"},
		{"amp", "&amp;", "&"},
		{"numeric decimal", "&#8594;", "→"},
		{"numeric hex", "&#x41;", "A"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			blocks, err := Parse([]byte(test.source))
			if err != nil {
				t.Fatal(err)
			}
			if len(blocks) != 1 || len(blocks[0].Spans) != 1 || blocks[0].Spans[0].Text != test.want {
				t.Fatalf("spans = %v, want %q", blocks, test.want)
			}
		})
	}
}

func TestParseCodeSpansKeepLiterals(t *testing.T) {
	tests := []struct {
		name, source, want string
	}{
		{"escape", "`\\*`", "\\*"},
		{"entity", "`&amp;`", "&amp;"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			blocks, err := Parse([]byte(test.source))
			if err != nil {
				t.Fatal(err)
			}
			if len(blocks) != 1 || len(blocks[0].Spans) != 1 || blocks[0].Spans[0].Text != test.want || !blocks[0].Spans[0].Code {
				t.Fatalf("spans = %v, want code span %q", blocks, test.want)
			}
		})
	}
}

func TestParseRawTextKeepsLiterals(t *testing.T) {
	source := []byte("\\*niet\\* &amp;")
	paragraph := ast.NewParagraph()
	paragraph.AppendChild(paragraph, ast.NewRawTextSegment(text.NewSegment(0, len(source))))
	got := spans(paragraph, source, false, false, false, "")
	if len(got) != 1 || got[0].Text != string(source) {
		t.Fatalf("spans = %v, want raw text %q", got, source)
	}
}

func TestParseTaskItems(t *testing.T) {
	tests := []struct {
		name, source string
		task         TaskState
		text         string
	}{
		{"open", "- [ ] open", TaskOpen, "open"},
		{"done", "- [x] klaar", TaskDone, "klaar"},
		{"plain", "- gewoon", NoTask, "gewoon"},
		{"open with leading space", "- [ ]  open", TaskOpen, "open"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			blocks, err := Parse([]byte(test.source))
			if err != nil {
				t.Fatal(err)
			}
			if len(blocks) != 1 || blocks[0].Kind != ListItem {
				t.Fatalf("Parse returned %v, want one list item", blocks)
			}
			if blocks[0].Task != test.task {
				t.Fatalf("task = %v, want %v", blocks[0].Task, test.task)
			}
			if len(blocks[0].Spans) != 1 || blocks[0].Spans[0].Text != test.text {
				t.Fatalf("spans = %v, want %q", blocks[0].Spans, test.text)
			}
		})
	}
}

func TestParseSampleHasNoRawEscapesOrEntities(t *testing.T) {
	source, err := os.ReadFile(filepath.Join("..", "..", "testdata", "sample.md"))
	if err != nil {
		t.Fatal(err)
	}
	blocks, err := Parse(source)
	if err != nil {
		t.Fatal(err)
	}
	for _, span := range collectSpans(blocks) {
		if hasEscapedPunctuation(span.Text) || entityPattern.MatchString(span.Text) {
			t.Fatalf("span %q still contains raw markup", span.Text)
		}
	}
}

func collectSpans(blocks []Block) []Span {
	var spans []Span
	for _, block := range blocks {
		spans = append(spans, block.Spans...)
		for _, row := range block.Rows {
			for _, cell := range row.Cells {
				spans = append(spans, cell.Spans...)
			}
		}
	}
	return spans
}

func hasEscapedPunctuation(s string) bool {
	for i := 0; i+1 < len(s); i++ {
		if s[i] == '\\' && util.IsPunct(s[i+1]) {
			return true
		}
	}
	return false
}

var entityPattern = regexp.MustCompile(`&(#x?[0-9a-fA-F]+|[a-zA-Z][a-zA-Z0-9]+);`)

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
