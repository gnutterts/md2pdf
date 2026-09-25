// SPDX-License-Identifier: MIT

package markdown

import (
	"fmt"
	"math/rand"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// kinds summarises blocks as "Kind:text" for comparison.
func kinds(blocks []Block) []string {
	var out []string
	names := map[Kind]string{Heading: "H", Paragraph: "P", ListItem: "L", CodeBlock: "C", Rule: "R", Table: "T", Image: "I"}
	for _, b := range blocks {
		text := PlainText(b.Spans)
		if b.Kind == Image {
			text = b.Path + "|" + text
		}
		if b.Continued {
			text = "+" + text
		}
		out = append(out, names[b.Kind]+":"+text)
	}
	return out
}

func parseIn(t *testing.T, source, dir string) []Block {
	t.Helper()
	blocks, err := ParseIn([]byte(source), dir)
	if err != nil {
		t.Fatal(err)
	}
	return blocks
}

func TestImageBecomesABlock(t *testing.T) {
	dir := filepath.Join("docs", "x")
	for _, test := range []struct {
		name, source string
		want         []string
	}{
		{"alone", "![alt *text*](p.png)\n", []string{"I:" + filepath.Join(dir, "p.png") + "|alt text"}},
		{"inline", "before ![a](p.png) after\n", []string{"P:before", "I:" + filepath.Join(dir, "p.png") + "|a", "P:after"}},
		{"two", "![a](1.png)![b](2.png)\n", []string{"I:" + filepath.Join(dir, "1.png") + "|a", "I:" + filepath.Join(dir, "2.png") + "|b"}},
		{"absolute", "![a](/tmp/p.png)\n", []string{"I:" + filepath.FromSlash("/tmp/p.png") + "|a"}},
		{"escaped", "![a](my%20pic.png)\n", []string{"I:" + filepath.Join(dir, "my pic.png") + "|a"}},
		{"remote stays text", "see ![remote](https://x.org/p.png) here\n", []string{"P:see remote here"}},
		{"query and fragment are dropped", "![a](p.png?v=2#top)\n", []string{"I:" + filepath.Join(dir, "p.png") + "|a"}},
		{"other schemes stay text", "![a](file:///etc/p.png) ![b](javascript:x)\n", []string{"P:a b"}},
		{"data stays text", "![d](data:image/png;base64,AAAA)\n", []string{"P:d"}},
		{"in a heading stays text", "# Title ![a](p.png)\n", []string{"H:Title a"}},
		{"in a link stays text", "[![a](p.png)](https://x.org)\n", []string{"P:a"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := kinds(parseIn(t, test.source, dir)); fmt.Sprint(got) != fmt.Sprint(test.want) {
				t.Fatalf("got %q, want %q", got, test.want)
			}
		})
	}
}

func TestImageInAListItemKeepsTheItem(t *testing.T) {
	blocks := parseIn(t, "- first ![a](p.png) rest\n- second\n", "")
	if got, want := kinds(blocks), []string{"L:first", "I:p.png|a", "L:+rest", "L:second"}; fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("got %q, want %q", got, want)
	}
	if image := blocks[1]; !image.InItem || image.Depth != 0 {
		t.Fatalf("image block = %+v, want InItem at depth 0", image)
	}
	if blocks[0].Continued || !blocks[2].InItem {
		t.Fatalf("marker and continuation wrong: %+v / %+v", blocks[0], blocks[2])
	}
}

func TestImageOpeningAListItemKeepsItsMarker(t *testing.T) {
	blocks := parseIn(t, "1. ![a](p.png)\n", "")
	if got, want := kinds(blocks), []string{"L:", "I:p.png|a"}; fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("got %q, want %q", got, want)
	}
	if !blocks[0].Ordered || blocks[0].Number != 1 {
		t.Fatalf("marker = %+v", blocks[0])
	}
}

func TestImageInAQuoteKeepsTheQuote(t *testing.T) {
	for _, b := range parseIn(t, "> text ![a](p.png) more\n", "") {
		if b.Quote != 1 {
			t.Fatalf("%+v is not in the quote", b)
		}
	}
}

func TestImageInATableCellStaysText(t *testing.T) {
	blocks := parseIn(t, "| h |\n|---|\n| ![a](p.png) |\n", "")
	if len(blocks) != 1 || blocks[0].Kind != Table || PlainText(blocks[0].Rows[1].Cells[0].Spans) != "a" {
		t.Fatalf("blocks = %+v", blocks)
	}
}

// TestSplittingLosesNoText places images at random positions and checks that
// the text around them survives exactly once.
func TestSplittingLosesNoText(t *testing.T) {
	random := rand.New(rand.NewSource(1))
	words := strings.Fields("alpha beta gamma delta epsilon zeta eta theta iota kappa")
	image := regexp.MustCompile(`!\[[^\]]*\]\([^)]*\)`)
	for round := 0; round < 200; round++ {
		var parts []string
		for i := 0; i < 3+random.Intn(8); i++ {
			if random.Intn(3) == 0 {
				parts = append(parts, fmt.Sprintf("![alt%d](i%d.png)", i, i))
			} else {
				parts = append(parts, words[random.Intn(len(words))])
			}
		}
		source := strings.Join(parts, " ")
		var text []string
		for _, b := range parseIn(t, source+"\n", "") {
			if b.Kind != Image {
				text = append(text, strings.Fields(PlainText(b.Spans))...)
			}
		}
		want := strings.Fields(image.ReplaceAllString(source, " "))
		if strings.Join(text, " ") != strings.Join(want, " ") {
			t.Fatalf("source %q: text %q, want %q", source, text, want)
		}
	}
}
