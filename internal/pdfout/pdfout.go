// SPDX-License-Identifier: MIT

// Package pdfout provides an fpdf implementation of render.Canvas.
package pdfout

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"os"
	"strings"
	"unicode/utf16"

	"github.com/gnutterts/md2pdf/internal/markdown"
	"github.com/gnutterts/md2pdf/internal/render"
	"github.com/gnutterts/md2pdf/internal/text"
	"github.com/go-pdf/fpdf"
)

// DefaultMargin is the margin on every side in points.
const DefaultMargin = 64.0

// Layout is the paper size and margin of a document.
type Layout struct {
	Paper  string  // a3, a4, a5, letter or legal; empty means a4
	Margin float64 // points on every side; 0 means DefaultMargin
}

// paperSizes are the page sizes in points that fpdf knows, portrait.
var paperSizes = map[string][2]float64{
	"a3":     {841.89, 1190.55},
	"a4":     {595.28, 841.89},
	"a5":     {419.53, 595.28},
	"letter": {612, 792},
	"legal":  {612, 1008},
}

// PaperSize returns the portrait size in points of a paper name, ignoring case.
func PaperSize(name string) (width, height float64, ok bool) {
	size, ok := paperSizes[strings.ToLower(name)]
	return size[0], size[1], ok
}

// Document is an A4 PDF with a drawing surface.
type Document struct {
	pdf    *fpdf.Fpdf
	margin float64
	width  float64 // page width in points
	indent float64
	images int

	// outline holds the heading levels of the bookmarks that are still open; its
	// length is the outline depth of the next bookmark.
	outline []int

	font    fontStyle
	fontSet bool
	// lineSize is the largest font size chosen since the last line break; the
	// height of a wrapped line follows it, not the smaller size of an inline span.
	lineSize float64
	strike   bool
}

// fontStyle remembers the last font chosen through Canvas.Style, without
// strike-through, which Document keeps separately.
type fontStyle struct {
	family string
	style  string
	size   float64
}

// New creates an empty A4 document with the default margin.
func New() *Document { return NewWithLayout(Layout{}) }

// NewWithLayout creates an empty document with the given paper size and margin.
func NewWithLayout(layout Layout) *Document {
	paper := strings.ToLower(layout.Paper)
	if _, _, ok := PaperSize(paper); !ok {
		paper = "a4"
	}
	margin := layout.Margin
	if margin <= 0 {
		margin = DefaultMargin
	}
	pdf := fpdf.New("P", "pt", paper, "")
	pdf.SetMargins(margin, margin, margin)
	pdf.SetAutoPageBreak(true, margin)
	pdf.AddPage()
	width, _ := pdf.GetPageSize()
	return &Document{pdf: pdf, margin: margin, width: width}
}

// SetInfo sets the document information; empty values are left unset.
func (d *Document) SetInfo(title, author, creator string) {
	if title != "" {
		d.pdf.SetTitle(title, true)
	}
	if author != "" {
		d.pdf.SetAuthor(author, true)
	}
	if creator != "" {
		d.pdf.SetCreator(creator, true)
	}
}

// SetPageNumbers puts "n / N" centred in grey below the text of every page.
func (d *Document) SetPageNumbers(on bool) {
	if !on {
		d.pdf.SetFooterFunc(nil)
		return
	}
	d.pdf.AliasNbPages("")
	// fpdf saves and restores the font (with underline), the colours and the
	// line width around the footer, so nothing is restored here.
	d.pdf.SetFooterFunc(func() {
		pdf := d.pdf
		pdf.SetXY(d.margin, -d.margin/2-4)
		pdf.SetFont("Helvetica", "", 9)
		pdf.SetTextColor(128, 128, 128)
		pdf.CellFormat(d.width-2*d.margin, 10, fmt.Sprintf("%d / {nb}", pdf.PageNo()), "", 0, "C", false, 0, "")
	})
}

// Canvas returns the drawing surface of the document.
func (d *Document) Canvas() render.Canvas { return documentCanvas{d: d} }

