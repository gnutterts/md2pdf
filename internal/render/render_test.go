package render

import (
	"fmt"
	"strings"
	"testing"

	"github.com/gnutterts/md2pdf/internal/markdown"
)

type actieveStijl struct {
	familie      string
	grootte      float64
	vet, cursief bool
}

type nepVel struct {
	aanroepen []string
	stijl     actieveStijl
	tabellen  [][]markdown.Rij
}

func (n *nepVel) NieuwePagina() { n.aanroepen = append(n.aanroepen, "pagina") }
func (n *nepVel) Stijl(f string, v, c bool, g float64) {
	n.stijl = actieveStijl{familie: f, grootte: g, vet: v, cursief: c}
	n.aanroepen = append(n.aanroepen, "stijl:"+n.stijl.string())
}
func (n *nepVel) Tekst(s string) {
	n.aanroepen = append(n.aanroepen, "tekst:"+s+":"+n.stijl.string())
}
func (n *nepVel) Link(s, u string) {
	n.aanroepen = append(n.aanroepen, "link:"+s+":"+u+":"+n.stijl.string())
}
func (n *nepVel) Regeleinde(h float64) {
	n.aanroepen = append(n.aanroepen, fmt.Sprintf("einde:%g", h))
}
func (n *nepVel) Inspringen(p float64) { n.aanroepen = append(n.aanroepen, "inspringen") }
func (n *nepVel) HangendInspringen()   { n.aanroepen = append(n.aanroepen, "hangend") }
func (n *nepVel) Codeblok(r []string) {
	n.aanroepen = append(n.aanroepen, "code:"+strings.Join(r, ","))
}
func (n *nepVel) Streep() { n.aanroepen = append(n.aanroepen, "streep") }
func (n *nepVel) Tabel(rijen []markdown.Rij) {
	n.tabellen = append(n.tabellen, rijen)
	n.aanroepen = append(n.aanroepen, fmt.Sprintf("tabel:%d", len(rijen)))
}
func (n *nepVel) Fout() error { return nil }
func (s actieveStijl) string() string {
	return fmt.Sprintf("%s:%s:%g", s.familie, stijl(s.vet, s.cursief), s.grootte)
}
func stijl(v, c bool) string {
	if v && c {
		return "BI"
	}
	if v {
		return "B"
	}
	if c {
		return "I"
	}
	return ""
}

func TestTekenVolgordeEnStijlen(t *testing.T) {
	tests := []struct {
		naam    string
		blokken []markdown.Blok
		wil     []string
	}{
		{"kop en alinea", []markdown.Blok{{Soort: markdown.Kop, Niveau: 1, Stukken: []markdown.Stuk{{Tekst: "Titel"}}}, {Soort: markdown.Alinea, Stukken: []markdown.Stuk{{Tekst: "tekst"}}}}, []string{"stijl:Helvetica:B:20", "tekst:Titel:Helvetica:B:20", "stijl:Helvetica::11", "tekst:tekst:Helvetica::11"}},
		{"geneste lijst", []markdown.Blok{{Soort: markdown.Lijstitem, Diepte: 1, Stukken: []markdown.Stuk{{Tekst: "binnen"}}}}, []string{"inspringen", "tekst:• :Helvetica::11", "tekst:binnen:Helvetica::11", "inspringen"}},
		{"codeblok", []markdown.Blok{{Soort: markdown.Codeblok, Regels: []string{"x"}}}, []string{"inspringen", "stijl:Courier::9.5", "code:x", "inspringen"}},
		{"link", []markdown.Blok{{Soort: markdown.Alinea, Stukken: []markdown.Stuk{{Tekst: "site", URL: "https://x"}}}}, []string{"stijl:Helvetica::11", "link:site:https://x:Helvetica::11"}},
		{"citaat", []markdown.Blok{{Soort: markdown.Citaat, Stukken: []markdown.Stuk{{Tekst: "woord"}}}}, []string{"inspringen", "stijl:Helvetica:I:11", "tekst:woord:Helvetica:I:11", "inspringen"}},
	}
	for _, test := range tests {
		t.Run(test.naam, func(t *testing.T) {
			vel := &nepVel{}
			if err := Teken(test.blokken, vel); err != nil {
				t.Fatal(err)
			}
			controleerVolgorde(t, vel.aanroepen, test.wil)
		})
	}
}

