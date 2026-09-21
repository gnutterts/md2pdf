// SPDX-License-Identifier: MIT

package render

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gnutterts/md2pdf/internal/markdown"
	"github.com/gnutterts/md2pdf/internal/mermaid"
)

type activeStyle struct {
	family       string
	size         float64
	bold, italic bool
}

type fakeCanvas struct {
	calls  []string
	style  activeStyle
	tables [][]markdown.Row
}

func (n *fakeCanvas) NewPage() { n.calls = append(n.calls, "pagina") }
func (n *fakeCanvas) Style(f string, v, c bool, g float64) {
	n.style = activeStyle{family: f, size: g, bold: v, italic: c}
	n.calls = append(n.calls, "style:"+n.style.string())
}
func (n *fakeCanvas) Text(s string) {
	n.calls = append(n.calls, "text:"+s+":"+n.style.string())
}
func (n *fakeCanvas) Link(s, u string) {
	n.calls = append(n.calls, "link:"+s+":"+u+":"+n.style.string())
}
func (n *fakeCanvas) LineBreak(h float64) {
	n.calls = append(n.calls, fmt.Sprintf("end:%g", h))
}
func (n *fakeCanvas) Indent(p float64) { n.calls = append(n.calls, "indent") }
func (n *fakeCanvas) HangingIndent()   { n.calls = append(n.calls, "hanging") }
func (n *fakeCanvas) CodeBlock(r []string) {
	n.calls = append(n.calls, "code:"+strings.Join(r, ","))
}
func (n *fakeCanvas) Diagram([]byte) error { n.calls = append(n.calls, "diagram"); return nil }
func (n *fakeCanvas) Rule()                { n.calls = append(n.calls, "rule") }
func (n *fakeCanvas) Table(rows []markdown.Row) {
	n.tables = append(n.tables, rows)
	n.calls = append(n.calls, fmt.Sprintf("table:%d", len(rows)))
}
func (n *fakeCanvas) Err() error { return nil }
func (s activeStyle) string() string {
	return fmt.Sprintf("%s:%s:%g", s.family, style(s.bold, s.italic), s.size)
}
func TestDrawMermaidFallback(t *testing.T) {
	block := []markdown.Block{{Kind: markdown.CodeBlock, Language: "mermaid", Lines: []string{"graph TD", "A-->B"}}}
	for _, test := range []struct {
		name, script string
		wantDiagram  bool
		warnings     int
	}{
		{"succeeds", "#!/bin/sh\nprintf png > \"$4\"\n", true, 0},
		{"fails", "#!/bin/sh\nexit 1\n", false, 1},
		{"off", "", false, 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			canvas := &fakeCanvas{}
			options := Options{}
			if test.script != "" {
				path := filepath.Join(t.TempDir(), "renderer")
				if err := os.WriteFile(path, []byte(test.script), 0o755); err != nil {
					t.Fatal(err)
				}
				options.Mermaid = mermaid.Renderer{Path: path}
			}
			warnings := 0
			options.Warn = func(string) { warnings++ }
			if err := Draw(block, canvas, options); err != nil {
				t.Fatal(err)
			}
			hasDiagram := contains(canvas.calls, "diagram")
			hasCode := contains(canvas.calls, "code:graph TD,A-->B")
			if hasDiagram != test.wantDiagram || hasCode == test.wantDiagram || warnings != test.warnings {
				t.Fatalf("calls=%v, warnings=%d", canvas.calls, warnings)
			}
		})
	}
}

func contains(calls []string, want string) bool {
	for _, call := range calls {
		if call == want {
			return true
		}
	}
	return false
}

func style(v, c bool) string {
	if v && c {
		return "BI"
	}
	if v {
		return "B"
	}
	if c {
		return "I"
	}
	return ""
}

func TestDrawOrderAndStyles(t *testing.T) {
	tests := []struct {
		name   string
		blocks []markdown.Block
		want   []string
	}{
		{"heading and paragraph", []markdown.Block{{Kind: markdown.Heading, Level: 1, Spans: []markdown.Span{{Text: "Titel"}}}, {Kind: markdown.Paragraph, Spans: []markdown.Span{{Text: "text"}}}}, []string{"style:Helvetica:B:20", "text:Titel:Helvetica:B:20", "style:Helvetica::11", "text:text:Helvetica::11"}},
		{"nested list", []markdown.Block{{Kind: markdown.ListItem, Depth: 1, Spans: []markdown.Span{{Text: "inside"}}}}, []string{"indent", "text:• :Helvetica::11", "text:inside:Helvetica::11", "indent"}},
		{"code block", []markdown.Block{{Kind: markdown.CodeBlock, Lines: []string{"x"}}}, []string{"indent", "style:Courier::9.5", "code:x", "indent"}},
		{"link", []markdown.Block{{Kind: markdown.Paragraph, Spans: []markdown.Span{{Text: "site", URL: "https://x"}}}}, []string{"style:Helvetica::11", "link:site:https://x:Helvetica::11"}},
		{"quote", []markdown.Block{{Kind: markdown.Quote, Spans: []markdown.Span{{Text: "woord"}}}}, []string{"indent", "style:Helvetica:I:11", "text:woord:Helvetica:I:11", "indent"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			canvas := &fakeCanvas{}
			if err := Draw(test.blocks, canvas, Options{}); err != nil {
				t.Fatal(err)
			}
			checkOrder(t, canvas.calls, test.want)
		})
	}
}

