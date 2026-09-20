// Package render tekent het documentmodel op een Vel.
package render

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/gnutterts/md2pdf/internal/markdown"
	"github.com/gnutterts/md2pdf/internal/mermaid"
)

// Vel is het abstracte tekenvlak voor een PDF-document.
type Vel interface {
	NieuwePagina()
	Stijl(familie string, vet, cursief bool, grootte float64)
	Tekst(tekst string)
	Link(tekst, url string)
	Regeleinde(hoogte float64)
	Inspringen(punten float64)
	// HangendInspringen laat teruglopende regels uitlijnen op de huidige kolom
	// in plaats van op de inspringing van het blok, zodat de tekst van een
	// lijstitem onder zichzelf doorloopt en niet onder de opsommingsbol.
	HangendInspringen()
	Codeblok(regels []string)
	Streep()
	// Tabel tekent een volledige tabel: het vel bepaalt kolombreedtes en
	// paginabreuken, omdat alleen daar de fontmetrieken bekend zijn.
	Tabel(rijen []markdown.Rij)
	// Diagram tekent een PNG op de volle tekstbreedte, met behoud van
	// verhouding, en begint op een nieuwe pagina als het niet meer past.
	Diagram(png []byte) error
	Fout() error
}

const regelhoogte = 15.0

type basisstijl struct {
	familie      string
	grootte      float64
	vet, cursief bool
}

func hoogteVoor(grootte float64) float64 {
	hoogte := math.Round(grootte*1.35*2) / 2
	return math.Max(regelhoogte, hoogte)
}

// Opties bepaalt optioneel hoe diagrammen worden gerenderd.
type Opties struct {
	Mermaid   mermaid.Renderer
	Waarschuw func(melding string)
}

// Teken tekent blokken in hun oorspronkelijke volgorde.
func Teken(blokken []markdown.Blok, vel Vel, opties Opties) error {
	eerste := true
	for _, blok := range blokken {
		switch blok.Soort {
		case markdown.Kop:
			if !eerste {
				vel.Regeleinde(12)
			}
			basis := basisstijl{familie: "Helvetica", grootte: 11, vet: true}
			if blok.Niveau == 1 {
				basis.grootte = 20
			}
			if blok.Niveau == 2 {
				basis.grootte = 16
			}
			if blok.Niveau == 3 {
				basis.grootte = 13
			}
			vel.Stijl(basis.familie, basis.vet, basis.cursief, basis.grootte)
			stukken(vel, blok.Stukken, basis)
			vel.Regeleinde(hoogteVoor(basis.grootte))
			vel.Regeleinde(6)
		case markdown.Alinea:
			basis := basisstijl{familie: "Helvetica", grootte: 11}
			vel.Stijl(basis.familie, basis.vet, basis.cursief, basis.grootte)
			stukken(vel, blok.Stukken, basis)
			vel.Regeleinde(hoogteVoor(basis.grootte))
			vel.Regeleinde(6)
		case markdown.Lijstitem:
			vel.Inspringen(float64(blok.Diepte) * 14)
			basis := basisstijl{familie: "Helvetica", grootte: 11}
			vel.Stijl(basis.familie, basis.vet, basis.cursief, basis.grootte)
			if blok.Genummerd {
				vel.Tekst(strconv.Itoa(blok.Nummer) + ". ")
			} else {
				vel.Tekst("• ")
			}
			vel.HangendInspringen()
			stukken(vel, blok.Stukken, basis)
			vel.Regeleinde(hoogteVoor(basis.grootte))
			vel.Inspringen(-float64(blok.Diepte) * 14)
		case markdown.Codeblok:
			if blok.Taal == "mermaid" && opties.Mermaid.Beschikbaar() {
				png, err := opties.Mermaid.NaarPNG(strings.Join(blok.Regels, "\n"))
				if err == nil {
					err = vel.Diagram(png)
				}
				if err == nil {
					vel.Regeleinde(6)
					break
				}
				waarschuw(opties, fmt.Sprintf("mermaid-diagram kon niet worden getekend: %v", err))
			}
			tekenCodeblok(vel, blok.Regels)
		case markdown.Citaat:
			vel.Inspringen(14)
			basis := basisstijl{familie: "Helvetica", grootte: 11, cursief: true}
			vel.Stijl(basis.familie, basis.vet, basis.cursief, basis.grootte)
			stukken(vel, blok.Stukken, basis)
			vel.Regeleinde(hoogteVoor(basis.grootte))
			vel.Inspringen(-14)
			vel.Regeleinde(6)
		case markdown.Streep:
			vel.Streep()
			vel.Regeleinde(6)
		case markdown.Tabel:
			vel.Tabel(blok.Rijen)
			vel.Regeleinde(6)
		}
		eerste = false
	}
	return vel.Fout()
}

func tekenCodeblok(vel Vel, regels []string) {
	vel.Inspringen(10)
	vel.Stijl("Courier", false, false, 9.5)
	vel.Codeblok(regels)
	vel.Inspringen(-10)
	vel.Regeleinde(6)
}

func waarschuw(opties Opties, melding string) {
	if opties.Waarschuw != nil {
		opties.Waarschuw(melding)
	}
}

func stukken(vel Vel, stukken []markdown.Stuk, basis basisstijl) {
	for _, stuk := range stukken {
		familie, grootte := basis.familie, basis.grootte
		if stuk.Code {
			familie = "Courier"
			grootte = math.Round(basis.grootte*0.85*2) / 2
		}
		vel.Stijl(familie, basis.vet || stuk.Vet, basis.cursief || stuk.Cursief, grootte)
		if stuk.URL != "" {
			vel.Link(stuk.Tekst, stuk.URL)
		} else {
			vel.Tekst(stuk.Tekst)
		}
	}
}
