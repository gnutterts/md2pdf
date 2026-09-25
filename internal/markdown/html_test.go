// SPDX-License-Identifier: MIT

package markdown

import (
	"fmt"
	"path/filepath"
	"reflect"
	"testing"
)

func TestHTMLText(t *testing.T) {
	tests := []struct {
		name, html string
		want       []string
	}{
		{"line break", "Line one<br>line two<br/>line three.", []string{"Line one\nline two\nline three."}},
		{"line break in cell", "x<br>y", []string{"x\ny"}},
		{"div with image and paragraph", `<div align="center"><img src="logo.png" alt="Project logo"><p>Centered &amp; entity.</p></div>`, []string{"Project logo", "Centered & entity."}},
		{"details summary", `<details><summary>Click to expand</summary></details>`, []string{"Click to expand"}},
		{"table cells", `<table><tr><td>cell one</td><td>cell two</td></tr></table>`, []string{"cell one | cell two"}},
		{"comment script and style", `<!-- comment --><script>var x = 1;</script><style>p { color: red; }</style>`, nil},
		{"comment over multiple lines", "before <!-- one\n two --> after", []string{"before after"}},
		{"uppercase break", "a<BR/>b", []string{"a\nb"}},
		{"single quoted alt", `<img alt='enkel'>`, []string{"enkel"}},
		{"greater than in attribute value", `<div title="a > b">text</div>`, []string{"text"}},
		{"comment only", `<!-- nothing -->`, nil},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := htmlText(test.html)
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("htmlText(%q) = %#v, want %#v", test.html, got, test.want)
			}
		})
	}
}

func TestParseHTMLBreakInParagraph(t *testing.T) {
	blocks, err := Parse([]byte("Line one<br>line two<br/>line three."))
	if err != nil {
		t.Fatal(err)
	}
	if len(blocks) != 1 || blocks[0].Kind != Paragraph || len(blocks[0].Spans) != 1 {
		t.Fatalf("Parse returned %v, want one paragraph with one span", blocks)
	}
	want := "Line one\nline two\nline three."
	if blocks[0].Spans[0].Text != want {
		t.Fatalf("span text = %q, want %q", blocks[0].Spans[0].Text, want)
	}
}

func TestParseHTMLBreakInTable(t *testing.T) {
	blocks, err := Parse([]byte("| A |\n|---|\n| x<br>y |\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(blocks) != 1 || blocks[0].Kind != Table || len(blocks[0].Rows) != 2 {
		t.Fatalf("Parse returned %v, want a table", blocks)
	}
	cell := blocks[0].Rows[1].Cells[0]
	if len(cell.Spans) != 1 || cell.Spans[0].Text != "x\ny" {
		t.Fatalf("cell spans = %v, want one span with text %q", cell.Spans, "x\ny")
	}
}

func TestParseHTMLBlock(t *testing.T) {
	source := []byte(`<div align="center"><img src="logo.png" alt="Project logo"><p>Centered &amp; entity.</p></div>`)
	blocks, err := Parse(source)
	if err != nil {
		t.Fatal(err)
	}
	// A local <img> is an image block with its alt text; the text around it stays.
	if len(blocks) != 2 {
		t.Fatalf("Parse returned %d blocks, want 2: %v", len(blocks), blocks)
	}
	if blocks[0].Kind != Image || blocks[0].Path != "logo.png" || PlainText(blocks[0].Spans) != "Project logo" {
		t.Fatalf("block 0 = %v, want image logo.png with its alt text", blocks[0])
	}
	if blocks[1].Kind != Paragraph || PlainText(blocks[1].Spans) != "Centered & entity." {
		t.Fatalf("block 1 = %v, want the paragraph", blocks[1])
	}
}

func TestParseHTMLImages(t *testing.T) {
	dir := filepath.Join("docs")
	blocks, err := ParseIn([]byte("<p>Before <img src=\"a.png\" alt=\"A\"> after <img src=\"https://x.org/b.png\" alt=\"remote\"></p>\n"), dir)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := kinds(blocks), []string{"P:Before", "I:" + filepath.Join(dir, "a.png") + "|A", "P:after remote"}; fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("got %q, want %q", got, want)
	}
	// htmlText itself still turns every image into its alt text.
	if got := htmlText("<p>x <img src=\"a.png\" alt=\"A\"></p>"); fmt.Sprint(got) != "[x A]" {
		t.Fatalf("htmlText = %q", got)
	}
}

func TestParseDetailsWithMarkdown(t *testing.T) {
	source := []byte("<details><summary>Click to expand</summary>\n\nSome **markdown** here.\n\n</details>\n")
	blocks, err := Parse(source)
	if err != nil {
		t.Fatal(err)
	}
	if len(blocks) != 2 {
		t.Fatalf("Parse returned %d blocks, want 2: %v", len(blocks), blocks)
	}
	if blocks[0].Kind != Paragraph || len(blocks[0].Spans) != 1 || blocks[0].Spans[0].Text != "Click to expand" {
		t.Fatalf("summary block = %v, want paragraph %q", blocks[0], "Click to expand")
	}
	if blocks[1].Kind != Paragraph || spanText(blocks[1].Spans) != "Some markdown here." {
		t.Fatalf("markdown block = %v, want paragraph %q", blocks[1], "Some markdown here.")
	}
	if !hasBold(blocks[1].Spans) {
		t.Fatalf("markdown formatting lost: %v", blocks[1].Spans)
	}
}

func spanText(spans []Span) string {
	text := ""
	for _, span := range spans {
		text += span.Text
	}
	return text
}

func hasBold(spans []Span) bool {
	for _, span := range spans {
		if span.Bold {
			return true
		}
	}
	return false
}

func TestHTMLTextKeepsStrayAngleBrackets(t *testing.T) {
	tests := []struct {
		name, html string
		want       []string
	}{
		{"less than in text", "<div>5 < 6</div>", []string{"5 < 6"}},
		{"unclosed tag", "<p>a <b", []string{"a <b"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := htmlText(test.html)
			if len(got) != len(test.want) || (len(got) > 0 && got[0] != test.want[0]) {
				t.Fatalf("htmlText(%q) = %q, want %q", test.html, got, test.want)
			}
		})
	}
}