// Write writes the document to path.
func (d *Document) Write(path string) error {
	if err := d.pdf.Error(); err != nil {
		return err
	}
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	outputErr := d.pdf.Output(file)
	closeErr := file.Close()
	return errors.Join(outputErr, closeErr)
}

type documentCanvas struct{ d *Document }

func (v documentCanvas) NewPage() { v.d.pdf.AddPage(); v.resetX() }
func (v documentCanvas) Style(family string, bold, italic bool, size float64) {
	style := ""
	if bold {
		style += "B"
	}
	if italic {
		style += "I"
	}
	v.d.font = fontStyle{family: family, style: style, size: size}
	v.d.lineSize = max(v.d.lineSize, size)
	v.d.fontSet = true
	v.applyFont()
}
func (v documentCanvas) Strike(on bool) {
	v.d.strike = on
	v.applyFont()
}

// Bookmark adds an outline entry for a heading. The outline depth is the
// number of open headings of a lower level, so levels are contiguous whatever
// the heading levels do (a skipped level, a document that starts at level 3,
// or a merged file that starts again at level 1). The entry is placed on the
// page where the heading text starts: when the heading no longer fits on the
// current page, a new page is started first.
func (v documentCanvas) Bookmark(title string, level int) {
	if title == "" {
		return
	}
	d := v.d
	if !v.rowFits(math.Max(15, d.font.size*1.35)) {
		v.NewPage()
	}
	for len(d.outline) > 0 && d.outline[len(d.outline)-1] >= level {
		d.outline = d.outline[:len(d.outline)-1]
	}
	// fpdf converts the title to UTF-16 only for UTF-8 fonts. The core fonts
	// used now would send it as PDFDocEncoding, which differs from cp1252 for
	// the characters 0x80 to 0x9F, so the title is encoded here. When an
	// embedded UTF-8 font is used, pass the title unchanged instead.
	d.pdf.Bookmark(utf16Title(title), len(d.outline), -1)
	d.outline = append(d.outline, level)
}

// utf16Title encodes text as UTF-16BE with a byte order mark.
func utf16Title(text string) string {
	units := utf16.Encode([]rune(text))
	encoded := make([]byte, 0, 2+2*len(units))
	encoded = append(encoded, 0xfe, 0xff)
	for _, unit := range units {
		encoded = append(encoded, byte(unit>>8), byte(unit))
	}
	return string(encoded)
}

// lineHeight is the height of a line of the block that is being written.
func (v documentCanvas) lineHeight() float64 {
	if v.d.lineSize <= 0 {
		return render.LineHeight(0)
	}
	return render.LineHeight(v.d.lineSize)
}
func (v documentCanvas) Text(s string) { v.d.pdf.Write(v.lineHeight(), text.ToCP1252(s)) }
func (v documentCanvas) Link(s, url string) {
	r, g, b := v.d.pdf.GetTextColor()
	v.d.pdf.SetTextColor(0, 70, 160)
	if v.d.fontSet {
		v.d.pdf.SetFont(v.d.font.family, v.styleString()+"U", v.d.font.size)
	}
	v.d.pdf.WriteLinkString(v.lineHeight(), text.ToCP1252(s), url)
	if v.d.fontSet {
		v.applyFont()
	}
	v.d.pdf.SetTextColor(r, g, b)
}
func (v documentCanvas) styleString() string {
	style := v.d.font.style
	if v.d.strike {
		style += "S"
	}
	return style
}
func (v documentCanvas) applyFont() {
	if !v.d.fontSet {
		return
	}
	v.d.pdf.SetFont(v.d.font.family, v.styleString(), v.d.font.size)
}
func (v documentCanvas) LineBreak(h float64) { v.d.pdf.Ln(h); v.d.lineSize = 0; v.resetX() }
func (v documentCanvas) Indent(p float64) {
	v.d.indent += p
	v.d.pdf.SetLeftMargin(v.d.margin + v.d.indent)
	v.resetX()
}
func (v documentCanvas) HangingIndent() { v.d.pdf.SetLeftMargin(v.d.pdf.GetX()) }

