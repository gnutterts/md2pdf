// SPDX-License-Identifier: MIT

// Package render draws the document model on a Canvas.
package render

import (
	"fmt"
	"math"
	"runtime"
	"strconv"
	"strings"
	"sync"

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
	// scale is how many times larger than its size on the page the PNG was rendered.
	Diagram(png []byte, scale float64) error
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
	// Diagram renders the text of a diagram to a PNG. When nil, Mermaid.ToPNG is used;
	// NewDiagramCache makes a version that renders every distinct text once.
	Diagram func(text string) ([]byte, error)
	// Strict turns every warning into an error: Draw stops at the first one and
	// returns it.
	Strict bool
}

// drawState is what drawing remembers between blocks.
type drawState struct {
	first bool
	err   error // the first warning when Options.Strict is set
}

// Draw draws blocks in their original order.
func Draw(blocks []markdown.Block, canvas Canvas, options Options) error {
	state := &drawState{first: true}
	for i := 0; i < len(blocks) && state.err == nil; {
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
					if state.err != nil {
						return
					}
					drawBlock(canvas, b, options, state, listNeedsSpace(blocks, i+k))
				}
			})
			i = j
			continue
		}
		drawBlock(canvas, block, options, state, listNeedsSpace(blocks, i))
		i++
	}
	if state.err != nil {
		return state.err
	}
	return canvas.Err()
}

// drawBlock draws one block and marks it as handled.
func drawBlock(canvas Canvas, block markdown.Block, options Options, state *drawState, spaceAfter bool) {
	switch block.Kind {
	case markdown.Heading:
		if !state.first {
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
			produce := options.Diagram
			if produce == nil {
				produce = options.Mermaid.ToPNG
			}
			png, err := produce(strings.Join(block.Lines, "\n"))
			if err == nil {
				err = canvas.Diagram(png, options.Mermaid.Scale)
			}
			if err == nil {
				canvas.LineBreak(6)
				canvas.Indent(-indent)
				break
			}
			canvas.Indent(-indent)
			state.warn(options, fmt.Sprintf("could not draw mermaid diagram: %v", err))
			if state.err != nil {
				return // strict: do not draw the fallback either
			}
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
	state.first = false
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

// warn reports a warning; with Options.Strict it also becomes the error of Draw.
func (state *drawState) warn(options Options, message string) {
	if options.Strict && state.err == nil {
		state.err = fmt.Errorf("%s (--strict)", message)
	}
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

type diagramEntry struct {
	once sync.Once
	png  []byte
	err  error
}

// NewDiagramCache wraps produce so that every distinct diagram text is
// rendered once, also when several goroutines ask for the same text at the same
// time: the others wait for the first. Failures are remembered too: a broken
// diagram does not cost its time limit again, although every place it appears
// still gets a warning.
func NewDiagramCache(produce func(text string) ([]byte, error)) func(text string) ([]byte, error) {
	var mutex sync.Mutex
	entries := map[string]*diagramEntry{}
	return func(text string) ([]byte, error) {
		mutex.Lock()
		entry, ok := entries[text]
		if !ok {
			entry = &diagramEntry{}
			entries[text] = entry
		}
		mutex.Unlock()
		entry.once.Do(func() { entry.png, entry.err = safely(produce, text) })
		return entry.png, entry.err
	}
}

// safely calls produce and turns a panic into an error, so that a failing
// renderer in a background goroutine cannot bring the program down.
func safely(produce func(string) ([]byte, error), text string) (png []byte, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			png, err = nil, fmt.Errorf("diagram renderer failed: %v", recovered)
		}
	}()
	return produce(text)
}

// DiagramTexts returns the distinct Mermaid diagram texts of blocks in
// document order.
func DiagramTexts(blocks []markdown.Block) []string {
	seen := map[string]bool{}
	var texts []string
	for _, block := range blocks {
		if block.Kind != markdown.CodeBlock || block.Language != "mermaid" {
			continue
		}
		text := strings.Join(block.Lines, "\n")
		if !seen[text] {
			seen[text] = true
			texts = append(texts, text)
		}
	}
	return texts
}

// Workers is the number of diagrams rendered at the same time.
func Workers() int { return max(1, min(runtime.NumCPU(), 4)) }

// Prerender renders texts through get with at most workers at a time. get is
// meant to be a NewDiagramCache function: the results are kept there, and
// drawing later only reads them. Errors are not returned here; drawing reports
// them where each diagram appears.
func Prerender(get func(string) ([]byte, error), texts []string, workers int) {
	if workers < 1 {
		workers = 1
	}
	slots := make(chan struct{}, workers)
	var wait sync.WaitGroup
	for _, text := range texts {
		wait.Add(1)
		slots <- struct{}{}
		go func(text string) {
			defer func() { <-slots; wait.Done() }()
			_, _ = safely(get, text)
		}(text)
	}
	wait.Wait()
}
