package cli

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

type nepBestandssysteem struct {
	paden  map[string]bool
	mappen map[string][]string
	fouten map[string]error
}

func (n nepBestandssysteem) Bestaat(pad string) (bool, error) {
	if err, ok := n.fouten[pad]; ok {
		return false, err
	}
	isMap, ok := n.paden[pad]
	if !ok {
		return false, os.ErrNotExist
	}
	return isMap, nil
}

func (n nepBestandssysteem) LeesMap(pad string) ([]string, error) {
	if err, ok := n.fouten[pad]; ok {
		return nil, err
	}
	inhoud, ok := n.mappen[pad]
	if !ok {
		return nil, os.ErrNotExist
	}
	return inhoud, nil
}

func nieuwNepFS() nepBestandssysteem {
	return nepBestandssysteem{
		paden: map[string]bool{"boek.md": false, "notitie.txt": false, "_Footer.md": false, "wiki": true, "wiki/": true, "leeg": true, "navigatie": true, "doelmap": true, "doel.pdf": false},
		mappen: map[string][]string{
			"wiki":      {"z.md", "_Sidebar.md", "B.md", "a.MD", "_Footer.md", "10.md", "2.md", "A.md", "b.md", "index.md", "eind.md", "hoofdstuk.md", "laatste.md", "lezen.txt", "x.markdown"},
			"wiki/":     {"z.md", "_Sidebar.md", "B.md", "a.MD", "_Footer.md", "10.md", "2.md", "A.md", "b.md", "index.md", "eind.md", "hoofdstuk.md", "laatste.md", "lezen.txt", "x.markdown"},
			"leeg":      {"tekst.txt"},
			"navigatie": {"_Footer.md", "_Sidebar.MD"},
		},
		fouten: map[string]error{},
	}
}

func TestPlannenModiEnDoelen(t *testing.T) {
	fs := nieuwNepFS()
	tests := []struct {
		naam string
		args []string
		plan Plan
	}{
		{"enkel markdown", []string{"boek.md"}, Plan{Modus: ModusEnkel, Taken: []Taak{{Bronnen: []string{"boek.md"}, Doel: "boek.pdf"}}}},
		{"enkel zonder markdownextensie", []string{"notitie.txt"}, Plan{Modus: ModusEnkel, Taken: []Taak{{Bronnen: []string{"notitie.txt"}, Doel: "notitie.txt.pdf"}}}},
		{"enkel met uitvoer na pad", []string{"boek.md", "-o", "eigen.pdf"}, Plan{Modus: ModusEnkel, Taken: []Taak{{Bronnen: []string{"boek.md"}, Doel: "eigen.pdf"}}}},
		{"samengevoegd", []string{"wiki"}, Plan{Modus: ModusSamengevoegd, Taken: []Taak{{Bronnen: wikiBronnen("wiki"), Doel: "wiki.pdf"}}}},
		{"samengevoegd met schuine streep", []string{"wiki/"}, Plan{Modus: ModusSamengevoegd, Taken: []Taak{{Bronnen: wikiBronnen("wiki/"), Doel: "wiki.pdf"}}}},
		{"samengevoegd met uitvoer", []string{"-o", "boek.pdf", "wiki"}, Plan{Modus: ModusSamengevoegd, Taken: []Taak{{Bronnen: wikiBronnen("wiki"), Doel: "boek.pdf"}}}},
		{"los", []string{"wiki", "--los"}, Plan{Modus: ModusLos, Taken: losseTaken("wiki", "")}},
		{"los kort met mapdoel", []string{"-l", "wiki", "-o", "uit"}, Plan{Modus: ModusLos, Taken: losseTaken("wiki", "uit")}},
	}
	for _, test := range tests {
		t.Run(test.naam, func(t *testing.T) {
			plan, err := Plannen(test.args, fs)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(plan, test.plan) {
				t.Errorf("plan = %#v, wil %#v", plan, test.plan)
			}
		})
	}
}

func TestPlannenFouten(t *testing.T) {
	fs := nieuwNepFS()
	tests := []struct {
		naam  string
		args  []string
		tekst string
	}{
		{"geen argument", nil, "geen invoer opgegeven\n" + Gebruik},
		{"meerdere argumenten", []string{"boek.md", "notitie.txt"}, "precies één invoerpad is vereist"},
		{"onbekende vlag", []string{"--anders", "boek.md"}, "onbekende vlag: --anders"},
		{"ontbrekende uitvoer", []string{"boek.md", "-o"}, "-o verwacht een pad"},
		{"niet bestaand", []string{"weg.md"}, "kan \"weg.md\" niet lezen"},
		{"los bij bestand", []string{"--los", "boek.md"}, "--los werkt alleen op een map"},
		{"lege map", []string{"leeg"}, "map \"leeg\" bevat geen Markdown-bestanden"},
		{"bestanddoel is map", []string{"boek.md", "-o", "doelmap"}, "-o verwijst naar een map, maar hier is een bestandsnaam nodig"},
		{"losdoel is bestand", []string{"wiki", "--los", "-o", "doel.pdf"}, "-o verwijst naar een bestand, maar bij --los is een map nodig"},
	}
	for _, test := range tests {
		t.Run(test.naam, func(t *testing.T) {
			_, err := Plannen(test.args, fs)
			if err == nil || !strings.Contains(err.Error(), test.tekst) {
				t.Fatalf("fout = %v, wil %q", err, test.tekst)
			}
		})
	}
}