// Marker draws a list marker in the gutter before the current left margin,
// and leaves x at the left margin or just after the marker if the marker is
// wider than the gutter.
func (v documentCanvas) Marker(s string) {
	const gutter = 14.0
	left := v.d.margin + v.d.indent
	v.d.pdf.SetX(left - gutter)
	v.d.pdf.Write(15, text.ToCP1252(s))
	if v.d.pdf.GetX() < left {
		v.d.pdf.SetX(left)
	}
}

// Checkbox draws a task checkbox in the gutter before the current left margin.
func (v documentCanvas) Checkbox(prefix string, checked bool) {
	const gutter = 14.0
	const boxSize = 8.0
	const boxGap = 3.0
	left := v.d.margin + v.d.indent
	pdf := v.d.pdf
	pdf.SetX(left - gutter)
	if prefix != "" {
		pdf.Write(15, text.ToCP1252(prefix))
	}
	x := pdf.GetX()
	// GetY is the top of the 15 point text line; centre the box on it.
	top := pdf.GetY() + (15-boxSize)/2
	pdf.Rect(x, top, boxSize, boxSize, "D")
	if checked {
		pdf.Line(x+1.5, top+4.5, x+3.5, top+6.5)
		pdf.Line(x+3.5, top+6.5, x+6.5, top+2.5)
	}
	pdf.SetX(x + boxSize + boxGap)
	if pdf.GetX() < left {
		pdf.SetX(left)
	}
}

// Quote draws the blocks that draw() produces with a vertical bar in the
// gutter of every quote level, also across page breaks.
func (v documentCanvas) Quote(levels int, draw func()) {
	if levels <= 0 {
		draw()
		return
	}
	pdf := v.d.pdf
	drawRed, drawGreen, drawBlue := pdf.GetDrawColor()
	lineWidth := pdf.GetLineWidth()
	startPage := pdf.PageNo()
	startY := pdf.GetY()

	draw()

	endPage := pdf.PageNo()
	endY := pdf.GetY()
	_, pageHeight := pdf.GetPageSize()
	_, top, _, bottom := pdf.GetMargins()

	for level := 1; level <= levels; level++ {
		x := v.d.margin + 14*float64(level-1) + 4
		for page := startPage; page <= endPage; page++ {
			pdf.SetPage(page)
			pdf.SetDrawColor(180, 180, 180)
			pdf.SetLineWidth(1.5)
			y1 := startY
			if page > startPage {
				y1 = top
			}
			y2 := endY
			if page < endPage {
				y2 = pageHeight - bottom
			}
			pdf.Line(x, y1, x, y2)
		}
	}

	pdf.SetPage(endPage)
	pdf.SetDrawColor(drawRed, drawGreen, drawBlue)
	pdf.SetLineWidth(lineWidth)
}

func (v documentCanvas) CodeBlock(lines []string) {
	limit := int(v.contentWidth() / v.d.pdf.GetStringWidth("M"))
	for _, line := range lines {
		line = text.ToCP1252(line)
		for _, piece := range wrapCode(line, limit) {
			v.d.pdf.CellFormat(v.contentWidth(), 12, piece, "", 0, "", false, 0, "")
			v.LineBreak(12)
		}
	}
}

// wrapCode splits a code line into pieces of at most limit bytes. It prefers to
// break right after the last space within limit, but only when that space falls
// in the second half of limit; otherwise it breaks hard at limit. The line must
// already be cp1252, where every byte is one character.
func wrapCode(line string, limit int) []string {
	if limit < 1 {
		limit = 1
	}
	if line == "" {
		return []string{""}
	}
	var pieces []string
	for len(line) > limit {
		cut := limit
		if space := strings.LastIndexByte(line[:limit], ' '); space >= 0 && space*2 >= limit {
			cut = space + 1
		}
		pieces = append(pieces, line[:cut])
		line = line[cut:]
	}
	pieces = append(pieces, line)
	return pieces
}

