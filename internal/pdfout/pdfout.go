// SPDX-License-Identifier: MIT

// Package pdfout provides an fpdf implementation of render.Canvas.
package pdfout

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/draw"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"math"
	"os"
	"strconv"
	"strings"

	"github.com/gnutterts/md2pdf/internal/font"
	"github.com/gnutterts/md2pdf/internal/markdown"
	"github.com/gnutterts/md2pdf/internal/render"
	"github.com/go-pdf/fpdf"
)

// DefaultMargin is the margin on every side in points.
const DefaultMargin = 64.0

// Layout is the paper size and margin of a document.
type Layout struct {
	Paper  string  // a3, a4, a5, letter or legal; empty means a4
	Margin float64 // points on every side; 0 means DefaultMargin
	// Font and MonoFont are directories with Regular.ttf and optionally
	// Bold.ttf, Italic.ttf and BoldItalic.ttf, used instead of the embedded
	// DejaVu fonts for text and for code. Empty means the embedded font.
	Font     string
	MonoFont string
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
	indent float64 // the indentation in use: requested, but never wider than maxIndent
	// requested is the sum of all Indent calls, so that indenting back is exact even
	// when the indentation in use was capped.
	requested float64
	images    int

	// outline holds the heading levels of the bookmarks that are still open; its
	// length is the outline depth of the next bookmark.
	outline []int
	// fonts are the family and style keys registered with fpdf so far.
	fonts map[string]bool
	// fontDir and monoDir are Layout.Font and Layout.MonoFont.
	fontDir, monoDir string
	// headings are all headings drawn so far, for a table of contents.
	headings []Heading

	font    fontStyle
	fontSet bool
	// lineSize is the largest font size chosen since the last line break or new
	// page, starting from the font size then in use; the
	// height of a wrapped line follows it, not the smaller size of an inline span.
	lineSize float64
	// fresh is true from a line break or a new page until the first Style or
	// Text of the next line: that Style sets lineSize instead of raising it.
	fresh  bool
	strike bool
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
	d := &Document{pdf: pdf, margin: margin, width: width, fonts: map[string]bool{}, fontDir: layout.Font, monoDir: layout.MonoFont}
	// Start with a UTF-8 font, so that fpdf treats every string as UTF-8.
	d.setFont("Helvetica", "", 11)
	return d
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
		d.setFont("Helvetica", "", 9)
		pdf.SetTextColor(128, 128, 128)
		pdf.CellFormat(d.width-2*d.margin, 10, fmt.Sprintf("%d / {nb}", pdf.PageNo()), "", 0, "C", false, 0, "")
	})
}

// setFont selects a font. The names of the PDF core fonts that the renderer
// uses map to the embedded families: Helvetica to DejaVu Sans, Courier to
// DejaVu Sans Mono. A family and style are registered with fpdf on first use,
// so a document only parses the fonts it draws with. U and S in style are
// underline and strike-through, which fpdf draws itself.
func (d *Document) setFont(family, style string, size float64) {
	name, dir := font.Sans, d.fontDir
	if strings.EqualFold(family, "Courier") || family == font.Mono {
		name, dir = font.Mono, d.monoDir
	}
	if dir != "" {
		name += "Custom" // a different family, so it never mixes with the embedded one
	}
	face := ""
	if strings.Contains(style, "B") {
		face += "B"
	}
	if strings.Contains(style, "I") {
		face += "I"
	}
	if key := name + face; !d.fonts[key] {
		d.fonts[key] = true
		var data []byte
		var err error
		if dir != "" {
			data, err = font.FromDir(dir, face)
		} else {
			data, err = font.Bytes(name, face)
		}
		if err != nil {
			d.pdf.SetError(err)
			return
		}
		d.pdf.AddUTF8FontFromBytes(name, face, data)
	}
	d.pdf.SetFont(name, style, size)
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

func (v documentCanvas) NewPage() {
	v.d.pdf.AddPage()
	v.d.lineSize, v.d.fresh = v.d.font.size, true
	v.resetX()
}
func (v documentCanvas) Style(family string, bold, italic bool, size float64) {
	style := ""
	if bold {
		style += "B"
	}
	if italic {
		style += "I"
	}
	v.d.font = fontStyle{family: family, style: style, size: size}
	if v.d.fresh {
		v.d.lineSize, v.d.fresh = size, false
	} else {
		v.d.lineSize = max(v.d.lineSize, size)
	}
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
		// Not NewPage: the heading style is already chosen and must keep its line height.
		d.pdf.AddPage()
		v.resetX()
	}
	for len(d.outline) > 0 && d.outline[len(d.outline)-1] >= level {
		d.outline = d.outline[:len(d.outline)-1]
	}
	// The current font is always a UTF-8 font, so fpdf writes the title as UTF-16.
	d.pdf.Bookmark(title, len(d.outline), -1)
	d.outline = append(d.outline, level)
	d.headings = append(d.headings, Heading{Text: title, Level: level, Page: d.pdf.PageNo(), Y: d.pdf.GetY()})
}

