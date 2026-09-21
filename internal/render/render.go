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
	Text(text string)
	Link(text, url string)
	LineBreak(height float64)
	Indent(punten float64)
	// Marker draws a list marker in the gutter before the current left
	// margin, and leaves x at the left margin or just after the marker if
	// the marker is wider than the gutter.
	Marker(text string)
	// HangingIndent aligns wrapped lines with the current column
	// instead of the indentation of the block, so that the text of a
	// list item continues below itself and not below the bullet.
	HangingIndent()
	CodeBlock(lines []string)
	Rule()
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

func heightFor(size float64) float64 {
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
	for _, block := range blocks {
		switch block.Kind {
		case markdown.Heading:
			if !first {
				canvas.LineBreak(12)
			}
			base := baseStyle{family: "Helvetica", size: 11, bold: true}
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
			spans(canvas, block.Spans, base)
			canvas.LineBreak(heightFor(base.size))
			canvas.Indent(-indent)
			canvas.LineBreak(6)
		case markdown.Paragraph:
			base := baseStyle{family: "Helvetica", size: 11, italic: block.Quote > 0}
			indent := continuationIndent(block)
			canvas.Indent(indent)
			canvas.Style(base.family, base.bold, base.italic, base.size)
			spans(canvas, block.Spans, base)
			canvas.LineBreak(heightFor(base.size))
			canvas.Indent(-indent)
			canvas.LineBreak(6)
		case markdown.ListItem:
			indent := listTextIndent(block)
			canvas.Indent(indent)
			base := baseStyle{family: "Helvetica", size: 11, italic: block.Quote > 0}
			canvas.Style(base.family, base.bold, base.italic, base.size)
			if !block.Continued {
				marker := "• "
				if block.Ordered {
					marker = strconv.Itoa(block.Number) + ". "
				}
				switch block.Task {
				case markdown.TaskOpen:
					marker = taskMarker(block, "[ ] ")
				case markdown.TaskDone:
					marker = taskMarker(block, "[x] ")
				}
				canvas.Marker(marker)
				canvas.HangingIndent()
			}
			spans(canvas, block.Spans, base)
			canvas.LineBreak(heightFor(base.size))
			canvas.Indent(-indent)
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
		first = false
	}
	return canvas.Err()
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

// taskMarker keeps the number of an ordered task item and replaces the bullet
// of an unordered one.
func taskMarker(block markdown.Block, box string) string {
	if block.Ordered {
		return strconv.Itoa(block.Number) + ". " + box
	}
	return box
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
}