// Diagram registers and draws a PNG without a temporary file.
func (v documentCanvas) Diagram(png []byte) error {
	v.d.images++
	name := fmt.Sprintf("diagram-%d.png", v.d.images)
	options := fpdf.ImageOptions{ImageType: "PNG"}
	info := v.d.pdf.RegisterImageOptionsReader(name, options, bytes.NewReader(png))
	if err := v.d.pdf.Error(); err != nil {
		return err
	}
	if info == nil || info.Width() <= 0 || info.Height() <= 0 {
		return errors.New("invalid PNG for diagram")
	}
	width := min(info.Width(), v.contentWidth())
	height := info.Height() * width / info.Width()
	// A diagram taller than a whole page would run over the edge and be
	// clipped; then the page height determines the scale, not the width.
	if maxHeight := v.pageContentHeight(); height > maxHeight {
		height = maxHeight
		width = info.Width() * height / info.Height()
	}
	if !v.rowFits(height) {
		v.NewPage()
	}
	v.d.pdf.ImageOptions(name, v.d.pdf.GetX(), v.d.pdf.GetY(), width, height, true, options, 0, "")
	if err := v.d.pdf.Error(); err != nil {
		return err
	}
	v.resetX()
	return nil
}

const (
	tableFontSize   = 10.0
	tableLineHeight = 14.0
	tableMinHeight  = 16.0
	tableMinWidth   = 40.0
	tablePadding    = 8.0
	tableBorder     = 0.4
	tableGray       = 230
)

// Table draws a complete table with header row, column widths, and page breaks.
func (v documentCanvas) Table(rows []markdown.Row) {
	columns := columnCount(rows)
	if columns == 0 {
		return
	}
	pdf := v.d.pdf

	autoBreak, breakMargin := pdf.GetAutoPageBreak()
	cellMargin := pdf.GetCellMargin()
	lineWidth := pdf.GetLineWidth()
	tr, tg, tb := pdf.GetDrawColor()
	fr, fg, fb := pdf.GetFillColor()
	txr, txg, txb := pdf.GetTextColor()

	pdf.SetAutoPageBreak(false, breakMargin)
	pdf.SetCellMargin(tablePadding / 2)
	pdf.SetLineWidth(tableBorder)
	pdf.SetDrawColor(0, 0, 0)
	pdf.SetTextColor(0, 0, 0)

	defer func() {
		pdf.SetAutoPageBreak(autoBreak, breakMargin)
		pdf.SetCellMargin(cellMargin)
		pdf.SetLineWidth(lineWidth)
		pdf.SetDrawColor(tr, tg, tb)
		pdf.SetFillColor(fr, fg, fb)
		pdf.SetTextColor(txr, txg, txb)
		v.applyFont()
		v.resetX()
	}()

	widths := v.columnWidths(rows, columns)
	headerIndex := -1
	if len(rows) > 0 && rows[0].Header {
		headerIndex = 0
	}

	for i := range rows {
		height := v.rowHeight(rows[i], widths)
		if !v.rowFits(height) {
			pdf.AddPage()
			v.resetX()
			if i != headerIndex && headerIndex >= 0 {
				v.drawRow(rows[headerIndex], widths)
			}
		}
		v.drawRow(rows[i], widths)
	}
}

// columnCount counts the columns of the widest row.
func columnCount(rows []markdown.Row) int {
	columns := 0
	for _, row := range rows {
		if len(row.Cells) > columns {
			columns = len(row.Cells)
		}
	}
	return columns
}

// cellRun is a piece of cell text with one visual style.
type cellRun struct {
	text               string
	bold, italic, code bool
	strike             bool
	url                string
}

type cellLine []cellRun

// setRunFont selects a run's font. Header cells add bold to every run.
func (v documentCanvas) setRunFont(run cellRun, header bool) {
	family, size := "Helvetica", tableFontSize
	if run.code {
		family = "Courier"
		size = math.Round(tableFontSize*0.85*2) / 2
	}
	style := ""
	if header || run.bold {
		style += "B"
	}
	if run.italic {
		style += "I"
	}
	if run.strike {
		style += "S"
	}
	if run.url != "" {
		style += "U"
	}
	v.d.pdf.SetFont(family, style, size)
}

