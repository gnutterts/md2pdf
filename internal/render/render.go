// SPDX-License-Identifier: MIT

// Package render draws the document model on a Canvas.
package render

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/gnutterts/md2pdf/internal/markdown"
	"github.com/gnutterts/md2pdf/internal/mermaid"
)

// Canvas is the abstract drawing surface for a PDF document.
type Canvas interface {
	NewPage()
	Style(family string, bold, italic bool, size float64)
	// Strike turns strike-through on or off for the text that follows.
	Strike(on bool)
	Text(text string)
	Link(text, url string)
	LineBreak(height float64)
	Indent(punten float64)
	// Marker draws a list marker in the gutter before the current left
	// margin, and leaves x at the left margin or just after the marker if
	// the marker is wider than the gutter.
	Marker(text string)
	// Checkbox draws a task checkbox in the gutter before the current left
	// margin. prefix, when non-empty, is drawn before the box. checked adds
	// a check mark inside the box. It leaves x at the left margin or just
	// after the box if the box is wider than the gutter.
	Checkbox(prefix string, checked bool)
	// Quote draws the blocks that draw() produces with a vertical bar in the
	// gutter of every quote level, also across page breaks.
	Quote(levels int, draw func())
	// HangingIndent aligns wrapped lines with the current column
	// instead of the indentation of the block, so that the text of a
	// list item continues below itself and not below the bullet.
	HangingIndent()
	CodeBlock(lines []string)
	Rule()
	// Bookmark adds an entry to the outline of the PDF at the current position.
	Bookmark(text string, level int)
	// Table draws a complete table: the canvas determines column widths and
	// page breaks, because only there are the font metrics known.
	Table(rows []markdown.Row)
	// Diagram draws a PNG at the full text width, preserving
	// proportions, and starts on a new page when it no longer fits.
	Diagram(png []byte) error
	Err() error
}

const lineHeight = 15.0

type baseStyle struct {
	family       string
	size         float64
	bold, italic bool
}

// LineHeight is the height of a line of text of the given font size in points.
func LineHeight(size float64) float64 {
	height := math.Round(size*1.35*2) / 2
	return math.Max(lineHeight, height)
}

// Options optionally determines how diagrams are rendered.
type Options struct {
	Mermaid mermaid.Renderer
	Warn    func(message string)
}

// Draw draws blocks in their original order.
func Draw(blocks []markdown.Block, canvas Canvas, options Options) error {
	first := true
	for i := 0; i < len(blocks); {
		block := blocks[i]
		if block.Quote > 0 {
			level := block.Quote
			j := i
			for j < len(blocks) && blocks[j].Quote == level {
				j++
			}
			run := blocks[i:j]
			canvas.Quote(level, func() {
				for k, b := range run {
					drawBlock(canvas, b, options, &first, listNeedsSpace(blocks, i+k))
				}
			})
			i = j
			continue
		}
		drawBlock(canvas, block, options, &first, listNeedsSpace(blocks, i))
		i++
	}
	return canvas.Err()
}

