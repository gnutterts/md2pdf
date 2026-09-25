// SPDX-License-Identifier: MIT

package runner

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

	"github.com/gnutterts/md2pdf/internal/cli"
	"github.com/gnutterts/md2pdf/internal/mermaid"
	"github.com/gnutterts/md2pdf/internal/render"
)

// pdfPages returns the object number and the decompressed content of every page, in order.
func pdfPages(t *testing.T, content []byte) (objects []string, streams []string) {
	t.Helper()
	for _, m := range regexp.MustCompile(`(\d+) 0 obj\n<</Type /Page\n(?s:.*?)/Contents (\d+) 0 R`).FindAllSubmatch(content, -1) {
		objects = append(objects, string(m[1]))
		body := regexp.MustCompile(`(?s)\n` + string(m[2]) + ` 0 obj\n<<[^>]*>>\nstream\n(.*?)\nendstream`).FindSubmatch(content)
		if body == nil {
			t.Fatalf("no content stream for page object %s", m[1])
		}
		reader, err := zlib.NewReader(bytes.NewReader(body[1]))
		if err != nil {
			t.Fatal(err)
		}
		data, _ := io.ReadAll(reader)
		streams = append(streams, string(data))
	}
	return objects, streams
}

func tocRun(t *testing.T, markdown string, plan cli.Plan) ([]string, []string, []byte, []string) {
	t.Helper()
	dir := t.TempDir()
	source := filepath.Join(dir, "doc.md")
	if err := os.WriteFile(source, []byte(markdown), 0o600); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(dir, "doc.pdf")
	plan.Mode = cli.ModeSingle
	plan.Tasks = []cli.Task{{Sources: []string{source}, Target: target}}
	var warnings []string
	if err := Run(plan, render.Options{Warn: func(m string) { warnings = append(warnings, m) }}); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	objects, streams := pdfPages(t, content)
	return objects, streams, content, warnings
}

// filler is enough text to push what follows onto a later page.
var filler = strings.Repeat("Some words that fill the page. ", 180) + "\n\n"

// pageOf returns the page (from 1) whose stream draws text, or 0.
func pageOf(streams []string, text string) int {
	for i, stream := range streams {
		if strings.Contains(stream, "("+text+")") {
			return i + 1
		}
	}
	return 0
}

func TestTOCListsHeadingsWithTheirPagesAndLinks(t *testing.T) {
	doc := "# Alpha\n\n" + filler + "## Beta\n\n" + filler + "# Gamma\n\ntext\n"
	objects, streams, content, _ := tocRun(t, doc, cli.Plan{TOC: true})
	if !strings.Contains(streams[0], "(Contents)") {
		t.Fatal("page 1 is not the table of contents")
	}
	for _, heading := range []string{"Alpha", "Beta", "Gamma"} {
		page := pageOf(streams[1:], heading) + 1
		if page < 2 {
			t.Fatalf("%s is not drawn after the table of contents", heading)
		}
		// The table shows the heading followed by its page number.
		entry := regexp.MustCompile(`\(` + heading + `\) ?Tj.*?\((\d+)\) ?Tj`).FindStringSubmatch(strings.ReplaceAll(streams[0], "\n", " "))
		if entry == nil || entry[1] != strconv.Itoa(page) {
			t.Errorf("%s: the table says page %v, the heading is on page %d", heading, entry, page)
		}
	}
	// Every entry has two links (text and number) to the page object of its heading.
	dests := regexp.MustCompile(`/Subtype /Link[^>]*?/Dest \[(\d+) 0 R`).FindAllSubmatch(content, -1)
	var got []string
	for _, d := range dests {
		got = append(got, string(d[1]))
	}
	want := []string{}
	for _, heading := range []string{"Alpha", "Beta", "Gamma"} {
		object := objects[pageOf(streams[1:], heading)]
		want = append(want, object, object)
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("link destinations %v, want %v", got, want)
	}
}

func TestALongTableOfContentsSpansPagesAndStaysRight(t *testing.T) {
	var doc strings.Builder
	for i := 1; i <= 300; i++ {
		fmt.Fprintf(&doc, "## Heading number %d\n\nA line of text under heading %d.\n\n", i, i)
	}
	_, streams, _, _ := tocRun(t, doc.String(), cli.Plan{TOC: true})
	table := pageOf(streams, "A line of text under heading 1.") - 1 // pages before the first body text
	if table < 2 {
		t.Fatalf("the table of contents takes %d pages, want several", table)
	}
	joined := strings.ReplaceAll(strings.Join(streams[:table], " "), "\n", " ")
	for _, i := range []int{1, 150, 300} {
		heading := fmt.Sprintf("Heading number %d", i)
		entry := regexp.MustCompile(`\(` + heading + `\) ?Tj.*?\((\d+)\) ?Tj`).FindStringSubmatch(joined)
		page := table + pageOf(streams[table:], heading)
		if entry == nil || entry[1] != strconv.Itoa(page) {
			t.Errorf("%s: the table says %v, the heading is on page %d", heading, entry, page)
		}
	}
}

func TestTOCDepthLimitsTheEntries(t *testing.T) {
	_, streams, _, _ := tocRun(t, "# One\n\n## Two\n\n### Three\n", cli.Plan{TOC: true, TOCDepth: 1})
	if !strings.Contains(streams[0], "(One)") || strings.Contains(streams[0], "(Two)") {
		t.Fatalf("table of contents:\n%s", streams[0])
	}
}

func TestTOCWithoutHeadingsWarnsAndAddsNoPage(t *testing.T) {
	_, streams, _, warnings := tocRun(t, "just text\n", cli.Plan{TOC: true})
	if len(streams) != 1 || strings.Contains(streams[0], "(Contents)") {
		t.Fatalf("%d pages; want only the text", len(streams))
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "no table of contents") {
		t.Fatalf("warnings = %q", warnings)
	}
}

func TestTOCWarnsOnceForAFailedDiagram(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "fails")
	os.WriteFile(script, []byte("#!/bin/sh\nexit 1\n"), 0o755)
	source := filepath.Join(dir, "doc.md")
	os.WriteFile(source, []byte("# Title\n\n```mermaid\ngraph TD\n```\n"), 0o600)
	var warnings []string
	options := render.Options{Mermaid: mermaid.Renderer{Path: script}}
	plan := cli.Plan{Mode: cli.ModeSingle, TOC: true, Tasks: []cli.Task{{Sources: []string{source}, Target: filepath.Join(dir, "doc.pdf")}}}
	options.Warn = func(m string) { warnings = append(warnings, m) }
	if err := Run(plan, options); err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 1 {
		t.Fatalf("warnings = %q, want one", warnings)
	}
}