func sameRun(a, b cellRun) bool {
	return a.bold == b.bold && a.italic == b.italic && a.code == b.code &&
		a.strike == b.strike && a.url == b.url
}

func appendRun(line *cellLine, run cellRun) {
	if run.text == "" {
		return
	}
	if n := len(*line); n > 0 && sameRun((*line)[n-1], run) {
		(*line)[n-1].text += run.text
		return
	}
	*line = append(*line, run)
}

func (v documentCanvas) runWidth(run cellRun, header bool) float64 {
	v.setRunFont(run, header)
	return v.d.pdf.GetStringWidth(text.ToCP1252(run.text))
}

func (v documentCanvas) lineWidth(line cellLine, header bool) float64 {
	width := 0.0
	for _, run := range line {
		width += v.runWidth(run, header)
	}
	return width
}

// cellLines wraps styled runs at spaces. A word wider than the cell is split
// character by character, retaining the style of every character.
func (v documentCanvas) cellLines(cell markdown.Cell, width float64, header bool) []cellLine {
	available := width - tablePadding
	lines := []cellLine{{}}
	pending := cellLine(nil)
	hasText := false

	flushWord := func(word cellLine) {
		if len(word) == 0 {
			return
		}
		line := &lines[len(lines)-1]
		candidate := v.lineWidth(*line, header) + v.lineWidth(pending, header) + v.lineWidth(word, header)
		if len(*line) > 0 && candidate > available {
			lines = append(lines, cellLine{})
			line = &lines[len(lines)-1]
			pending = nil
		}
		wordWidth := v.lineWidth(word, header)
		if wordWidth <= available {
			for _, run := range pending {
				appendRun(line, run)
			}
			for _, run := range word {
				appendRun(line, run)
			}
			pending = nil
			return
		}
		// The word cannot fit on an empty line, so split it into characters.
		pending = nil
		for _, run := range word {
			for _, char := range run.text {
				piece := run
				piece.text = string(char)
				if len(*line) > 0 && v.lineWidth(*line, header)+v.runWidth(piece, header) > available {
					lines = append(lines, cellLine{})
					line = &lines[len(lines)-1]
				}
				appendRun(line, piece)
			}
		}
	}

	word := cellLine(nil)
	flush := func() { flushWord(word); word = nil }
	for _, span := range cell.Spans {
		run := cellRun{text: "", bold: span.Bold, italic: span.Italic, code: span.Code, strike: span.Strike, url: span.URL}
		for _, char := range span.Text {
			hasText = true
			switch char {
			case '\n':
				flush()
				pending = nil
				lines = append(lines, cellLine{})
			case ' ':
				flush()
				run.text = " "
				appendRun(&pending, run)
			default:
				run.text = string(char)
				appendRun(&word, run)
			}
		}
	}
	flush()
	if len(pending) > 0 {
		for _, run := range pending {
			appendRun(&lines[len(lines)-1], run)
		}
	}
	if !hasText {
		return nil
	}
	return lines
}

// columnWidths measures the widest cell per column and scales down when needed.
func (v documentCanvas) columnWidths(rows []markdown.Row, columns int) []float64 {
	widths := make([]float64, columns)
	for column := 0; column < columns; column++ {
		width := tableMinWidth
		for _, row := range rows {
			if column >= len(row.Cells) {
				continue
			}
			cellWidth := v.lineWidth(cellRuns(row.Cells[column]), row.Header) + tablePadding
			if cellWidth > width {
				width = cellWidth
			}
		}
		widths[column] = width
	}

	sum := 0.0
	for _, width := range widths {
		sum += width
	}
	if sum > v.contentWidth() {
		factor := v.contentWidth() / sum
		for i := range widths {
			widths[i] *= factor
		}
	}
	return widths
}

// cellRuns converts spans to runs for width calculations.
func cellRuns(cell markdown.Cell) cellLine {
	var runs cellLine
	for _, span := range cell.Spans {
		appendRun(&runs, cellRun{
			text: span.Text, bold: span.Bold, italic: span.Italic,
			code: span.Code, strike: span.Strike, url: span.URL,
		})
	}
	return runs
}