// lineHeight is the height of a line of the block that is being written.
func (v documentCanvas) lineHeight() float64 {
	if v.d.lineSize <= 0 {
		return render.LineHeight(0)
	}
	return render.LineHeight(v.d.lineSize)
}
func (v documentCanvas) Text(s string) {
	v.d.fresh = false
	v.d.pdf.Write(v.lineHeight(), s)
}
func (v documentCanvas) Link(s, url string) {
	r, g, b := v.d.pdf.GetTextColor()
	v.d.pdf.SetTextColor(0, 70, 160)
	if v.d.fontSet {
		v.d.setFont(v.d.font.family, v.styleString()+"U", v.d.font.size)
	}
	v.d.fresh = false
	v.d.pdf.WriteLinkString(v.lineHeight(), s, url)
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
	v.d.setFont(v.d.font.family, v.styleString(), v.d.font.size)
}
func (v documentCanvas) LineBreak(h float64) {
	v.d.pdf.Ln(h)
	v.d.lineSize, v.d.fresh = v.d.font.size, true
	v.resetX()
}

// minTextWidth is the text width that deep nesting never takes away.
const minTextWidth = 120.0

// maxIndent is the widest indentation that still leaves minTextWidth of text.
func (d *Document) maxIndent() float64 {
	return max(d.width-2*d.margin-minTextWidth, 0)
}

func (v documentCanvas) Indent(p float64) {
	v.d.requested = max(v.d.requested+p, 0)
	v.d.indent = min(v.d.requested, v.d.maxIndent())
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
	v.d.pdf.Write(15, s)
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
		pdf.Write(15, prefix)
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
		x := v.d.margin + min(14*float64(level-1), max(v.d.maxIndent()-14, 0)) + 4 // stays left of the capped text
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
		for _, piece := range wrapCode(line, limit) {
			v.d.pdf.CellFormat(v.contentWidth(), 12, piece, "", 0, "", false, 0, "")
			v.LineBreak(12)
		}
	}
}

// wrapCode splits a code line into pieces of at most limit bytes. It prefers to
// break right after the last space within limit, but only when that space falls
// in the second half of limit; otherwise it breaks hard at limit. limit counts
// characters, which all have the same width in a monospaced font.
func wrapCode(line string, limit int) []string {
	if limit < 1 {
		limit = 1
	}
	if line == "" {
		return []string{""}
	}
	runes := []rune(line)
	var pieces []string
	for len(runes) > limit {
		cut := limit
		for space := limit - 1; space*2 >= limit; space-- {
			if runes[space] == ' ' {
				cut = space + 1
				break
			}
		}
		pieces = append(pieces, string(runes[:cut]))
		runes = runes[cut:]
	}
	return append(pieces, string(runes))
}

// Diagram registers and draws a PNG without a temporary file. The PNG was
// rendered at scale times the size it has on the page; a scale of 0 means 1.
func (v documentCanvas) Diagram(png []byte, scale float64) error {
	if scale <= 0 {
		scale = 1
	}
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
	return v.placeImage(name, options, info.Width()/scale, info)
}

// placeImage draws a registered image at most width points wide and no wider
// than the text, keeping its proportions, and starts a new page when it does not
// fit on this one.
func (v documentCanvas) placeImage(name string, options fpdf.ImageOptions, width float64, info *fpdf.ImageInfoType) error {
	width = min(width, v.contentWidth())
	height := info.Height() * width / info.Width()
	// An image taller than a whole page would run over the edge and be
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

// Limits on an image file, so that a huge picture cannot exhaust memory.
const (
	maxImageFile   = 64 << 20
	maxImagePixels = 40_000_000
)

// Image draws a PNG, JPEG or GIF file at its natural size (96 pixels to the
// inch), but no wider than the text. The file is checked before fpdf sees it,
// and an image fpdf cannot read directly is converted first, so that a bad
// picture never leaves fpdf in its sticky error state.
func (v documentCanvas) Image(path string) error {
	pdf := v.d.pdf
	if err := pdf.Error(); err != nil {
		return err
	}
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return errors.New("not found")
		}
		return err
	}
	if info.IsDir() {
		return errors.New("is a directory")
	}
	if info.Size() > maxImageFile {
		return fmt.Errorf("file is too large (%.1f MiB, limit %d MiB)", float64(info.Size())/(1<<20), maxImageFile>>20)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return errors.New("unsupported format (PNG, JPEG and GIF can be drawn)")
	}
	if config.Width <= 0 || config.Height <= 0 || config.Width > maxImagePixels/config.Height {
		return fmt.Errorf("image of %dx%d pixels is too large", config.Width, config.Height)
	}
	v.d.images++
	name := fmt.Sprintf("image-%d", v.d.images)
	options := fpdf.ImageOptions{ImageType: strings.ToUpper(format)}
	registered := pdf.RegisterImageOptionsReader(name, options, bytes.NewReader(data))
	if pdf.Error() != nil {
		// fpdf rejects some valid files, such as interlaced or 16-bit PNGs:
		// decode them here and hand fpdf a plain 8-bit PNG instead.
		pdf.ClearError()
		converted, convertErr := plainPNG(data)
		if convertErr != nil {
			return fmt.Errorf("cannot read image: %w", convertErr)
		}
		v.d.images++
		name = fmt.Sprintf("image-%d", v.d.images)
		options = fpdf.ImageOptions{ImageType: "PNG"}
		registered = pdf.RegisterImageOptionsReader(name, options, bytes.NewReader(converted))
		if err := pdf.Error(); err != nil {
			pdf.ClearError()
			return fmt.Errorf("cannot read image: %w", err)
		}
	}
	if registered == nil || registered.Width() <= 0 || registered.Height() <= 0 {
		return errors.New("cannot read image")
	}
	return v.placeImage(name, options, float64(config.Width)*72/96, registered)
}