func TestPlannenOnderstreepteNamen(t *testing.T) {
	fs := nieuwNepFS()
	tests := []struct {
		naam  string
		args  []string
		plan  Plan
		tekst string
	}{
		{"samengevoegd slaat navigatie over", []string{"wiki"}, Plan{Modus: ModusSamengevoegd, Taken: []Taak{{Bronnen: wikiBronnen("wiki"), Doel: "wiki.pdf"}}}, ""},
		{"los slaat navigatie over", []string{"--los", "wiki"}, Plan{Modus: ModusLos, Taken: losseTaken("wiki", "")}, ""},
		{"alleen navigatie geeft fout", []string{"navigatie"}, Plan{}, "map \"navigatie\" bevat geen Markdown-bestanden (namen die met _ beginnen worden overgeslagen)"},
		{"expliciet navigatiebestand", []string{"_Footer.md"}, Plan{Modus: ModusEnkel, Taken: []Taak{{Bronnen: []string{"_Footer.md"}, Doel: "_Footer.pdf"}}}, ""},
	}
	for _, test := range tests {
		t.Run(test.naam, func(t *testing.T) {
			plan, err := Plannen(test.args, fs)
			if test.tekst != "" {
				if err == nil || err.Error() != test.tekst {
					t.Fatalf("fout = %v, wil %q", err, test.tekst)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(plan, test.plan) {
				t.Errorf("plan = %#v, wil %#v", plan, test.plan)
			}
			if test.plan.Modus == ModusSamengevoegd && len(plan.Taken[0].Bronnen) != 11 {
				t.Errorf("aantal bronnen = %d, wil 11", len(plan.Taken[0].Bronnen))
			}
			if test.plan.Modus == ModusLos && len(plan.Taken) != 11 {
				t.Errorf("aantal taken = %d, wil 11", len(plan.Taken))
			}
		})
	}
}

func TestPlannenHulpEnVersie(t *testing.T) {
	fs := nieuwNepFS()
	for _, test := range []struct {
		args []string
		wil  error
	}{
		{[]string{"-h"}, ErrHulp},
		{[]string{"--help"}, ErrHulp},
		{[]string{"--version"}, ErrVersie},
	} {
		_, err := Plannen(test.args, fs)
		if !errors.Is(err, test.wil) {
			t.Errorf("Plannen(%q): %v, wil %v", test.args, err, test.wil)
		}
	}
}

func TestOSBestandssysteem(t *testing.T) {
	mapnaam := t.TempDir()
	if err := os.WriteFile(filepath.Join(mapnaam, "bestand"), []byte("inhoud"), 0o600); err != nil {
		t.Fatal(err)
	}
	fs := OSBestandssysteem{}
	isMap, err := fs.Bestaat(mapnaam)
	if err != nil || !isMap {
		t.Fatalf("Bestaat(map) = %v, %v", isMap, err)
	}
	isMap, err = fs.Bestaat(filepath.Join(mapnaam, "bestand"))
	if err != nil || isMap {
		t.Fatalf("Bestaat(bestand) = %v, %v", isMap, err)
	}
	namen, err := fs.LeesMap(mapnaam)
	if err != nil || !reflect.DeepEqual(namen, []string{"bestand"}) {
		t.Fatalf("LeesMap = %q, %v", namen, err)
	}
}

func wikiBronnen(mapnaam string) []string {
	return []string{
		filepath.Join(mapnaam, "10.md"), filepath.Join(mapnaam, "2.md"), filepath.Join(mapnaam, "A.md"), filepath.Join(mapnaam, "B.md"), filepath.Join(mapnaam, "a.MD"), filepath.Join(mapnaam, "b.md"), filepath.Join(mapnaam, "eind.md"), filepath.Join(mapnaam, "hoofdstuk.md"), filepath.Join(mapnaam, "index.md"), filepath.Join(mapnaam, "laatste.md"), filepath.Join(mapnaam, "z.md"),
	}
}

func losseTaken(mapnaam, doelmap string) []Taak {
	bronnen := wikiBronnen(mapnaam)
	taken := make([]Taak, len(bronnen))
	for i, bron := range bronnen {
		doel := pdfNaam(bron)
		if doelmap != "" {
			doel = filepath.Join(doelmap, filepath.Base(doel))
		}
		taken[i] = Taak{Bronnen: []string{bron}, Doel: doel}
	}
	return taken
}