// rowHeight is the height of the tallest cell, with a minimum of 16 points.
func (v documentCanvas) rowHeight(row markdown.Row, widths []float64) float64 {
	height := tableMinHeight
	for column, cell := range row.Cells {
		if column >= len(widths) {
			break
		}
		lines := v.cellLines(cell, widths[column], row.Header)
		cellHeight := float64(len(lines)) * tableLineHeight
		if cellHeight > height {
			height = cellHeight
		}
	}
	return height
}

// pageContentHeight is the height available for content on one page.
func (v documentCanvas) pageContentHeight() float64 {
	_, pageHeight := v.d.pdf.GetPageSize()
	_, top, _, bottom := v.d.pdf.GetMargins()
	return pageHeight - top - bottom
}

// rowFits reports whether a row still fits on the current page.
func (v documentCanvas) rowFits(height float64) bool {
	_, pageHeight := v.d.pdf.GetPageSize()
	_, _, _, bottom := v.d.pdf.GetMargins()
	return v.d.pdf.GetY()+height <= pageHeight-bottom
}

// drawRow draws one row at the current position.
func (v documentCanvas) drawRow(row markdown.Row, widths []float64) {
	height := v.rowHeight(row, widths)
	x := v.d.margin + v.d.indent
	y := v.d.pdf.GetY()
	for column, cell := range row.Cells {
		if column >= len(widths) {
			break
		}
		v.drawCell(cell, row.Header, x, y, widths[column], height)
		x += widths[column]
	}
	v.d.pdf.SetXY(v.d.margin+v.d.indent, y+height)
}

// drawCell draws the background, border, and text of one cell.
func (v documentCanvas) drawCell(cell markdown.Cell, header bool, x, y, width, height float64) {
	pdf := v.d.pdf
	if header {
		pdf.SetFillColor(tableGray, tableGray, tableGray)
		pdf.Rect(x, y, width, height, "FD")
	} else {
		pdf.Rect(x, y, width, height, "D")
	}
	lines := v.cellLines(cell, width, header)
	for i, line := range lines {
		textWidth := v.lineWidth(line, header)
		start := x + tablePadding/2
		switch alignment(cell.Alignment) {
		case "C":
			start += (width - tablePadding - textWidth) / 2
		case "R":
			start += width - tablePadding - textWidth
		}
		for _, run := range line {
			v.setRunFont(run, header)
			runWidth := pdf.GetStringWidth(text.ToCP1252(run.text))
			if run.url != "" {
				pdf.SetTextColor(0, 70, 160)
			}
			// CellFormat supplies the baseline and the underline/strike drawing.
			// Its margin is compensated so text starts at start exactly.
			pdf.SetXY(start-tablePadding/2, y+float64(i)*tableLineHeight)
			pdf.CellFormat(runWidth+tablePadding, tableLineHeight, text.ToCP1252(run.text), "", 0, "", false, 0, "")
			if run.url != "" {
				pdf.LinkString(start, y+float64(i)*tableLineHeight, runWidth, tableLineHeight, run.url)
				pdf.SetTextColor(0, 0, 0)
			}
			start += runWidth
		}
	}
}

// alignment translates the Markdown alignment to the fpdf letter codes.
func alignment(u markdown.Alignment) string {
	switch u {
	case markdown.AlignCenter:
		return "C"
	case markdown.AlignRight:
		return "R"
	default:
		return "L"
	}
}
func (v documentCanvas) Rule() {
	y := v.d.pdf.GetY() + 3
	v.d.pdf.Line(v.d.margin+v.d.indent, y, v.d.width-v.d.margin, y)
}
func (v documentCanvas) Err() error            { return v.d.pdf.Error() }
func (v documentCanvas) resetX()               { v.d.pdf.SetX(v.d.margin + v.d.indent) }
func (v documentCanvas) contentWidth() float64 { return v.d.width - 2*v.d.margin - v.d.indent }
