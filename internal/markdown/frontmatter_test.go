// SPDX-License-Identifier: MIT

package markdown

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
)

func TestSplitFrontMatterYAML(t *testing.T) {
	tests := []struct {
		name     string
		source   string
		want     Metadata
		wantRest string
	}{
		{
			name:     "simple fields skip list and nested map",
			source:   "---\ntitle: \"My title\"\nauthor: 'Jane Doe'\ntags:\n  - one\n  - two\nnested:\n  key: value\n---\n# Body\n",
			want:     Metadata{"title": "My title", "author": "Jane Doe"},
			wantRest: "# Body\n",
		},
		{
			name:     "closed with dots",
			source:   "---\ntitle: T\n...\nbody\n",
			want:     Metadata{"title": "T"},
			wantRest: "body\n",
		},
		{
			name:     "trailing spaces after open",
			source:   "---   \ntitle: T\n---\nbody\n",
			want:     Metadata{"title": "T"},
			wantRest: "body\n",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			meta, rest := SplitFrontMatter([]byte(test.source))
			if !reflect.DeepEqual(meta, test.want) {
				t.Fatalf("SplitFrontMatter metadata = %v, want %v", meta, test.want)
			}
			if string(rest) != test.wantRest {
				t.Fatalf("SplitFrontMatter rest = %q, want %q", rest, test.wantRest)
			}
		})
	}
}

func TestParseYAMLFrontMatterLeavesKeysOut(t *testing.T) {
	source := []byte("---\ntitle: \"My title\"\nauthor: 'Jane Doe'\ntags:\n  - one\nnested:\n  key: value\n---\n# Body\n")
	blocks, err := Parse(source)
	if err != nil {
		t.Fatal(err)
	}
	for _, span := range collectSpans(blocks) {
		for _, key := range []string{"title", "author", "tags", "nested"} {
			if strings.Contains(span.Text, key) {
				t.Fatalf("span %q contains front matter key %q", span.Text, key)
			}
		}
	}
}

func TestSplitFrontMatterUnchanged(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{"yaml without close", "---\ntitle: T\n"},
		{"toml without close", "+++\ntitle = \"T\"\n"},
		{"dashes on second line", "\n---\n"},
		{"plain markdown", "# Title\n"},
		{"empty", ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			source := []byte(test.source)
			meta, rest := SplitFrontMatter(source)
			if meta == nil {
				t.Fatal("SplitFrontMatter returned nil metadata")
			}
			if len(meta) != 0 {
				t.Fatalf("metadata = %v, want empty", meta)
			}
			if !bytes.Equal(rest, source) {
				t.Fatalf("rest = %q, want %q", rest, source)
			}
		})
	}
}

func TestParseDashesOnSecondLineStayARule(t *testing.T) {
	blocks, err := Parse([]byte("\n---\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(blocks) != 1 || blocks[0].Kind != Rule {
		t.Fatalf("blocks = %v, want a rule", blocks)
	}
}

func TestSplitFrontMatterTOML(t *testing.T) {
	source := "+++\ntitle = \"My title\"\nauthor = 'Jane Doe'\n[table]\nkey = \"ignored\"\n+++\nbody\n"
	meta, rest := SplitFrontMatter([]byte(source))
	want := Metadata{"title": "My title", "author": "Jane Doe"}
	if !reflect.DeepEqual(meta, want) {
		t.Fatalf("SplitFrontMatter metadata = %v, want %v", meta, want)
	}
	if string(rest) != "body\n" {
		t.Fatalf("SplitFrontMatter rest = %q, want %q", rest, "body\n")
	}
}

func TestSplitFrontMatterPandoc(t *testing.T) {
	tests := []struct {
		name     string
		source   string
		want     Metadata
		wantRest string
	}{
		{
			name:     "three lines",
			source:   "% Title\n% Author\n% Date\nbody\n",
			want:     Metadata{"title": "Title", "author": "Author", "date": "Date"},
			wantRest: "body\n",
		},
		{
			name:     "title only",
			source:   "% Title\nbody\n",
			want:     Metadata{"title": "Title"},
			wantRest: "body\n",
		},
		{
			name:     "empty author line",
			source:   "% Title\n%\n% Date\nbody\n",
			want:     Metadata{"title": "Title", "date": "Date"},
			wantRest: "body\n",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			meta, rest := SplitFrontMatter([]byte(test.source))
			if !reflect.DeepEqual(meta, test.want) {
				t.Fatalf("SplitFrontMatter metadata = %v, want %v", meta, test.want)
			}
			if string(rest) != test.wantRest {
				t.Fatalf("SplitFrontMatter rest = %q, want %q", rest, test.wantRest)
			}
		})
	}
}

