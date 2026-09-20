// Package markdown zet Markdown om in een klein, onafhankelijk documentmodel.
package markdown

import (
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	extast "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/text"
)

// Soort is het soort blok in een document.
type Soort int

const (
	Kop Soort = iota
	Alinea
	Lijstitem
	Codeblok
	Citaat
	Streep
	Tabel
)

// Stuk is een opgemaakt inline-fragment.
type Stuk struct {
	Tekst   string
	Vet     bool
	Cursief bool
	Code    bool
	URL     string
}

// Blok is een onderdeel van een document.
type Blok struct {
	Soort     Soort
	Niveau    int
	Diepte    int
	Genummerd bool
	Nummer    int
	Taal      string
	Stukken   []Stuk
	Regels    []string
	Rijen     [][]Stuk
}

// Ontleed leest Markdown naar een platte documentstructuur.
func Ontleed(bron []byte) ([]Blok, error) {
	md := goldmark.New(goldmark.WithExtensions(extension.GFM))
	doc := md.Parser().Parse(text.NewReader(bron))
	var blokken []Blok
	verwerkKinderen(doc, bron, &blokken, 0, false, false, 0)
	return blokken, nil
}

func verwerkKinderen(ouder ast.Node, bron []byte, uit *[]Blok, diepte int, inLijst, genummerd bool, nummer int) {
	for kind := ouder.FirstChild(); kind != nil; kind = kind.NextSibling() {
		switch n := kind.(type) {
		case *ast.Heading:
			*uit = append(*uit, Blok{Soort: Kop, Niveau: n.Level, Stukken: stukken(n, bron, false, false, false, "")})
		case *ast.Paragraph, *ast.TextBlock:
			soort := Alinea
			if inLijst {
				soort = Lijstitem
			}
			*uit = append(*uit, Blok{Soort: soort, Diepte: diepte, Genummerd: genummerd, Nummer: nummer, Stukken: stukken(n, bron, false, false, false, "")})
		case *ast.List:
			lijstDiepte := diepte
			if inLijst {
				lijstDiepte++
			}
			volg := n.Start
			for item := n.FirstChild(); item != nil; item = item.NextSibling() {
				if li, ok := item.(*ast.ListItem); ok {
					nr := 0
					if n.IsOrdered() {
						nr = volg
						volg++
					}
					verwerkKinderen(li, bron, uit, lijstDiepte, true, n.IsOrdered(), nr)
				}
			}
		case *ast.FencedCodeBlock:
			*uit = append(*uit, codeblok(n, bron, string(n.Language(bron))))
		case *ast.CodeBlock:
			*uit = append(*uit, codeblok(n, bron, ""))
		case *ast.Blockquote:
			for regel := n.FirstChild(); regel != nil; regel = regel.NextSibling() {
				*uit = append(*uit, Blok{Soort: Citaat, Stukken: stukken(regel, bron, false, true, false, "")})
			}
		case *ast.ThematicBreak:
			*uit = append(*uit, Blok{Soort: Streep})
		case *extast.Table:
			*uit = append(*uit, tabel(n, bron))
		}
	}
}

func codeblok(n ast.Node, bron []byte, taal string) Blok {
	regels := make([]string, 0)
	for i := 0; i < n.Lines().Len(); i++ {
		stuk := n.Lines().At(i)
		regels = append(regels, strings.TrimSuffix(string(stuk.Value(bron)), "\n"))
	}
	return Blok{Soort: Codeblok, Taal: taal, Regels: regels}
}

func tabel(n *extast.Table, bron []byte) Blok {
	var rijen [][]Stuk
	for rij := n.FirstChild(); rij != nil; rij = rij.NextSibling() {
		var cellen []Stuk
		for cel := rij.FirstChild(); cel != nil; cel = cel.NextSibling() {
			cellen = append(cellen, stukken(cel, bron, false, false, false, "")...)
		}
		rijen = append(rijen, cellen)
	}
	return Blok{Soort: Tabel, Rijen: rijen}
}

func stukken(n ast.Node, bron []byte, vet, cursief, code bool, url string) []Stuk {
	var uit []Stuk
	var loop func(ast.Node, bool, bool, bool, string)
	toevoegen := func(tekst string, v, c, co bool, u string) {
		if tekst == "" {
			return
		}
		if len(uit) > 0 && uit[len(uit)-1].Vet == v && uit[len(uit)-1].Cursief == c && uit[len(uit)-1].Code == co && uit[len(uit)-1].URL == u {
			uit[len(uit)-1].Tekst += tekst
			return
		}
		uit = append(uit, Stuk{Tekst: tekst, Vet: v, Cursief: c, Code: co, URL: u})
	}
	loop = func(k ast.Node, v, c, co bool, u string) {
		switch x := k.(type) {
		case *ast.Text:
			tekst := string(x.Segment.Value(bron))
			if x.SoftLineBreak() {
				tekst += " "
			}
			if x.HardLineBreak() {
				tekst += "\n"
			}
			toevoegen(tekst, v, c, co, u)
		case *ast.String:
			toevoegen(string(x.Value), v, c, co, u)
		case *ast.CodeSpan:
			toevoegen(string(x.Text(bron)), v, c, true, u)
		case *ast.Emphasis:
			for q := x.FirstChild(); q != nil; q = q.NextSibling() {
				loop(q, v || x.Level >= 2, c || x.Level == 1 || x.Level == 3, co, u)
			}
		case *ast.Link:
			for q := x.FirstChild(); q != nil; q = q.NextSibling() {
				loop(q, v, c, co, string(x.Destination))
			}
		default:
			for q := k.FirstChild(); q != nil; q = q.NextSibling() {
				loop(q, v, c, co, u)
			}
		}
	}
	for kind := n.FirstChild(); kind != nil; kind = kind.NextSibling() {
		loop(kind, vet, cursief, code, url)
	}
	return uit
}
