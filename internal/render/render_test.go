package render

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gnutterts/md2pdf/internal/markdown"
	"github.com/gnutterts/md2pdf/internal/mermaid"
)

type actieveStijl struct {
	familie      string
	grootte      float64
	vet, cursief bool
}

type nepVel struct {
	aanroepen []string
	stijl     actieveStijl
	tabellen  [][]markdown.Row
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
func (n *nepVel) Diagram([]byte) error { n.aanroepen = append(n.aanroepen, "diagram"); return nil }
func (n *nepVel) Streep()              { n.aanroepen = append(n.aanroepen, "streep") }
func (n *nepVel) Tabel(rijen []markdown.Row) {
	n.tabellen = append(n.tabellen, rijen)
	n.aanroepen = append(n.aanroepen, fmt.Sprintf("tabel:%d", len(rijen)))
}
func (n *nepVel) Fout() error { return nil }
func (s actieveStijl) string() string {
	return fmt.Sprintf("%s:%s:%g", s.familie, stijl(s.vet, s.cursief), s.grootte)
}
func TestTekenMermaidTerugval(t *testing.T) {
	blok := []markdown.Block{{Kind: markdown.CodeBlock, Language: "mermaid", Lines: []string{"graph TD", "A-->B"}}}
	for _, test := range []struct {
		naam, script   string
		wilDiagram     bool
		waarschuwingen int
	}{
		{"gelukt", "#!/bin/sh\nprintf png > \"$4\"\n", true, 0},
		{"faalt", "#!/bin/sh\nexit 1\n", false, 1},
		{"uit", "", false, 0},
	} {
		t.Run(test.naam, func(t *testing.T) {
			vel := &nepVel{}
			opties := Opties{}
			if test.script != "" {
				pad := filepath.Join(t.TempDir(), "renderer")
				if err := os.WriteFile(pad, []byte(test.script), 0o755); err != nil {
					t.Fatal(err)
				}
				opties.Mermaid = mermaid.Renderer{Path: pad}
			}
			waarschuwingen := 0
			opties.Waarschuw = func(string) { waarschuwingen++ }
			if err := Teken(blok, vel, opties); err != nil {
				t.Fatal(err)
			}
			heeftDiagram := bevat(vel.aanroepen, "diagram")
			heeftCode := bevat(vel.aanroepen, "code:graph TD,A-->B")
			if heeftDiagram != test.wilDiagram || heeftCode == test.wilDiagram || waarschuwingen != test.waarschuwingen {
				t.Fatalf("aanroepen=%v, waarschuwingen=%d", vel.aanroepen, waarschuwingen)
			}
		})
	}
}

func bevat(aanroepen []string, wil string) bool {
	for _, aanroep := range aanroepen {
		if aanroep == wil {
			return true
		}
	}
	return false
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
		blokken []markdown.Block
		wil     []string
	}{
		{"kop en alinea", []markdown.Block{{Kind: markdown.Heading, Level: 1, Spans: []markdown.Span{{Text: "Titel"}}}, {Kind: markdown.Paragraph, Spans: []markdown.Span{{Text: "tekst"}}}}, []string{"stijl:Helvetica:B:20", "tekst:Titel:Helvetica:B:20", "stijl:Helvetica::11", "tekst:tekst:Helvetica::11"}},
		{"geneste lijst", []markdown.Block{{Kind: markdown.ListItem, Depth: 1, Spans: []markdown.Span{{Text: "binnen"}}}}, []string{"inspringen", "tekst:• :Helvetica::11", "tekst:binnen:Helvetica::11", "inspringen"}},
		{"codeblok", []markdown.Block{{Kind: markdown.CodeBlock, Lines: []string{"x"}}}, []string{"inspringen", "stijl:Courier::9.5", "code:x", "inspringen"}},
		{"link", []markdown.Block{{Kind: markdown.Paragraph, Spans: []markdown.Span{{Text: "site", URL: "https://x"}}}}, []string{"stijl:Helvetica::11", "link:site:https://x:Helvetica::11"}},
		{"citaat", []markdown.Block{{Kind: markdown.Quote, Spans: []markdown.Span{{Text: "woord"}}}}, []string{"inspringen", "stijl:Helvetica:I:11", "tekst:woord:Helvetica:I:11", "inspringen"}},
	}
	for _, test := range tests {
		t.Run(test.naam, func(t *testing.T) {
			vel := &nepVel{}
			if err := Teken(test.blokken, vel, Opties{}); err != nil {
				t.Fatal(err)
			}
			controleerVolgorde(t, vel.aanroepen, test.wil)
		})
	}
}

func TestTekenTabel(t *testing.T) {
	blokken := []markdown.Block{{Kind: markdown.Table, Rows: []markdown.Row{
		{Header: true, Cells: []markdown.Cell{{Spans: []markdown.Span{{Text: "A"}}}, {Spans: []markdown.Span{{Text: "B"}}}}},
		{Cells: []markdown.Cell{{Spans: []markdown.Span{{Text: "1"}}}, {Spans: []markdown.Span{{Text: "2"}}}}},
	}}}
	vel := &nepVel{}
	if err := Teken(blokken, vel, Opties{}); err != nil {
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
	if !vel.tabellen[0][0].Header || vel.tabellen[0][1].Header {
		t.Fatalf("kopregel niet als eerste rij: %v", vel.tabellen[0])
	}
}

func TestRegelhoogteVolgtKopgrootte(t *testing.T) {
	vel := &nepVel{}
	if err := Teken([]markdown.Block{{Kind: markdown.Heading, Level: 1, Spans: []markdown.Span{{Text: "Titel"}}}}, vel, Opties{}); err != nil {
		t.Fatal(err)
	}
	controleerVolgorde(t, vel.aanroepen, []string{"einde:27"})
}

func TestTekstStijlen(t *testing.T) {
	tests := []struct {
		naam string
		blok markdown.Block
		wil  string
	}{
		{"kop niveau 1", markdown.Block{Kind: markdown.Heading, Level: 1, Spans: []markdown.Span{{Text: "Een"}}}, "tekst:Een:Helvetica:B:20"},
		{"kop niveau 2", markdown.Block{Kind: markdown.Heading, Level: 2, Spans: []markdown.Span{{Text: "Twee"}}}, "tekst:Twee:Helvetica:B:16"},
		{"kop niveau 3", markdown.Block{Kind: markdown.Heading, Level: 3, Spans: []markdown.Span{{Text: "Drie"}}}, "tekst:Drie:Helvetica:B:13"},
		{"kop niveau 4", markdown.Block{Kind: markdown.Heading, Level: 4, Spans: []markdown.Span{{Text: "Vier"}}}, "tekst:Vier:Helvetica:B:11"},
		{"vette alinea", markdown.Block{Kind: markdown.Paragraph, Spans: []markdown.Span{{Text: "vet", Bold: true}}}, "tekst:vet:Helvetica:B:11"},
		{"code in kop", markdown.Block{Kind: markdown.Heading, Level: 1, Spans: []markdown.Span{{Text: "code", Code: true}}}, "tekst:code:Courier:B:17"},
		{"code in alinea", markdown.Block{Kind: markdown.Paragraph, Spans: []markdown.Span{{Text: "code", Code: true}}}, "tekst:code:Courier::9.5"},
	}
	for _, test := range tests {
		t.Run(test.naam, func(t *testing.T) {
			vel := &nepVel{}
			if err := Teken([]markdown.Block{test.blok}, vel, Opties{}); err != nil {
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
