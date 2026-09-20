// Package pdfout provides an fpdf implementation of render.Canvas.
package pdfout

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/gnutterts/md2pdf/internal/markdown"
	"github.com/gnutterts/md2pdf/internal/render"
	"github.com/gnutterts/md2pdf/internal/text"
	"github.com/go-pdf/fpdf"
)

const (
	margin = 56.0
	width  = 595.28
)

// Document is an A4 PDF with a drawing surface.
type Document struct {
	pdf    *fpdf.Fpdf
	indent float64
	images int
}

// New creates an empty A4 document.
func New() *Document {
	pdf := fpdf.New("P", "pt", "A4", "")
	pdf.SetMargins(margin, margin, margin)
	pdf.SetAutoPageBreak(true, margin)
	pdf.AddPage()
	return &Document{pdf: pdf}
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
	v.d.pdf.SetFont(family, style, size)
}
func (v documentCanvas) Text(s string)       { v.d.pdf.Write(15, text.ToCP1252(s)) }
func (v documentCanvas) Link(s, url string)  { v.d.pdf.WriteLinkString(15, text.ToCP1252(s), url) }
func (v documentCanvas) LineBreak(h float64) { v.d.pdf.Ln(h); v.resetX() }
func (v documentCanvas) Indent(p float64) {
	v.d.indent += p
	v.d.pdf.SetLeftMargin(margin + v.d.indent)
	v.resetX()
}
func (v documentCanvas) HangingIndent() { v.d.pdf.SetLeftMargin(v.d.pdf.GetX()) }

func (v documentCanvas) CodeBlock(lines []string) {
	for _, line := range lines {
		line = text.ToCP1252(line)
		for len(line) > 0 && v.d.pdf.GetStringWidth(line) > v.contentWidth() {
			line = line[:len(line)-1]
		}
		v.d.pdf.CellFormat(v.contentWidth(), 12, line, "", 0, "", false, 0, "")
		v.LineBreak(12)
	}
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
		return errors.New("ongeldige PNG voor diagram")
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

// cellText joins the spans of a cell so that nothing is lost.
func cellText(cell markdown.Cell) string {
	var b strings.Builder
	for _, span := range cell.Spans {
		b.WriteString(span.Text)
	}
	return b.String()
}

// setCellFont chooses the font for a cell: bold for the header row, normal for data.
func (v documentCanvas) setCellFont(header bool) {
	style := ""
	if header {
		style = "B"
	}
	v.d.pdf.SetFont("Helvetica", style, tableFontSize)
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
			v.setCellFont(row.Header)
			cellWidth := v.d.pdf.GetStringWidth(text.ToCP1252(cellText(row.Cells[column]))) + tablePadding
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

// linesFor wraps cell content at word boundaries to fit within the column width.
// The full column width is passed in: fpdf.SplitLines already subtracts the
// cell margin twice (wmax = w - 2*cMargin), so subtracting it here again wraps too early.
func (v documentCanvas) linesFor(cell markdown.Cell, width float64) []string {
	text := text.ToCP1252(cellText(cell))
	if text == "" {
		return nil
	}
	lines := v.d.pdf.SplitLines([]byte(text), width)
	out := make([]string, len(lines))
	for i, line := range lines {
		out[i] = string(line)
	}
	return out
}

// rowHeight is the height of the tallest cell, with a minimum of 16 points.
func (v documentCanvas) rowHeight(row markdown.Row, widths []float64) float64 {
	height := tableMinHeight
	for column, cell := range row.Cells {
		if column >= len(widths) {
			break
		}
		v.setCellFont(row.Header)
		lines := v.linesFor(cell, widths[column])
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
	x := margin + v.d.indent
	y := v.d.pdf.GetY()
	for column, cell := range row.Cells {
		if column >= len(widths) {
			break
		}
		v.drawCell(cell, row.Header, x, y, widths[column], height)
		x += widths[column]
	}
	v.d.pdf.SetXY(margin+v.d.indent, y+height)
}

// drawCell draws the background, border, and text of one cell.
func (v documentCanvas) drawCell(cell markdown.Cell, header bool, x, y, width, height float64) {
	pdf := v.d.pdf
	v.setCellFont(header)
	if header {
		pdf.SetFillColor(tableGray, tableGray, tableGray)
		pdf.Rect(x, y, width, height, "FD")
	} else {
		pdf.Rect(x, y, width, height, "D")
	}
	lines := v.linesFor(cell, width)
	for i, line := range lines {
		pdf.SetXY(x, y+float64(i)*tableLineHeight)
		pdf.CellFormat(width, tableLineHeight, line, "", 0, alignment(cell.Alignment), false, 0, "")
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
	v.d.pdf.Line(margin+v.d.indent, y, width-margin, y)
}
func (v documentCanvas) Err() error            { return v.d.pdf.Error() }
func (v documentCanvas) resetX()               { v.d.pdf.SetX(margin + v.d.indent) }
func (v documentCanvas) contentWidth() float64 { return width - 2*margin - v.d.indent }
