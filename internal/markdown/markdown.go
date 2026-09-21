// SPDX-License-Identifier: MIT

// Package markdown converts Markdown to a small, independent document model.
package markdown

import (
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
	Quote
	Rule
	Table
)

// Span is a formatted inline fragment.
type Span struct {
	Text   string
	Bold   bool
	Italic bool
	Code   bool
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
	Kind     Kind
	Level    int
	Depth    int
	Ordered  bool
	Number   int
	Task     TaskState
	Language string
	Spans    []Span
	Lines    []string
	Rows     []Row
}

// Parse reads Markdown into a flat document structure.
func Parse(source []byte) ([]Block, error) {
	md := goldmark.New(goldmark.WithExtensions(extension.GFM))
	doc := md.Parser().Parse(text.NewReader(source))
	var blocks []Block
	walkChildren(doc, source, &blocks, 0, false, false, 0)
	return blocks, nil
}

func walkChildren(parent ast.Node, source []byte, out *[]Block, depth int, inList, ordered bool, number int) {
	for kind := parent.FirstChild(); kind != nil; kind = kind.NextSibling() {
		switch n := kind.(type) {
		case *ast.Heading:
			*out = append(*out, Block{Kind: Heading, Level: n.Level, Spans: spans(n, source, false, false, false, "")})
		case *ast.Paragraph, *ast.TextBlock:
			kind := Paragraph
			if inList {
				kind = ListItem
			}
			block := Block{Kind: kind, Depth: depth, Ordered: ordered, Number: number, Spans: spans(n, source, false, false, false, "")}
			if inList {
				block.Task = taskState(n)
				if block.Task != NoTask && len(block.Spans) > 0 {
					block.Spans[0].Text = strings.TrimPrefix(block.Spans[0].Text, " ")
				}
			}
			*out = append(*out, block)
		case *ast.List:
			listDepth := depth
			if inList {
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
					walkChildren(li, source, out, listDepth, true, n.IsOrdered(), number)
				}
			}
		case *ast.FencedCodeBlock:
			*out = append(*out, codeBlock(n, source, string(n.Language(source))))
		case *ast.CodeBlock:
			*out = append(*out, codeBlock(n, source, ""))
		case *ast.Blockquote:
			for line := n.FirstChild(); line != nil; line = line.NextSibling() {
				*out = append(*out, Block{Kind: Quote, Spans: spans(line, source, false, true, false, "")})
			}
		case *ast.ThematicBreak:
			*out = append(*out, Block{Kind: Rule})
		case *extast.Table:
			*out = append(*out, table(n, source))
		}
	}
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
	var out []Span
	var loop func(ast.Node, bool, bool, bool, string)
	appendSpan := func(text string, v, c, co bool, u string) {
		if text == "" {
			return
		}
		if len(out) > 0 && out[len(out)-1].Bold == v && out[len(out)-1].Italic == c && out[len(out)-1].Code == co && out[len(out)-1].URL == u {
			out[len(out)-1].Text += text
			return
		}
		out = append(out, Span{Text: text, Bold: v, Italic: c, Code: co, URL: u})
	}
	loop = func(k ast.Node, v, c, co bool, u string) {
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
			appendSpan(text, v, c, co, u)
		case *ast.String:
			appendSpan(string(x.Value), v, c, co, u)
		case *ast.CodeSpan:
			appendSpan(string(x.Text(source)), v, c, true, u)
		case *ast.AutoLink:
			label := string(x.Label(source))
			linkURL := string(x.URL(source))
			if x.AutoLinkType == ast.AutoLinkEmail {
				linkURL = "mailto:" + label
			}
			appendSpan(label, v, c, co, linkURL)
		case *ast.Emphasis:
			for q := x.FirstChild(); q != nil; q = q.NextSibling() {
				loop(q, v || x.Level >= 2, c || x.Level == 1 || x.Level == 3, co, u)
			}
		case *ast.Link:
			for q := x.FirstChild(); q != nil; q = q.NextSibling() {
				loop(q, v, c, co, string(x.Destination))
			}
		default:
			for q := k.FirstChild(); q != nil; q = q.NextSibling() {
				loop(q, v, c, co, u)
			}
		}
	}
	for kind := n.FirstChild(); kind != nil; kind = kind.NextSibling() {
		loop(kind, bold, italic, code, url)
	}
	return out
}