func TestTekenTabel(t *testing.T) {
	blokken := []markdown.Blok{{Soort: markdown.Tabel, Rijen: []markdown.Rij{
		{Kop: true, Cellen: []markdown.Cel{{Stukken: []markdown.Stuk{{Tekst: "A"}}}, {Stukken: []markdown.Stuk{{Tekst: "B"}}}}},
		{Cellen: []markdown.Cel{{Stukken: []markdown.Stuk{{Tekst: "1"}}}, {Stukken: []markdown.Stuk{{Tekst: "2"}}}}},
	}}}
	vel := &nepVel{}
	if err := Teken(blokken, vel); err != nil {
		t.Fatal(err)
	}
	tabelAanroepen := 0
	for _, aanroep := range vel.aanroepen {
		if strings.HasPrefix(aanroep, "tabel:") {
			tabelAanroepen++
		}
	}
	if tabelAanroepen != 1 {
		t.Fatalf("Tabel is %d keer aangeroepen: %q", tabelAanroepen, vel.aanroepen)
	}
	if len(vel.tabellen) != 1 || len(vel.tabellen[0]) != 2 {
		t.Fatalf("Tabel kreeg %d rijen, wil 2: %v", len(vel.tabellen), vel.tabellen)
	}
	if !vel.tabellen[0][0].Kop || vel.tabellen[0][1].Kop {
		t.Fatalf("kopregel niet als eerste rij: %v", vel.tabellen[0])
	}
}

func TestRegelhoogteVolgtKopgrootte(t *testing.T) {
	vel := &nepVel{}
	if err := Teken([]markdown.Blok{{Soort: markdown.Kop, Niveau: 1, Stukken: []markdown.Stuk{{Tekst: "Titel"}}}}, vel); err != nil {
		t.Fatal(err)
	}
	controleerVolgorde(t, vel.aanroepen, []string{"einde:27"})
}

func TestTekstStijlen(t *testing.T) {
	tests := []struct {
		naam string
		blok markdown.Blok
		wil  string
	}{
		{"kop niveau 1", markdown.Blok{Soort: markdown.Kop, Niveau: 1, Stukken: []markdown.Stuk{{Tekst: "Een"}}}, "tekst:Een:Helvetica:B:20"},
		{"kop niveau 2", markdown.Blok{Soort: markdown.Kop, Niveau: 2, Stukken: []markdown.Stuk{{Tekst: "Twee"}}}, "tekst:Twee:Helvetica:B:16"},
		{"kop niveau 3", markdown.Blok{Soort: markdown.Kop, Niveau: 3, Stukken: []markdown.Stuk{{Tekst: "Drie"}}}, "tekst:Drie:Helvetica:B:13"},
		{"kop niveau 4", markdown.Blok{Soort: markdown.Kop, Niveau: 4, Stukken: []markdown.Stuk{{Tekst: "Vier"}}}, "tekst:Vier:Helvetica:B:11"},
		{"vette alinea", markdown.Blok{Soort: markdown.Alinea, Stukken: []markdown.Stuk{{Tekst: "vet", Vet: true}}}, "tekst:vet:Helvetica:B:11"},
		{"code in kop", markdown.Blok{Soort: markdown.Kop, Niveau: 1, Stukken: []markdown.Stuk{{Tekst: "code", Code: true}}}, "tekst:code:Courier:B:17"},
		{"code in alinea", markdown.Blok{Soort: markdown.Alinea, Stukken: []markdown.Stuk{{Tekst: "code", Code: true}}}, "tekst:code:Courier::9.5"},
	}
	for _, test := range tests {
		t.Run(test.naam, func(t *testing.T) {
			vel := &nepVel{}
			if err := Teken([]markdown.Blok{test.blok}, vel); err != nil {
				t.Fatal(err)
			}
			controleerVolgorde(t, vel.aanroepen, []string{test.wil})
		})
	}
}

func controleerVolgorde(t *testing.T, kreeg, wil []string) {
	t.Helper()
	vanaf := 0
	for _, verwacht := range wil {
		gevonden := -1
		for i := vanaf; i < len(kreeg); i++ {
			if kreeg[i] == verwacht {
				gevonden = i
				break
			}
		}
		if gevonden < 0 {
			t.Fatalf("%q ontbreekt in %q", verwacht, kreeg)
		}
		vanaf = gevonden + 1
	}
}
