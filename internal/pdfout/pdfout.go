// Package pdfout biedt een fpdf-implementatie van render.Vel.
package pdfout

import (
	"errors"
	"os"

	"github.com/gnutterts/md2pdf/internal/render"
	"github.com/gnutterts/md2pdf/internal/tekst"
	"github.com/go-pdf/fpdf"
)

const (
	marge   = 56.0
	breedte = 595.28
)

// Document is een A4-PDF met een tekenvlak.
type Document struct {
	pdf         *fpdf.Fpdf
	inspringing float64
}

// Nieuw maakt een leeg A4-document.
func Nieuw() *Document {
	pdf := fpdf.New("P", "pt", "A4", "")
	pdf.SetMargins(marge, marge, marge)
	pdf.SetAutoPageBreak(true, marge)
	pdf.AddPage()
	return &Document{pdf: pdf}
}

// Vel geeft het tekenvlak van het document terug.
func (d *Document) Vel() render.Vel { return documentVel{d: d} }

// Schrijf schrijft het document naar pad.
func (d *Document) Schrijf(pad string) error {
	if err := d.pdf.Error(); err != nil {
		return err
	}
	bestand, err := os.Create(pad)
	if err != nil {
		return err
	}
	uitvoerFout := d.pdf.Output(bestand)
	sluitFout := bestand.Close()
	return errors.Join(uitvoerFout, sluitFout)
}

type documentVel struct{ d *Document }

func (v documentVel) NieuwePagina() { v.d.pdf.AddPage(); v.zetX() }
func (v documentVel) Stijl(familie string, vet, cursief bool, grootte float64) {
	stijl := ""
	if vet {
		stijl += "B"
	}
	if cursief {
		stijl += "I"
	}
	v.d.pdf.SetFont(familie, stijl, grootte)
}
func (v documentVel) Tekst(s string)       { v.d.pdf.Write(15, tekst.NaarCP1252(s)) }
func (v documentVel) Link(s, url string)   { v.d.pdf.WriteLinkString(15, tekst.NaarCP1252(s), url) }
func (v documentVel) Regeleinde(h float64) { v.d.pdf.Ln(h); v.zetX() }
func (v documentVel) Inspringen(p float64) {
	v.d.inspringing += p
	v.d.pdf.SetLeftMargin(marge + v.d.inspringing)
	v.zetX()
}
func (v documentVel) HangendInspringen() { v.d.pdf.SetLeftMargin(v.d.pdf.GetX()) }

func (v documentVel) Codeblok(regels []string) {
	for _, regel := range regels {
		regel = tekst.NaarCP1252(regel)
		for len(regel) > 0 && v.d.pdf.GetStringWidth(regel) > v.tekstbreedte() {
			regel = regel[:len(regel)-1]
		}
		v.d.pdf.CellFormat(v.tekstbreedte(), 12, regel, "", 0, "", false, 0, "")
		v.Regeleinde(12)
	}
}
func (v documentVel) Streep() {
	y := v.d.pdf.GetY() + 3
	v.d.pdf.Line(marge+v.d.inspringing, y, breedte-marge, y)
}
func (v documentVel) Fout() error           { return v.d.pdf.Error() }
func (v documentVel) zetX()                 { v.d.pdf.SetX(marge + v.d.inspringing) }
func (v documentVel) tekstbreedte() float64 { return breedte - 2*marge - v.d.inspringing }
