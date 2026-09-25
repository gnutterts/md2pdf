// SPDX-License-Identifier: MIT

package render

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gnutterts/md2pdf/internal/markdown"
	"github.com/gnutterts/md2pdf/internal/mermaid"
)

type activeStyle struct {
	family       string
	size         float64
	bold, italic bool
}

type fakeCanvas struct {
	calls    []string
	style    activeStyle
	tables   [][]markdown.Row
	scales   []float64 // the scale of every diagram
	imageErr error     // what Image returns
}

func (n *fakeCanvas) NewPage() { n.calls = append(n.calls, "pagina") }
func (n *fakeCanvas) Style(f string, v, c bool, g float64) {
	n.style = activeStyle{family: f, size: g, bold: v, italic: c}
	n.calls = append(n.calls, "style:"+n.style.string())
}
func (n *fakeCanvas) Strike(on bool) {
	n.calls = append(n.calls, fmt.Sprintf("strike:%t", on))
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
func (n *fakeCanvas) Marker(s string) {
	n.calls = append(n.calls, "marker:"+s+":"+n.style.string())
}
func (n *fakeCanvas) Checkbox(prefix string, checked bool) {
	n.calls = append(n.calls, fmt.Sprintf("checkbox:%s:%t:%s", prefix, checked, n.style.string()))
}
func (n *fakeCanvas) Quote(levels int, draw func()) {
	n.calls = append(n.calls, fmt.Sprintf("quote-start:%d", levels))
	draw()
	n.calls = append(n.calls, fmt.Sprintf("quote-end:%d", levels))
}
func (n *fakeCanvas) HangingIndent() { n.calls = append(n.calls, "hanging") }
func (n *fakeCanvas) CodeBlock(r []string) {
	n.calls = append(n.calls, "code:"+strings.Join(r, ","))
}
func (n *fakeCanvas) Diagram(_ []byte, scale float64) error {
	n.calls = append(n.calls, "diagram")
	n.scales = append(n.scales, scale)
	return nil
}
func (n *fakeCanvas) Image(path string) error {
	if n.imageErr != nil {
		return n.imageErr
	}
	n.calls = append(n.calls, "image:"+path)
	return nil
}
func (n *fakeCanvas) Rule() { n.calls = append(n.calls, "rule") }
func (n *fakeCanvas) Bookmark(text string, level int) {
	n.calls = append(n.calls, fmt.Sprintf("bookmark:%d:%s", level, text))
}
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
		{"nested list", []markdown.Block{{Kind: markdown.ListItem, Depth: 1, Spans: []markdown.Span{{Text: "inside"}}}}, []string{"indent", "marker:• :Helvetica::11", "text:inside:Helvetica::11", "indent"}},
		{"code block", []markdown.Block{{Kind: markdown.CodeBlock, Lines: []string{"x"}}}, []string{"indent", "style:Courier::9.5", "code:x", "indent"}},
		{"link", []markdown.Block{{Kind: markdown.Paragraph, Spans: []markdown.Span{{Text: "site", URL: "https://x"}}}}, []string{"style:Helvetica::11", "link:site:https://x:Helvetica::11"}},
		{"quote", []markdown.Block{{Kind: markdown.Paragraph, Quote: 1, Spans: []markdown.Span{{Text: "woord"}}}}, []string{"indent", "style:Helvetica:I:11", "text:woord:Helvetica:I:11", "indent"}},
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
		name    string
		task    markdown.TaskState
		checked bool
	}{
		{"open", markdown.TaskOpen, false},
		{"done", markdown.TaskDone, true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			canvas := &fakeCanvas{}
			blocks := []markdown.Block{{Kind: markdown.ListItem, Task: test.task, Spans: []markdown.Span{{Text: "doe"}}}}
			if err := Draw(blocks, canvas, Options{}); err != nil {
				t.Fatal(err)
			}
			checkOrder(t, canvas.calls, []string{fmt.Sprintf("checkbox::%t:Helvetica::11", test.checked), "text:doe:Helvetica::11"})
			for _, call := range canvas.calls {
				if strings.HasPrefix(call, "marker:") {
					t.Fatalf("task item drew a marker: %q", canvas.calls)
				}
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
	checkOrder(t, canvas.calls, []string{"checkbox:3. :true:Helvetica::11", "text:doe:Helvetica::11"})
}

func TestDrawQuoteBlocksShareOneQuoteCall(t *testing.T) {
	canvas := &fakeCanvas{}
	blocks := []markdown.Block{
		{Kind: markdown.Paragraph, Quote: 1, Spans: []markdown.Span{{Text: "one"}}},
		{Kind: markdown.Paragraph, Quote: 1, Spans: []markdown.Span{{Text: "two"}}},
	}
	if err := Draw(blocks, canvas, Options{}); err != nil {
		t.Fatal(err)
	}
	checkOrder(t, canvas.calls, []string{
		"quote-start:1",
		"text:one:Helvetica:I:11",
		"text:two:Helvetica:I:11",
		"quote-end:1",
	})
	quoteCalls := 0
	for _, call := range canvas.calls {
		if call == "quote-start:1" {
			quoteCalls++
		}
	}
	if quoteCalls != 1 {
		t.Fatalf("two quote paragraphs used %d Quote calls, want 1: %q", quoteCalls, canvas.calls)
	}
}

func TestDrawListItemSpacing(t *testing.T) {
	canvas := &fakeCanvas{}
	blocks := []markdown.Block{
		{Kind: markdown.ListItem, Spans: []markdown.Span{{Text: "one"}}},
		{Kind: markdown.ListItem, Spans: []markdown.Span{{Text: "two"}}},
	}
	if err := Draw(blocks, canvas, Options{}); err != nil {
		t.Fatal(err)
	}
	if got := canvas.calls[len(canvas.calls)-1]; got != "end:6" {
		t.Fatalf("last list item is not followed by LineBreak(6): %q", canvas.calls)
	}
	spacing := 0
	for _, call := range canvas.calls {
		if call == "end:6" {
			spacing++
		}
	}
	if spacing != 1 {
		t.Fatalf("two list items have %d LineBreak(6) calls, want only the final one: %q", spacing, canvas.calls)
	}
}

func TestDrawListThenParagraphGetsSpacing(t *testing.T) {
	canvas := &fakeCanvas{}
	blocks := []markdown.Block{
		{Kind: markdown.ListItem, Spans: []markdown.Span{{Text: "item"}}},
		{Kind: markdown.Paragraph, Spans: []markdown.Span{{Text: "after"}}},
	}
	if err := Draw(blocks, canvas, Options{}); err != nil {
		t.Fatal(err)
	}
	checkOrder(t, canvas.calls, []string{
		"text:item:Helvetica::11",
		"end:6",
		"text:after:Helvetica::11",
	})
}

func TestDrawTwoAdjacentListsOfDifferentKindGetSpacing(t *testing.T) {
	canvas := &fakeCanvas{}
	blocks := []markdown.Block{
		{Kind: markdown.ListItem, Ordered: false, Spans: []markdown.Span{{Text: "bullet"}}},
		{Kind: markdown.ListItem, Ordered: true, Number: 1, Spans: []markdown.Span{{Text: "number"}}},
	}
	if err := Draw(blocks, canvas, Options{}); err != nil {
		t.Fatal(err)
	}
	checkOrder(t, canvas.calls, []string{
		"text:bullet:Helvetica::11",
		"end:6",
		"text:number:Helvetica::11",
	})
}

func TestDrawContinuedListItemSkipsMarker(t *testing.T) {
	canvas := &fakeCanvas{}
	blocks := []markdown.Block{{Kind: markdown.ListItem, Depth: 0, Continued: true, Spans: []markdown.Span{{Text: "vervolg"}}}}
	if err := Draw(blocks, canvas, Options{}); err != nil {
		t.Fatal(err)
	}
	for _, call := range canvas.calls {
		if strings.HasPrefix(call, "marker:") {
			t.Fatalf("continuation drew a marker: %q", canvas.calls)
		}
	}
	checkOrder(t, canvas.calls, []string{"text:vervolg:Helvetica::11"})
}

func TestDrawListItemDrawsOneMarker(t *testing.T) {
	canvas := &fakeCanvas{}
	blocks := []markdown.Block{{Kind: markdown.ListItem, Depth: 0, Spans: []markdown.Span{{Text: "een"}}}}
	if err := Draw(blocks, canvas, Options{}); err != nil {
		t.Fatal(err)
	}
	markers := 0
	for _, call := range canvas.calls {
		if strings.HasPrefix(call, "marker:") {
			markers++
		}
	}
	if markers != 1 {
		t.Fatalf("item drew %d markers, want 1: %q", markers, canvas.calls)
	}
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
		{"heading level 5", markdown.Block{Kind: markdown.Heading, Level: 5, Spans: []markdown.Span{{Text: "Five"}}}, "text:Five:Helvetica:BI:11"},
		{"heading level 6", markdown.Block{Kind: markdown.Heading, Level: 6, Spans: []markdown.Span{{Text: "Six"}}}, "text:Six:Helvetica:I:11"},
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

func TestDrawStrikethrough(t *testing.T) {
	canvas := &fakeCanvas{}
	blocks := []markdown.Block{{Kind: markdown.Paragraph, Spans: []markdown.Span{{Text: "weg", Strike: true}}}}
	if err := Draw(blocks, canvas, Options{}); err != nil {
		t.Fatal(err)
	}
	checkOrder(t, canvas.calls, []string{"strike:true", "text:weg:Helvetica::11", "strike:false"})
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

func TestDrawListSpaceWhenQuoteLevelChanges(t *testing.T) {
	canvas := &fakeCanvas{}
	blocks := []markdown.Block{
		{Kind: markdown.ListItem, Quote: 1, Spans: []markdown.Span{{Text: "quoted"}}},
		{Kind: markdown.ListItem, Spans: []markdown.Span{{Text: "plain"}}},
	}
	if err := Draw(blocks, canvas, Options{}); err != nil {
		t.Fatal(err)
	}
	checkOrder(t, canvas.calls, []string{"text:quoted:Helvetica:I:11", "end:6", "text:plain:Helvetica::11"})
}

func TestHeadingsBecomeBookmarks(t *testing.T) {
	heading := func(level int, text string) markdown.Block {
		return markdown.Block{Kind: markdown.Heading, Level: level, Spans: []markdown.Span{{Text: text, Bold: true}, {Text: " two", Code: true}}}
	}
	canvas := &fakeCanvas{}
	blocks := []markdown.Block{heading(1, "One"), heading(3, "Three"), heading(2, "Two")}
	if err := Draw(blocks, canvas, Options{}); err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, call := range canvas.calls {
		if strings.HasPrefix(call, "bookmark:") {
			got = append(got, call)
		}
	}
	want := []string{"bookmark:1:One two", "bookmark:3:Three two", "bookmark:2:Two two"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("bookmarks = %v, want %v", got, want)
	}
}

func TestStrictTurnsAWarningIntoAnErrorAndStops(t *testing.T) {
	path := filepath.Join(t.TempDir(), "renderer")
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	blocks := []markdown.Block{
		{Kind: markdown.CodeBlock, Language: "mermaid", Lines: []string{"graph TD"}},
		{Kind: markdown.Paragraph, Spans: []markdown.Span{{Text: "after"}}},
	}
	canvas := &fakeCanvas{}
	warnings := 0
	err := Draw(blocks, canvas, Options{Mermaid: mermaid.Renderer{Path: path}, Strict: true, Warn: func(string) { warnings++ }})
	if err == nil || !strings.Contains(err.Error(), "could not draw mermaid diagram") || !strings.HasSuffix(err.Error(), "(--strict)") {
		t.Fatalf("err = %v", err)
	}
	if warnings != 1 {
		t.Fatalf("warnings = %d, want the warning to be reported once", warnings)
	}
	if contains(canvas.calls, "text:after:Helvetica::11") {
		t.Fatalf("drawing went on after the strict error: %v", canvas.calls)
	}
	if contains(canvas.calls, "code:graph TD") {
		t.Fatalf("the fallback code block was drawn although --strict failed the run: %v", canvas.calls)
	}
}

func TestStrictStopsInsideAQuote(t *testing.T) {
	path := filepath.Join(t.TempDir(), "renderer")
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	blocks := []markdown.Block{
		{Kind: markdown.CodeBlock, Language: "mermaid", Lines: []string{"graph TD"}, Quote: 1},
		{Kind: markdown.Paragraph, Quote: 1, Spans: []markdown.Span{{Text: "after"}}},
	}
	canvas := &fakeCanvas{}
	if err := Draw(blocks, canvas, Options{Mermaid: mermaid.Renderer{Path: path}, Strict: true}); err == nil {
		t.Fatal("no error")
	}
	for _, call := range canvas.calls {
		if strings.HasPrefix(call, "text:after") {
			t.Fatalf("drawing went on after the strict error: %v", canvas.calls)
		}
	}
}

func TestDiagramsAreDrawnWithTheRendererScale(t *testing.T) {
	path := filepath.Join(t.TempDir(), "renderer")
	if err := os.WriteFile(path, []byte("#!/bin/sh\nprintf png > \"$4\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	canvas := &fakeCanvas{}
	block := []markdown.Block{{Kind: markdown.CodeBlock, Language: "mermaid", Lines: []string{"graph TD"}}}
	if err := Draw(block, canvas, Options{Mermaid: mermaid.Renderer{Path: path, Scale: 2}}); err != nil {
		t.Fatal(err)
	}
	if len(canvas.scales) != 1 || canvas.scales[0] != 2 {
		t.Fatalf("scales = %v, want [2]", canvas.scales)
	}
}

func TestTheDiagramCacheRendersEachTextOnce(t *testing.T) {
	calls := map[string]int{}
	cache := NewDiagramCache(func(text string) ([]byte, error) { calls[text]++; return []byte("png:" + text), nil })
	blocks := []markdown.Block{
		{Kind: markdown.CodeBlock, Language: "mermaid", Lines: []string{"graph TD", "A-->B"}},
		{Kind: markdown.CodeBlock, Language: "mermaid", Lines: []string{"graph TD", "A-->B"}},
		{Kind: markdown.CodeBlock, Language: "mermaid", Lines: []string{"graph TD", "C-->D"}},
	}
	canvas := &fakeCanvas{}
	if err := Draw(blocks, canvas, Options{Mermaid: mermaid.Renderer{Path: "unused"}, Diagram: cache}); err != nil {
		t.Fatal(err)
	}
	if calls["graph TD\nA-->B"] != 1 || calls["graph TD\nC-->D"] != 1 || len(calls) != 2 {
		t.Fatalf("calls = %v, want one per distinct text", calls)
	}
	if n := len(canvas.scales); n != 3 {
		t.Fatalf("%d diagrams drawn, want 3", n)
	}
}

func TestTheDiagramCacheRemembersAFailureButEveryPlaceWarns(t *testing.T) {
	calls := 0
	cache := NewDiagramCache(func(string) ([]byte, error) { calls++; return nil, errors.New("broken") })
	blocks := []markdown.Block{
		{Kind: markdown.CodeBlock, Language: "mermaid", Lines: []string{"graph TD"}},
		{Kind: markdown.CodeBlock, Language: "mermaid", Lines: []string{"graph TD"}},
	}
	warnings := 0
	err := Draw(blocks, &fakeCanvas{}, Options{Mermaid: mermaid.Renderer{Path: "unused"}, Diagram: cache, Warn: func(string) { warnings++ }})
	if err != nil || calls != 1 || warnings != 2 {
		t.Fatalf("err = %v, calls = %d, warnings = %d, want nil, 1, 2", err, calls, warnings)
	}
}

func TestWithoutADiagramFunctionTheRendererIsUsed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "renderer")
	if err := os.WriteFile(path, []byte("#!/bin/sh\nprintf png > \"$4\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	canvas := &fakeCanvas{}
	block := []markdown.Block{{Kind: markdown.CodeBlock, Language: "mermaid", Lines: []string{"graph TD"}}}
	if err := Draw(block, canvas, Options{Mermaid: mermaid.Renderer{Path: path}}); err != nil || !contains(canvas.calls, "diagram") {
		t.Fatalf("err = %v, calls = %v", err, canvas.calls)
	}
}

func TestTheDiagramCacheRendersOnceUnderConcurrency(t *testing.T) {
	var calls atomic.Int32
	release := make(chan struct{})
	cache := NewDiagramCache(func(text string) ([]byte, error) {
		calls.Add(1)
		<-release // hold the first render until every goroutine is waiting
		return []byte("png:" + text), nil
	})
	const goroutines = 16
	var wait sync.WaitGroup
	results := make([]string, goroutines)
	for i := 0; i < goroutines; i++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			png, err := cache("graph TD")
			if err != nil {
				t.Error(err)
			}
			results[i] = string(png)
		}()
	}
	time.Sleep(50 * time.Millisecond)
	close(release)
	wait.Wait()
	if calls.Load() != 1 {
		t.Fatalf("%d renders, want 1", calls.Load())
	}
	for i, result := range results {
		if result != "png:graph TD" {
			t.Errorf("goroutine %d got %q", i, result)
		}
	}
}

func TestAnImageBlockDrawsNoAltTextYet(t *testing.T) {
	canvas := &fakeCanvas{}
	blocks := []markdown.Block{
		{Kind: markdown.Paragraph, Spans: []markdown.Span{{Text: "before"}}},
		{Kind: markdown.Image, Path: "p.png", Spans: []markdown.Span{{Text: "secret alt"}}},
		{Kind: markdown.Paragraph, Spans: []markdown.Span{{Text: "after"}}},
	}
	if err := Draw(blocks, canvas, Options{}); err != nil {
		t.Fatal(err)
	}
	for _, call := range canvas.calls {
		if strings.Contains(call, "secret alt") {
			t.Fatalf("alt text leaked: %v", canvas.calls)
		}
	}
}

func TestAnImageIsDrawnOrFallsBackToItalicAltText(t *testing.T) {
	block := []markdown.Block{{Kind: markdown.Image, Path: "p.png", Spans: []markdown.Span{{Text: "Alt"}}}}
	canvas := &fakeCanvas{}
	if err := Draw(block, canvas, Options{}); err != nil || !contains(canvas.calls, "image:p.png") {
		t.Fatalf("err = %v, calls = %v", err, canvas.calls)
	}
	canvas = &fakeCanvas{imageErr: errors.New("not found")}
	var warnings []string
	if err := Draw(block, canvas, Options{Warn: func(m string) { warnings = append(warnings, m) }}); err != nil {
		t.Fatal(err)
	}
	if !contains(canvas.calls, "text:Alt:Helvetica:I:11") || len(warnings) != 1 || warnings[0] != `image "p.png" skipped: not found` {
		t.Fatalf("calls = %v, warnings = %q", canvas.calls, warnings)
	}
	canvas = &fakeCanvas{imageErr: errors.New("not found")}
	if err := Draw(block, canvas, Options{Strict: true}); err == nil || !strings.Contains(err.Error(), "(--strict)") {
		t.Fatalf("strict: err = %v", err)
	}
	for _, call := range canvas.calls {
		if strings.HasPrefix(call, "text:") {
			t.Fatalf("strict drew the fallback: %v", canvas.calls)
		}
	}
	noAlt := []markdown.Block{{Kind: markdown.Image, Path: filepath.Join("dir", "logo.png")}}
	canvas = &fakeCanvas{imageErr: errors.New("not found")}
	if err := Draw(noAlt, canvas, Options{}); err != nil || !contains(canvas.calls, "text:logo.png:Helvetica:I:11") {
		t.Fatalf("without alt text: err = %v, calls = %v", err, canvas.calls)
	}
}