// drawBlock draws one block and marks it as handled.
func drawBlock(canvas Canvas, block markdown.Block, options Options, first *bool, spaceAfter bool) {
	switch block.Kind {
	case markdown.Heading:
		if !*first {
			canvas.LineBreak(12)
		}
		base := baseStyle{family: "Helvetica", size: 11, bold: true}
		switch block.Level {
		case 5:
			base.italic = true
		case 6:
			base.bold, base.italic = false, true
		}
		if block.Level == 1 {
			base.size = 20
		}
		if block.Level == 2 {
			base.size = 16
		}
		if block.Level == 3 {
			base.size = 13
		}
		indent := continuationIndent(block)
		canvas.Indent(indent)
		canvas.Style(base.family, base.bold, base.italic, base.size)
		canvas.Bookmark(markdown.PlainText(block.Spans), block.Level)
		spans(canvas, block.Spans, base)
		canvas.LineBreak(LineHeight(base.size))
		canvas.Indent(-indent)
		canvas.LineBreak(6)
	case markdown.Paragraph:
		base := baseStyle{family: "Helvetica", size: 11, italic: block.Quote > 0}
		indent := continuationIndent(block)
		canvas.Indent(indent)
		canvas.Style(base.family, base.bold, base.italic, base.size)
		spans(canvas, block.Spans, base)
		canvas.LineBreak(LineHeight(base.size))
		canvas.Indent(-indent)
		canvas.LineBreak(6)
	case markdown.ListItem:
		indent := listTextIndent(block)
		canvas.Indent(indent)
		base := baseStyle{family: "Helvetica", size: 11, italic: block.Quote > 0}
		canvas.Style(base.family, base.bold, base.italic, base.size)
		if !block.Continued {
			switch block.Task {
			case markdown.TaskOpen:
				canvas.Checkbox(taskPrefix(block), false)
			case markdown.TaskDone:
				canvas.Checkbox(taskPrefix(block), true)
			default:
				marker := "• "
				if block.Ordered {
					marker = strconv.Itoa(block.Number) + ". "
				}
				canvas.Marker(marker)
			}
			canvas.HangingIndent()
		}
		spans(canvas, block.Spans, base)
		canvas.LineBreak(LineHeight(base.size))
		canvas.Indent(-indent)
		if spaceAfter {
			canvas.LineBreak(6)
		}
	case markdown.CodeBlock:
		if block.Language == "mermaid" && options.Mermaid.Available() {
			indent := continuationIndent(block)
			canvas.Indent(indent)
			png, err := options.Mermaid.ToPNG(strings.Join(block.Lines, "\n"))
			if err == nil {
				err = canvas.Diagram(png)
			}
			if err == nil {
				canvas.LineBreak(6)
				canvas.Indent(-indent)
				break
			}
			canvas.Indent(-indent)
			warn(options, fmt.Sprintf("could not draw mermaid diagram: %v", err))
		}
		indent := continuationIndent(block)
		canvas.Indent(indent)
		drawCodeBlock(canvas, block.Lines)
		canvas.Indent(-indent)
	case markdown.Rule:
		indent := continuationIndent(block)
		canvas.Indent(indent)
		canvas.Rule()
		canvas.Indent(-indent)
		canvas.LineBreak(6)
	case markdown.Table:
		indent := continuationIndent(block)
		canvas.Indent(indent)
		canvas.Table(block.Rows)
		canvas.Indent(-indent)
		canvas.LineBreak(6)
	}
	*first = false
}

// listTextIndent is the distance from the left margin to the text of a list item.
func listTextIndent(block markdown.Block) float64 {
	return float64(block.Depth+1)*14 + float64(block.Quote)*14
}

// continuationIndent is the extra indentation of a block inside a quote or a list item.
func continuationIndent(block markdown.Block) float64 {
	indent := float64(block.Quote) * 14
	if block.InItem {
		indent += float64(block.Depth+1) * 14
	}
	return indent
}

// taskPrefix is the text before the checkbox of an ordered task item.
func taskPrefix(block markdown.Block) string {
	if block.Ordered {
		return strconv.Itoa(block.Number) + ". "
	}
	return ""
}

// listNeedsSpace reports whether a list item must be followed by extra space.
func listNeedsSpace(blocks []markdown.Block, i int) bool {
	if blocks[i].Kind != markdown.ListItem {
		return false
	}
	if i+1 >= len(blocks) {
		return true
	}
	next := blocks[i+1]
	if next.Quote != blocks[i].Quote {
		return true
	}
	if next.Kind != markdown.ListItem {
		return !next.InItem
	}
	// A marker item directly followed by another marker item of a
	// different kind (bulleted vs numbered) at the same depth starts a
	// separate list, which gets the same space a paragraph would get.
	return !blocks[i].Continued && !next.Continued && !next.InItem &&
		next.Depth == blocks[i].Depth && next.Ordered != blocks[i].Ordered
}

func drawCodeBlock(canvas Canvas, lines []string) {
	canvas.Indent(10)
	canvas.Style("Courier", false, false, 9.5)
	canvas.CodeBlock(lines)
	canvas.Indent(-10)
	canvas.LineBreak(6)
}

func warn(options Options, message string) {
	if options.Warn != nil {
		options.Warn(message)
	}
}

func spans(canvas Canvas, spans []markdown.Span, base baseStyle) {
	for _, span := range spans {
		canvas.Strike(span.Strike)
		family, size := base.family, base.size
		if span.Code {
			family = "Courier"
			size = math.Round(base.size*0.85*2) / 2
		}
		canvas.Style(family, base.bold || span.Bold, base.italic || span.Italic, size)
		if span.URL != "" {
			canvas.Link(span.Text, span.URL)
		} else {
			canvas.Text(span.Text)
		}
	}
	canvas.Strike(false)
}
