// SPDX-License-Identifier: MIT

// Package markdown converts Markdown to a small, independent document model.
package markdown

import (
	"net/url"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	extast "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// Kind is the kind of block in a document.
type Kind int

const (
	Heading Kind = iota
	Paragraph
	ListItem
	CodeBlock
	Rule
	Table
	// Image is a picture from the local file system; Path is resolved against the
	// directory of the source file and Spans hold the alt text.
	Image
)

// Span is a formatted inline fragment.
type Span struct {
	Text   string
	Bold   bool
	Italic bool
	Code   bool
	Strike bool
	URL    string
}

// Alignment is the horizontal alignment of a table cell.
type Alignment int

const (
	AlignLeft Alignment = iota
	AlignCenter
	AlignRight
)

// Cell is one cell of a table.
type Cell struct {
	Spans     []Span
	Alignment Alignment
}

// Row is a row of table cells.
type Row struct {
	Cells  []Cell
	Header bool
}

// TaskState is the state of a task list item checkbox.
type TaskState int

const (
	NoTask TaskState = iota
	TaskOpen
	TaskDone
)

// Block is part of a document.
type Block struct {
	Kind      Kind
	Level     int
	Depth     int
	Ordered   bool
	Number    int
	Task      TaskState
	Language  string
	Continued bool
	InItem    bool
	Quote     int
	Spans     []Span
	Lines     []string
	Rows      []Row
	Path      string
}

// walkParams carries the context in which blocks are walked.
type walkParams struct {
	depth   int
	ordered bool
	number  int
	quote   int
	inList  bool
	first   bool
	inItem  bool
	dir     string // directory of the source file, for image paths
}

// PlainText joins spans into text without formatting.
func PlainText(spans []Span) string {
	var b strings.Builder
	for _, span := range spans {
		b.WriteString(span.Text)
	}
	return strings.TrimSpace(strings.Join(strings.Fields(b.String()), " "))
}

// FirstHeading is the plain text of the first level 1 heading, or empty.
func FirstHeading(blocks []Block) string {
	for _, block := range blocks {
		if block.Kind == Heading && block.Level == 1 {
			if text := PlainText(block.Spans); text != "" {
				return text
			}
		}
	}
	return ""
}

// Parse reads Markdown into a flat document structure.
func Parse(source []byte) ([]Block, error) { return ParseIn(source, "") }

// ParseIn is Parse for a source file in dir: relative image paths are resolved
// against dir.
func ParseIn(source []byte, dir string) ([]Block, error) {
	_, source = SplitFrontMatter(source)
	md := goldmark.New(goldmark.WithExtensions(extension.GFM))
	doc := md.Parser().Parse(text.NewReader(source))
	var blocks []Block
	walkChildren(doc, source, &blocks, walkParams{dir: dir})
	return blocks, nil
}

func walkChildren(parent ast.Node, source []byte, out *[]Block, p walkParams) {
	for kind := parent.FirstChild(); kind != nil; kind = kind.NextSibling() {
		walkBlock(kind, source, out, p)
	}
}

func walkBlock(n ast.Node, source []byte, out *[]Block, p walkParams) {
	switch v := n.(type) {
	case *ast.Heading:
		*out = append(*out, Block{Kind: Heading, Level: v.Level, Depth: p.depth, InItem: p.inItem, Quote: p.quote, Spans: spans(n, source, false, false, false, "")})
	case *ast.Paragraph, *ast.TextBlock:
		if hasLocalImage(n) {
			splitAtImages(n, source, out, p)
			return
		}
		if p.inList {
			block := Block{Kind: ListItem, Depth: p.depth, Ordered: p.ordered, Number: p.number, Quote: p.quote, Spans: spans(n, source, false, false, false, "")}
			if !p.first {
				block.Continued = true
				block.InItem = true
			}
			block.Task = taskState(n)
			if block.Task != NoTask && len(block.Spans) > 0 {
				block.Spans[0].Text = strings.TrimPrefix(block.Spans[0].Text, " ")
			}
			*out = append(*out, block)
			return
		}
		*out = append(*out, Block{Kind: Paragraph, Depth: p.depth, InItem: p.inItem, Quote: p.quote, Spans: spans(n, source, false, false, false, "")})
	case *ast.List:
		walkList(v, source, out, p)
	case *ast.FencedCodeBlock:
		block := codeBlock(v, source, string(v.Language(source)))
		block.Depth = p.depth
		block.InItem = p.inItem
		block.Quote = p.quote
		*out = append(*out, block)
	case *ast.CodeBlock:
		block := codeBlock(v, source, "")
		block.Depth = p.depth
		block.InItem = p.inItem
		block.Quote = p.quote
		*out = append(*out, block)
	case *ast.Blockquote:
		child := p
		child.inList = false
		child.first = false
		child.quote++
		walkChildren(v, source, out, child)
	case *ast.ThematicBreak:
		*out = append(*out, Block{Kind: Rule, Depth: p.depth, InItem: p.inItem, Quote: p.quote})
	case *ast.HTMLBlock:
		keep := func(src string) bool { _, ok := localPath(src, p.dir); return ok }
		for _, part := range htmlParts(htmlBlockText(v, source), keep) {
			if part.src != "" {
				path, _ := localPath(part.src, p.dir)
				image := Block{Kind: Image, Path: path, Depth: p.depth, InItem: p.inItem, Quote: p.quote}
				if part.alt != "" {
					image.Spans = []Span{{Text: part.alt}}
				}
				*out = append(*out, image)
				continue
			}
			*out = append(*out, Block{Kind: Paragraph, Depth: p.depth, InItem: p.inItem, Quote: p.quote, Spans: []Span{{Text: part.text}}})
		}
	case *extast.Table:
		block := table(v, source)
		block.Depth = p.depth
		block.InItem = p.inItem
		block.Quote = p.quote
		*out = append(*out, block)
	}
}

func htmlBlockText(n *ast.HTMLBlock, source []byte) string {
	text := string(n.Lines().Value(source))
	if n.HasClosure() {
		text += string(n.ClosureLine.Value(source))
	}
	return text
}

func walkList(n *ast.List, source []byte, out *[]Block, p walkParams) {
	listDepth := p.depth
	if p.inList {
		listDepth++
	}
	next := n.Start
	for item := n.FirstChild(); item != nil; item = item.NextSibling() {
		if li, ok := item.(*ast.ListItem); ok {
			number := 0
			if n.IsOrdered() {
				number = next
				next++
			}
			walkListItem(li, source, out, walkParams{
				depth:   listDepth,
				ordered: n.IsOrdered(),
				number:  number,
				quote:   p.quote,
				dir:     p.dir,
			})
		}
	}
}

func walkListItem(item *ast.ListItem, source []byte, out *[]Block, p walkParams) {
	marker := false
	if !startsWithText(item) {
		// An item that opens with a code block, a list or nothing still gets its marker.
		*out = append(*out, Block{Kind: ListItem, Depth: p.depth, Ordered: p.ordered, Number: p.number, Quote: p.quote})
		marker = true
	}
	for child := item.FirstChild(); child != nil; child = child.NextSibling() {
		switch child.(type) {
		case *ast.List:
			walkList(child.(*ast.List), source, out, walkParams{depth: p.depth, quote: p.quote, inList: true, dir: p.dir})
		case *ast.Paragraph, *ast.TextBlock:
			childP := walkParams{
				depth:   p.depth,
				ordered: p.ordered,
				number:  p.number,
				quote:   p.quote,
				inList:  true,
				first:   !marker,
				inItem:  marker,
				dir:     p.dir,
			}
			marker = true
			walkBlock(child, source, out, childP)
		default:
			walkBlock(child, source, out, walkParams{depth: p.depth, quote: p.quote, inItem: true, dir: p.dir})
		}
	}
}

func startsWithText(item *ast.ListItem) bool {
	switch item.FirstChild().(type) {
	case *ast.Paragraph, *ast.TextBlock:
		return true
	}
	return false
}

func codeBlock(n ast.Node, source []byte, language string) Block {
	lines := make([]string, 0)
	for i := 0; i < n.Lines().Len(); i++ {
		span := n.Lines().At(i)
		lines = append(lines, strings.TrimSuffix(string(span.Value(source)), "\n"))
	}
	return Block{Kind: CodeBlock, Language: language, Lines: lines}
}

func table(n *extast.Table, source []byte) Block {
	var rows []Row
	for row := n.FirstChild(); row != nil; row = row.NextSibling() {
		var cells []Cell
		for cell := row.FirstChild(); cell != nil; cell = cell.NextSibling() {
			cells = append(cells, Cell{
				Spans:     spans(cell, source, false, false, false, ""),
				Alignment: alignment(cell),
			})
		}
		_, header := row.(*extast.TableHeader)
		rows = append(rows, Row{Cells: cells, Header: header})
	}
	return Block{Kind: Table, Rows: rows}
}

func alignment(cell ast.Node) Alignment {
	tableCell, ok := cell.(*extast.TableCell)
	if !ok {
		return AlignLeft
	}
	switch tableCell.Alignment {
	case extast.AlignRight:
		return AlignRight
	case extast.AlignCenter:
		return AlignCenter
	default:
		return AlignLeft
	}
}

func taskState(n ast.Node) TaskState {
	first := n.FirstChild()
	box, ok := first.(*extast.TaskCheckBox)
	if !ok {
		return NoTask
	}
	if box.IsChecked {
		return TaskDone
	}
	return TaskOpen
}

// resolveText removes backslash escapes and resolves entity and numeric
// references in one pass, so that an escaped ampersand stays literal.
func resolveText(b []byte) []byte {
	var out []byte
	start := 0
	for i := 0; i+1 < len(b); i++ {
		if b[i] == '\\' && util.IsPunct(b[i+1]) {
			out = append(out, resolveReferences(b[start:i])...)
			out = append(out, b[i+1])
			i++
			start = i + 1
		}
	}
	return append(out, resolveReferences(b[start:])...)
}

func resolveReferences(b []byte) []byte {
	return util.ResolveEntityNames(util.ResolveNumericReferences(b))
}

func spans(n ast.Node, source []byte, bold, italic, code bool, url string) []Span {
	var nodes []ast.Node
	for child := n.FirstChild(); child != nil; child = child.NextSibling() {
		nodes = append(nodes, child)
	}
	return spansWith(nodes, source, bold, italic, code, url)
}

// spansOf converts inline nodes without inherited formatting.
func spansOf(nodes []ast.Node, source []byte) []Span {
	return spansWith(nodes, source, false, false, false, "")
}

func spansWith(nodes []ast.Node, source []byte, bold, italic, code bool, url string) []Span {
	var out []Span
	var loop func(ast.Node, bool, bool, bool, bool, string)
	appendSpan := func(text string, v, c, co, s bool, u string) {
		if text == "" {
			return
		}
		if len(out) > 0 && out[len(out)-1].Bold == v && out[len(out)-1].Italic == c && out[len(out)-1].Code == co && out[len(out)-1].Strike == s && out[len(out)-1].URL == u {
			out[len(out)-1].Text += text
			return
		}
		out = append(out, Span{Text: text, Bold: v, Italic: c, Code: co, Strike: s, URL: u})
	}
	loop = func(k ast.Node, v, c, co, s bool, u string) {
		switch x := k.(type) {
		case *ast.Text:
			segment := x.Segment.Value(source)
			if !x.IsRaw() {
				segment = resolveText(segment)
			}
			text := string(segment)
			if x.SoftLineBreak() {
				text += " "
			}
			if x.HardLineBreak() {
				text += "\n"
			}
			appendSpan(text, v, c, co, s, u)
		case *ast.String:
			appendSpan(string(x.Value), v, c, co, s, u)
		case *ast.CodeSpan:
			appendSpan(string(x.Text(source)), v, c, true, s, u)
		case *ast.RawHTML:
			if isBreakTag(x.Segments.Value(source)) {
				appendSpan("\n", v, c, co, s, u)
			}
		case *ast.AutoLink:
			label := string(x.Label(source))
			linkURL := string(x.URL(source))
			if x.AutoLinkType == ast.AutoLinkEmail {
				linkURL = "mailto:" + label
			}
			appendSpan(label, v, c, co, s, linkURL)
		case *ast.Emphasis:
			for q := x.FirstChild(); q != nil; q = q.NextSibling() {
				loop(q, v || x.Level >= 2, c || x.Level == 1 || x.Level == 3, co, s, u)
			}
		case *ast.Link:
			for q := x.FirstChild(); q != nil; q = q.NextSibling() {
				loop(q, v, c, co, s, string(x.Destination))
			}
		case *extast.Strikethrough:
			for q := x.FirstChild(); q != nil; q = q.NextSibling() {
				loop(q, v, c, co, true, u)
			}
		default:
			for q := k.FirstChild(); q != nil; q = q.NextSibling() {
				loop(q, v, c, co, s, u)
			}
		}
	}
	for _, kind := range nodes {
		loop(kind, bold, italic, code, false, url)
	}
	return out
}

// urlScheme matches a URL scheme such as https: or data:; one letter is a Windows drive.
var urlScheme = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9+.-]+:`)

// localImage returns the resolved path of an image that md2pdf can draw from
// the file system; remote images and data URLs are not, and keep their alt text.
func localImage(n ast.Node, dir string) (string, bool) {
	image, ok := n.(*ast.Image)
	if !ok {
		return "", false
	}
	return localPath(string(image.Destination), dir)
}

// localPath resolves an image destination against dir, or reports that it is
// not a local file.
func localPath(destination, dir string) (string, bool) {
	if destination == "" || urlScheme.MatchString(destination) || strings.HasPrefix(destination, "//") {
		return "", false
	}
	if cut := strings.IndexAny(destination, "?#"); cut >= 0 {
		destination = destination[:cut]
	}
	if unescaped, err := url.PathUnescape(destination); err == nil {
		destination = unescaped
	}
	if destination == "" {
		return "", false
	}
	path := filepath.FromSlash(destination)
	if !filepath.IsAbs(path) && dir != "" {
		path = filepath.Join(dir, path)
	}
	return path, true
}

// hasLocalImage reports whether a paragraph holds a local image directly.
func hasLocalImage(n ast.Node) bool {
	for child := n.FirstChild(); child != nil; child = child.NextSibling() {
		if _, ok := localImage(child, ""); ok {
			return true
		}
	}
	return false
}

// splitAtImages turns a paragraph with local images into text · image · text
// blocks. Text blocks keep the list and quote context of the paragraph: in a
// list item the first one carries the marker and the rest continue the item.
func splitAtImages(n ast.Node, source []byte, out *[]Block, p walkParams) {
	first := p.first
	var pending []ast.Node
	flushed := false
	flush := func() {
		spans := trimSpans(spansOf(pending, source))
		pending = nil
		if len(spans) == 0 {
			return
		}
		block := Block{Kind: Paragraph, Depth: p.depth, InItem: p.inItem, Quote: p.quote, Spans: spans}
		if p.inList {
			block = Block{Kind: ListItem, Depth: p.depth, Ordered: p.ordered, Number: p.number, Quote: p.quote, Spans: spans}
			if !first || flushed {
				block.Continued = true
				block.InItem = true
			} else {
				block.Task = taskState(n)
				if block.Task != NoTask {
					block.Spans[0].Text = strings.TrimLeft(block.Spans[0].Text, " ")
				}
			}
		}
		flushed = true
		*out = append(*out, block)
	}
	for child := n.FirstChild(); child != nil; child = child.NextSibling() {
		path, ok := localImage(child, p.dir)
		if !ok {
			pending = append(pending, child)
			continue
		}
		flush()
		if p.inList && first && !flushed {
			// The image opens the item: the marker needs a line of its own.
			*out = append(*out, Block{Kind: ListItem, Depth: p.depth, Ordered: p.ordered, Number: p.number, Quote: p.quote, Task: taskState(n)})
			flushed = true
		}
		inItem := p.inItem || p.inList
		*out = append(*out, Block{Kind: Image, Path: path, Depth: p.depth, InItem: inItem, Quote: p.quote, Spans: spans(child, source, false, false, false, "")})
	}
	flush()
}

// trimSpans removes the white space that separated the text from an image.
func trimSpans(spans []Span) []Span {
	for len(spans) > 0 {
		spans[0].Text = strings.TrimLeft(spans[0].Text, " \t\r\n")
		if spans[0].Text != "" {
			break
		}
		spans = spans[1:]
	}
	for len(spans) > 0 {
		last := len(spans) - 1
		spans[last].Text = strings.TrimRight(spans[last].Text, " \t\r\n")
		if spans[last].Text != "" {
			break
		}
		spans = spans[:last]
	}
	return spans
}
