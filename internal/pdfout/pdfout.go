// Package pdfout biedt een fpdf-implementatie van render.Vel.
package pdfout

import (
	"errors"
	"os"
	"strings"

	"github.com/gnutterts/md2pdf/internal/markdown"
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

const (
	tabelFontGrootte = 10.0
	tabelRegelhoogte = 14.0
	tabelMinHoogte   = 16.0
	tabelMinBreedte  = 40.0
	tabelCelvulling  = 8.0
	tabelRand        = 0.4
	tabelGrijs       = 230
)

// Tabel tekent een volledige tabel met kopregel, kolombreedtes en paginabreuken.
func (v documentVel) Tabel(rijen []markdown.Rij) {
	kolommen := aantalKolommen(rijen)
	if kolommen == 0 {
		return
	}
	pdf := v.d.pdf

	autoBreek, breekMarge := pdf.GetAutoPageBreak()
	celmarge := pdf.GetCellMargin()
	lijnbreedte := pdf.GetLineWidth()
	tr, tg, tb := pdf.GetDrawColor()
	fr, fg, fb := pdf.GetFillColor()
	txr, txg, txb := pdf.GetTextColor()

	pdf.SetAutoPageBreak(false, breekMarge)
	pdf.SetCellMargin(tabelCelvulling / 2)
	pdf.SetLineWidth(tabelRand)
	pdf.SetDrawColor(0, 0, 0)
	pdf.SetTextColor(0, 0, 0)

	defer func() {
		pdf.SetAutoPageBreak(autoBreek, breekMarge)
		pdf.SetCellMargin(celmarge)
		pdf.SetLineWidth(lijnbreedte)
		pdf.SetDrawColor(tr, tg, tb)
		pdf.SetFillColor(fr, fg, fb)
		pdf.SetTextColor(txr, txg, txb)
		v.zetX()
	}()

	breedtes := v.kolombreedtes(rijen, kolommen)
	kopIndex := -1
	if len(rijen) > 0 && rijen[0].Kop {
		kopIndex = 0
	}

	for i := range rijen {
		hoogte := v.rijhoogte(rijen[i], breedtes)
		if !v.pastRij(hoogte) {
			pdf.AddPage()
			v.zetX()
			if i != kopIndex && kopIndex >= 0 {
				v.tekenRij(rijen[kopIndex], breedtes)
			}
		}
		v.tekenRij(rijen[i], breedtes)
	}
}

// aantalKolommen telt de kolommen van de breedste rij.
func aantalKolommen(rijen []markdown.Rij) int {
	kolommen := 0
	for _, rij := range rijen {
		if len(rij.Cellen) > kolommen {
			kolommen = len(rij.Cellen)
		}
	}
	return kolommen
}

// celtekst voegt de stukken van een cel samen zodat er niets verloren gaat.
func celtekst(cel markdown.Cel) string {
	var b strings.Builder
	for _, stuk := range cel.Stukken {
		b.WriteString(stuk.Tekst)
	}
	return b.String()
}

// stelCelfontIn kiest het font voor een cel: vet voor de kopregel, normaal voor data.
func (v documentVel) stelCelfontIn(kop bool) {
	stijl := ""
	if kop {
		stijl = "B"
	}
	v.d.pdf.SetFont("Helvetica", stijl, tabelFontGrootte)
}

// kolombreedtes meet de breedste cel per kolom en schaalt zo nodig terug.
func (v documentVel) kolombreedtes(rijen []markdown.Rij, kolommen int) []float64 {
	breedtes := make([]float64, kolommen)
	for kolom := 0; kolom < kolommen; kolom++ {
		breedte := tabelMinBreedte
		for _, rij := range rijen {
			if kolom >= len(rij.Cellen) {
				continue
			}
			v.stelCelfontIn(rij.Kop)
			celbreedte := v.d.pdf.GetStringWidth(tekst.NaarCP1252(celtekst(rij.Cellen[kolom]))) + tabelCelvulling
			if celbreedte > breedte {
				breedte = celbreedte
			}
		}
		breedtes[kolom] = breedte
	}

	som := 0.0
	for _, breedte := range breedtes {
		som += breedte
	}
	if som > v.tekstbreedte() {
		factor := v.tekstbreedte() / som
		for i := range breedtes {
			breedtes[i] *= factor
		}
	}
	return breedtes
}

// regelsVoor breekt celinhoud op woordgrens om binnen de kolombreedte.
// De volle kolombreedte gaat erin: fpdf.SplitLines trekt zelf al tweemaal de
// celmarge af (wmax = w - 2*cMargin), dus hier nog eens aftrekken breekt te vroeg af.
func (v documentVel) regelsVoor(cel markdown.Cel, breedte float64) []string {
	tekst := tekst.NaarCP1252(celtekst(cel))
	if tekst == "" {
		return nil
	}
	regels := v.d.pdf.SplitLines([]byte(tekst), breedte)
	uit := make([]string, len(regels))
	for i, regel := range regels {
		uit[i] = string(regel)
	}
	return uit
}

// rijhoogte is de hoogte van de hoogste cel, met een minimum van 16 punt.
func (v documentVel) rijhoogte(rij markdown.Rij, breedtes []float64) float64 {
	hoogte := tabelMinHoogte
	for kolom, cel := range rij.Cellen {
		if kolom >= len(breedtes) {
			break
		}
		v.stelCelfontIn(rij.Kop)
		regels := v.regelsVoor(cel, breedtes[kolom])
		celhoogte := float64(len(regels)) * tabelRegelhoogte
		if celhoogte > hoogte {
			hoogte = celhoogte
		}
	}
	return hoogte
}

// pastRij geeft aan of een rij nog op de huidige pagina past.
func (v documentVel) pastRij(hoogte float64) bool {
	_, paginaHoogte := v.d.pdf.GetPageSize()
	_, _, _, bodem := v.d.pdf.GetMargins()
	return v.d.pdf.GetY()+hoogte <= paginaHoogte-bodem
}

// tekenRij tekent één rij op de huidige positie.
func (v documentVel) tekenRij(rij markdown.Rij, breedtes []float64) {
	hoogte := v.rijhoogte(rij, breedtes)
	x := marge + v.d.inspringing
	y := v.d.pdf.GetY()
	for kolom, cel := range rij.Cellen {
		if kolom >= len(breedtes) {
			break
		}
		v.tekenCel(cel, rij.Kop, x, y, breedtes[kolom], hoogte)
		x += breedtes[kolom]
	}
	v.d.pdf.SetXY(marge+v.d.inspringing, y+hoogte)
}

// tekenCel tekent de achtergrond, rand en tekst van één cel.
func (v documentVel) tekenCel(cel markdown.Cel, kop bool, x, y, breedte, hoogte float64) {
	pdf := v.d.pdf
	v.stelCelfontIn(kop)
	if kop {
		pdf.SetFillColor(tabelGrijs, tabelGrijs, tabelGrijs)
		pdf.Rect(x, y, breedte, hoogte, "FD")
	} else {
		pdf.Rect(x, y, breedte, hoogte, "D")
	}
	regels := v.regelsVoor(cel, breedte)
	for i, regel := range regels {
		pdf.SetXY(x, y+float64(i)*tabelRegelhoogte)
		pdf.CellFormat(breedte, tabelRegelhoogte, regel, "", 0, uitlijning(cel.Uitlijning), false, 0, "")
	}
}

// uitlijning vertaalt de markdown-uitlijning naar de fpdf-lettercodes.
func uitlijning(u markdown.Uitlijning) string {
	switch u {
	case markdown.UitlijnMidden:
		return "C"
	case markdown.UitlijnRechts:
		return "R"
	default:
		return "L"
	}
}
func (v documentVel) Streep() {
	y := v.d.pdf.GetY() + 3
	v.d.pdf.Line(marge+v.d.inspringing, y, breedte-marge, y)
}
func (v documentVel) Fout() error           { return v.d.pdf.Error() }
func (v documentVel) zetX()                 { v.d.pdf.SetX(marge + v.d.inspringing) }
func (v documentVel) tekstbreedte() float64 { return breedte - 2*marge - v.d.inspringing }