// plainPNG decodes an image and encodes it as a non-interlaced 8-bit PNG.
func plainPNG(data []byte) ([]byte, error) {
	decoded, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	bounds := decoded.Bounds()
	plain := image.NewNRGBA(bounds)
	draw.Draw(plain, bounds, decoded, bounds.Min, draw.Src)
	var out bytes.Buffer
	if err := png.Encode(&out, plain); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
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
	v.d.setFont(family, style, size)
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
	return v.d.pdf.GetStringWidth(run.text)
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

// KeepWithNext starts a new page when height no longer fits, unless nothing has
// been drawn on the page yet.
func (v documentCanvas) KeepWithNext(height float64) bool {
	_, top, _, _ := v.d.pdf.GetMargins()
	if v.rowFits(height) || v.d.pdf.GetY() <= top+0.01 {
		return false
	}
	v.NewPage()
	return true
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
			runWidth := pdf.GetStringWidth(run.text)
			if run.url != "" {
				pdf.SetTextColor(0, 70, 160)
			}
			// CellFormat supplies the baseline and the underline/strike drawing.
			// Its margin is compensated so text starts at start exactly.
			pdf.SetXY(start-tablePadding/2, y+float64(i)*tableLineHeight)
			pdf.CellFormat(runWidth+tablePadding, tableLineHeight, run.text, "", 0, "", false, 0, "")
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

// Heading is a heading as it was placed: its text, its level (1 to 6), the page
// it is on and its distance from the top of that page.
type Heading struct {
	Text  string
	Level int
	Page  int
	Y     float64
}

// Headings returns the headings drawn so far, in document order.
func (d *Document) Headings() []Heading { return append([]Heading(nil), d.headings...) }

// PageCount is the number of pages so far.
func (d *Document) PageCount() int { return d.pdf.PageCount() }

// TOCTitle is the heading of the table of contents.
const TOCTitle = "Contents"

// TOC draws a table of contents of entries at the current position: a title,
// then one line per entry with its text, indented by level, and its page
// number, both clickable. offset is added to every page number, for the pages
// the table itself takes. It ends on a new page, ready for the document.
func (d *Document) TOC(entries []Heading, offset int) {
	pdf := d.pdf
	canvas := documentCanvas{d: d}
	canvas.Style("Helvetica", true, false, 20)
	canvas.Text(TOCTitle)
	canvas.LineBreak(27)
	canvas.LineBreak(6)
	top := 6
	for _, entry := range entries {
		top = min(top, entry.Level)
	}
	const lineHeight = 15.0
	d.setFont("Helvetica", "", 11)
	for _, entry := range entries {
		if !canvas.rowFits(lineHeight) {
			pdf.AddPage()
			canvas.resetX()
		}
		indent := 14 * float64(entry.Level-top)
		page := entry.Page + offset
		number := strconv.Itoa(page)
		numberWidth := pdf.GetStringWidth(number) + 2*pdf.GetCellMargin()
		textWidth := canvas.contentWidth() - indent - numberWidth - 12
		link := pdf.AddLink()
		pdf.SetLink(link, entry.Y, page)
		x, y := d.margin+indent, pdf.GetY()
		pdf.SetXY(x, y)
		pdf.CellFormat(textWidth, lineHeight, fitText(pdf, entry.Text, textWidth-2*pdf.GetCellMargin()), "", 0, "L", false, link, "")
		pdf.SetXY(d.margin+canvas.contentWidth()-numberWidth, y)
		pdf.CellFormat(numberWidth, lineHeight, number, "", 0, "R", false, link, "")
		pdf.SetXY(d.margin, y+lineHeight)
	}
	canvas.NewPage()
}

// fitText shortens s with an ellipsis until it is at most width wide.
func fitText(pdf *fpdf.Fpdf, s string, width float64) string {
	if pdf.GetStringWidth(s) <= width {
		return s
	}
	const ellipsis = "…"
	runes := []rune(s)
	for len(runes) > 0 && pdf.GetStringWidth(string(runes)+ellipsis) > width {
		runes = runes[:len(runes)-1]
	}
	s = string(runes)
	return strings.TrimRight(s, " ") + ellipsis
}

// TOCPages is the number of pages a table of contents of entries takes with
// this layout. Every entry is one line, shortened when it is too long, so the
// page numbers do not change it.
func TOCPages(layout Layout, entries []Heading) int {
	scratch := NewWithLayout(layout)
	scratch.TOC(entries, 0)
	return scratch.pdf.PageNo() - 1
}