func TestSplitFrontMatterCRLF(t *testing.T) {
	source := "---\r\ntitle: \"T\"\r\n---\r\nbody\r\n"
	meta, rest := SplitFrontMatter([]byte(source))
	want := Metadata{"title": "T"}
	if !reflect.DeepEqual(meta, want) {
		t.Fatalf("SplitFrontMatter metadata = %v, want %v", meta, want)
	}
	if string(rest) != "body\r\n" {
		t.Fatalf("SplitFrontMatter rest = %q, want %q", rest, "body\r\n")
	}
}

func TestSplitFrontMatterBOM(t *testing.T) {
	source := "\uFEFF---\ntitle: T\n---\nbody\n"
	meta, rest := SplitFrontMatter([]byte(source))
	want := Metadata{"title": "T"}
	if !reflect.DeepEqual(meta, want) {
		t.Fatalf("SplitFrontMatter metadata = %v, want %v", meta, want)
	}
	if string(rest) != "body\n" {
		t.Fatalf("SplitFrontMatter rest = %q, want %q", rest, "body\n")
	}
}

func TestParseFrontMatterMatchesBody(t *testing.T) {
	with := []byte("---\ntitle: T\nauthor: A\n---\n# Heading\n\nText\n")
	without := []byte("# Heading\n\nText\n")
	got, err := Parse(with)
	if err != nil {
		t.Fatal(err)
	}
	want, err := Parse(without)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Parse with front matter = %v, want %v", got, want)
	}
}

func TestParseLeadingRulesKeepTheTextBetween(t *testing.T) {
	source := "---\n# Title\n\nSome text.\n\n---\n\nMore text.\n"
	meta, body := SplitFrontMatter([]byte(source))
	if len(meta) != 0 || string(body) != source {
		t.Fatalf("meta = %v, body = %q, want the source unchanged", meta, body)
	}
	blocks, err := Parse([]byte(source))
	if err != nil {
		t.Fatal(err)
	}
	var headings int
	for _, block := range blocks {
		if block.Kind == Heading {
			headings++
		}
	}
	if headings != 1 {
		t.Fatalf("want the heading between the rules to survive, got %v", blocks)
	}
}

func TestSplitFrontMatterClosingFenceWithTrailingSpaces(t *testing.T) {
	meta, body := SplitFrontMatter([]byte("---\ntitle: A\n---  \nBody\n"))
	if meta["title"] != "A" || string(body) != "Body\n" {
		t.Fatalf("meta = %v, body = %q", meta, body)
	}
}

func TestSplitFrontMatterGuards(t *testing.T) {
	tests := []struct{ name, source string }{
		{"toml with prose", "+++\nJust a sentence.\n+++\nBody\n"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			meta, body := SplitFrontMatter([]byte(test.source))
			if len(meta) != 0 || string(body) != test.source {
				t.Fatalf("meta = %v, body = %q, want the source unchanged", meta, body)
			}
		})
	}
}

func TestSplitFrontMatterLenientFences(t *testing.T) {
	tests := []struct{ name, source string }{
		{"tab after yaml fence", "---\t\ntitle: A\n---\t\nBody\n"},
		{"space before colon", "---\ntitle : A\n---\nBody\n"},
		{"toml fence with space", "+++ \ntitle = \"A\"\n+++\nBody\n"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			meta, body := SplitFrontMatter([]byte(test.source))
			if meta["title"] != "A" || string(body) != "Body\n" {
				t.Fatalf("meta = %v, body = %q", meta, body)
			}
		})
	}
}

func TestParseByteOrderMarkWithoutFrontMatter(t *testing.T) {
	blocks, err := Parse([]byte("\xef\xbb\xbfHello"))
	if err != nil {
		t.Fatal(err)
	}
	if len(blocks) != 1 || blocks[0].Spans[0].Text != "Hello" {
		t.Fatalf("blocks = %v, want the text without a byte order mark", blocks)
	}
}

func TestMetadataGetIgnoresCase(t *testing.T) {
	meta, _ := SplitFrontMatter([]byte("---\nTitle: Hello\nauthor: Ann\n---\ntext\n"))
	if meta.Get("title") != "Hello" || meta.Get("AUTHOR") != "Ann" || meta.Get("missing") != "" {
		t.Fatalf("unexpected fields: %v", meta)
	}
}

func TestFirstHeadingUsesPlainTextOfFirstLevelOne(t *testing.T) {
	blocks, err := Parse([]byte("## Second\n\n# The *first* `one`\n\n# Later\n"))
	if err != nil {
		t.Fatal(err)
	}
	if got := FirstHeading(blocks); got != "The first one" {
		t.Fatalf("FirstHeading = %q", got)
	}
	blocks, _ = Parse([]byte("text only\n"))
	if got := FirstHeading(blocks); got != "" {
		t.Fatalf("FirstHeading without heading = %q", got)
	}
}