func TestDrawTaskItems(t *testing.T) {
	tests := []struct {
		name string
		task markdown.TaskState
		want string
	}{
		{"open", markdown.TaskOpen, "text:[ ] :Helvetica::11"},
		{"done", markdown.TaskDone, "text:[x] :Helvetica::11"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			canvas := &fakeCanvas{}
			blocks := []markdown.Block{{Kind: markdown.ListItem, Task: test.task, Spans: []markdown.Span{{Text: "doe"}}}}
			if err := Draw(blocks, canvas, Options{}); err != nil {
				t.Fatal(err)
			}
			checkOrder(t, canvas.calls, []string{test.want, "text:doe:Helvetica::11"})
			if contains(canvas.calls, "text:• :Helvetica::11") {
				t.Fatalf("task item drew a bullet: %q", canvas.calls)
			}
		})
	}
}

func TestDrawOrderedTaskKeepsNumber(t *testing.T) {
	canvas := &fakeCanvas{}
	blocks := []markdown.Block{{Kind: markdown.ListItem, Ordered: true, Number: 3, Task: markdown.TaskDone, Spans: []markdown.Span{{Text: "doe"}}}}
	if err := Draw(blocks, canvas, Options{}); err != nil {
		t.Fatal(err)
	}
	checkOrder(t, canvas.calls, []string{"text:3. [x] :Helvetica::11", "text:doe:Helvetica::11"})
}

func TestDrawTable(t *testing.T) {
	blocks := []markdown.Block{{Kind: markdown.Table, Rows: []markdown.Row{
		{Header: true, Cells: []markdown.Cell{{Spans: []markdown.Span{{Text: "A"}}}, {Spans: []markdown.Span{{Text: "B"}}}}},
		{Cells: []markdown.Cell{{Spans: []markdown.Span{{Text: "1"}}}, {Spans: []markdown.Span{{Text: "2"}}}}},
	}}}
	canvas := &fakeCanvas{}
	if err := Draw(blocks, canvas, Options{}); err != nil {
		t.Fatal(err)
	}
	tableAanroepen := 0
	for _, call := range canvas.calls {
		if strings.HasPrefix(call, "table:") {
			tableAanroepen++
		}
	}
	if tableAanroepen != 1 {
		t.Fatalf("Tabel is %d keer aangeroepen: %q", tableAanroepen, canvas.calls)
	}
	if len(canvas.tables) != 1 || len(canvas.tables[0]) != 2 {
		t.Fatalf("Tabel received %d rows, want 2: %v", len(canvas.tables), canvas.tables)
	}
	if !canvas.tables[0][0].Header || canvas.tables[0][1].Header {
		t.Fatalf("header row is not the first row: %v", canvas.tables[0])
	}
}

func TestLineHeightFollowsHeadingSize(t *testing.T) {
	canvas := &fakeCanvas{}
	if err := Draw([]markdown.Block{{Kind: markdown.Heading, Level: 1, Spans: []markdown.Span{{Text: "Titel"}}}}, canvas, Options{}); err != nil {
		t.Fatal(err)
	}
	checkOrder(t, canvas.calls, []string{"end:27"})
}

func TestTextStyles(t *testing.T) {
	tests := []struct {
		name  string
		block markdown.Block
		want  string
	}{
		{"heading level 1", markdown.Block{Kind: markdown.Heading, Level: 1, Spans: []markdown.Span{{Text: "One"}}}, "text:One:Helvetica:B:20"},
		{"heading level 2", markdown.Block{Kind: markdown.Heading, Level: 2, Spans: []markdown.Span{{Text: "Two"}}}, "text:Two:Helvetica:B:16"},
		{"heading level 3", markdown.Block{Kind: markdown.Heading, Level: 3, Spans: []markdown.Span{{Text: "Three"}}}, "text:Three:Helvetica:B:13"},
		{"heading level 4", markdown.Block{Kind: markdown.Heading, Level: 4, Spans: []markdown.Span{{Text: "Four"}}}, "text:Four:Helvetica:B:11"},
		{"bold paragraph", markdown.Block{Kind: markdown.Paragraph, Spans: []markdown.Span{{Text: "bold", Bold: true}}}, "text:bold:Helvetica:B:11"},
		{"code in heading", markdown.Block{Kind: markdown.Heading, Level: 1, Spans: []markdown.Span{{Text: "code", Code: true}}}, "text:code:Courier:B:17"},
		{"code in paragraph", markdown.Block{Kind: markdown.Paragraph, Spans: []markdown.Span{{Text: "code", Code: true}}}, "text:code:Courier::9.5"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			canvas := &fakeCanvas{}
			if err := Draw([]markdown.Block{test.block}, canvas, Options{}); err != nil {
				t.Fatal(err)
			}
			checkOrder(t, canvas.calls, []string{test.want})
		})
	}
}

func checkOrder(t *testing.T, got, want []string) {
	t.Helper()
	start := 0
	for _, expected := range want {
		found := -1
		for i := start; i < len(got); i++ {
			if got[i] == expected {
				found = i
				break
			}
		}
		if found < 0 {
			t.Fatalf("%q is missing in %q", expected, got)
		}
		start = found + 1
	}
}
