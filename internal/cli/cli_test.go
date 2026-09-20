package cli

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

type fakeFileSystem struct {
	paths  map[string]bool
	dirs   map[string][]string
	errors map[string]error
}

func (n fakeFileSystem) Stat(path string) (bool, error) {
	if err, ok := n.errors[path]; ok {
		return false, err
	}
	isDir, ok := n.paths[path]
	if !ok {
		return false, os.ErrNotExist
	}
	return isDir, nil
}

func (n fakeFileSystem) ReadDir(path string) ([]string, error) {
	if err, ok := n.errors[path]; ok {
		return nil, err
	}
	content, ok := n.dirs[path]
	if !ok {
		return nil, os.ErrNotExist
	}
	return content, nil
}

func newFakeFS() fakeFileSystem {
	return fakeFileSystem{
		paths: map[string]bool{"boek.md": false, "notitie.txt": false, "_Footer.md": false, "wiki": true, "wiki/": true, "leeg": true, "navigatie": true, "doelmap": true, "doel.pdf": false},
		dirs: map[string][]string{
			"wiki":      {"z.md", "_Sidebar.md", "B.md", "a.MD", "_Footer.md", "10.md", "2.md", "A.md", "b.md", "index.md", "eind.md", "hoofdstuk.md", "laatste.md", "lezen.txt", "x.markdown"},
			"wiki/":     {"z.md", "_Sidebar.md", "B.md", "a.MD", "_Footer.md", "10.md", "2.md", "A.md", "b.md", "index.md", "eind.md", "hoofdstuk.md", "laatste.md", "lezen.txt", "x.markdown"},
			"leeg":      {"tekst.txt"},
			"navigatie": {"_Footer.md", "_Sidebar.MD"},
		},
		errors: map[string]error{},
	}
}

func TestParseModesAndTargets(t *testing.T) {
	fs := newFakeFS()
	tests := []struct {
		name string
		args []string
		plan Plan
	}{
		{"enkel markdown", []string{"boek.md"}, Plan{Mode: ModeSingle, Tasks: []Task{{Sources: []string{"boek.md"}, Target: "boek.pdf"}}}},
		{"enkel zonder markdownextensie", []string{"notitie.txt"}, Plan{Mode: ModeSingle, Tasks: []Task{{Sources: []string{"notitie.txt"}, Target: "notitie.txt.pdf"}}}},
		{"enkel met uitvoer na pad", []string{"boek.md", "-o", "eigen.pdf"}, Plan{Mode: ModeSingle, Tasks: []Task{{Sources: []string{"boek.md"}, Target: "eigen.pdf"}}}},
		{"mermaid", []string{"--mermaid", "eigen-mmdc", "boek.md"}, Plan{Mode: ModeSingle, Tasks: []Task{{Sources: []string{"boek.md"}, Target: "boek.pdf"}}, Mermaid: "eigen-mmdc"}},
		{"mermaid na invoer", []string{"boek.md", "--mermaid", "eigen-mmdc"}, Plan{Mode: ModeSingle, Tasks: []Task{{Sources: []string{"boek.md"}, Target: "boek.pdf"}}, Mermaid: "eigen-mmdc"}},
		{"mermaid bij map", []string{"--mermaid", "uit", "wiki"}, Plan{Mode: ModeMerged, Tasks: []Task{{Sources: wikiSources("wiki"), Target: "wiki.pdf"}}, Mermaid: "uit"}},
		{"laatste mermaid wint", []string{"--mermaid", "eerste", "--mermaid", "tweede", "boek.md"}, Plan{Mode: ModeSingle, Tasks: []Task{{Sources: []string{"boek.md"}, Target: "boek.pdf"}}, Mermaid: "tweede"}},
		{"mermaid met los", []string{"--los", "--mermaid", "pad", "wiki"}, Plan{Mode: ModeSeparate, Tasks: separateTasks("wiki", ""), Mermaid: "pad"}},
		{"invoer na dubbele streep", []string{"--", "boek.md"}, Plan{Mode: ModeSingle, Tasks: []Task{{Sources: []string{"boek.md"}, Target: "boek.pdf"}}}},
		{"mermaid voor uitvoer", []string{"--mermaid", "pad", "-o", "eigen.pdf", "boek.md"}, Plan{Mode: ModeSingle, Tasks: []Task{{Sources: []string{"boek.md"}, Target: "eigen.pdf"}}, Mermaid: "pad"}},

		{"samengevoegd", []string{"wiki"}, Plan{Mode: ModeMerged, Tasks: []Task{{Sources: wikiSources("wiki"), Target: "wiki.pdf"}}}},
		{"samengevoegd met schuine streep", []string{"wiki/"}, Plan{Mode: ModeMerged, Tasks: []Task{{Sources: wikiSources("wiki/"), Target: "wiki.pdf"}}}},
		{"samengevoegd met uitvoer", []string{"-o", "boek.pdf", "wiki"}, Plan{Mode: ModeMerged, Tasks: []Task{{Sources: wikiSources("wiki"), Target: "boek.pdf"}}}},
		{"los", []string{"wiki", "--los"}, Plan{Mode: ModeSeparate, Tasks: separateTasks("wiki", "")}},
		{"los kort met mapdoel", []string{"-l", "wiki", "-o", "uit"}, Plan{Mode: ModeSeparate, Tasks: separateTasks("wiki", "uit")}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			plan, err := Parse(test.args, fs)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(plan, test.plan) {
				t.Errorf("plan = %#v, wil %#v", plan, test.plan)
			}
		})
	}
}

func TestParseErrors(t *testing.T) {
	fs := newFakeFS()
	tests := []struct {
		name string
		args []string
		text string
	}{
		{"geen argument", nil, "geen invoer opgegeven\n" + Usage},
		{"meerdere argumenten", []string{"boek.md", "notitie.txt"}, "precies één invoerpad is vereist"},
		{"onbekende vlag", []string{"--anders", "boek.md"}, "onbekende vlag: --anders"},
		{"ontbrekende uitvoer", []string{"boek.md", "-o"}, "-o verwacht een pad"},
		{"ontbrekende mermaid", []string{"boek.md", "--mermaid"}, "--mermaid verwacht een pad"},
		{"mermaid is geen afkorting", []string{"boek.md", "--mer"}, "onbekende vlag: --mer"},
		{"mermaid zonder invoer", []string{"--mermaid", "pad"}, "geen invoer opgegeven"},
		{"mermaid met onbekende vlag", []string{"--mermaid", "pad", "--anders", "boek.md"}, "onbekende vlag: --anders"},
		{"niet bestaand", []string{"weg.md"}, "kan \"weg.md\" niet lezen"},
		{"los bij bestand", []string{"--los", "boek.md"}, "--los werkt alleen op een map"},
		{"lege map", []string{"leeg"}, "map \"leeg\" bevat geen Markdown-bestanden"},
		{"bestanddoel is map", []string{"boek.md", "-o", "doelmap"}, "-o verwijst naar een map, maar hier is een bestandsnaam nodig"},
		{"losdoel is bestand", []string{"wiki", "--los", "-o", "doel.pdf"}, "-o verwijst naar een bestand, maar bij --los is een map nodig"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := Parse(test.args, fs)
			if err == nil || !strings.Contains(err.Error(), test.text) {
				t.Fatalf("fout = %v, wil %q", err, test.text)
			}
		})
	}
}

func TestParseUnderscoredNames(t *testing.T) {
	fs := newFakeFS()
	tests := []struct {
		name string
		args []string
		plan Plan
		text string
	}{
		{"samengevoegd slaat navigatie over", []string{"wiki"}, Plan{Mode: ModeMerged, Tasks: []Task{{Sources: wikiSources("wiki"), Target: "wiki.pdf"}}}, ""},
		{"los slaat navigatie over", []string{"--los", "wiki"}, Plan{Mode: ModeSeparate, Tasks: separateTasks("wiki", "")}, ""},
		{"alleen navigatie geeft fout", []string{"navigatie"}, Plan{}, "map \"navigatie\" bevat geen Markdown-bestanden (namen die met _ beginnen worden overgeslagen)"},
		{"expliciet navigatiebestand", []string{"_Footer.md"}, Plan{Mode: ModeSingle, Tasks: []Task{{Sources: []string{"_Footer.md"}, Target: "_Footer.pdf"}}}, ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			plan, err := Parse(test.args, fs)
			if test.text != "" {
				if err == nil || err.Error() != test.text {
					t.Fatalf("fout = %v, wil %q", err, test.text)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(plan, test.plan) {
				t.Errorf("plan = %#v, wil %#v", plan, test.plan)
			}
			if test.plan.Mode == ModeMerged && len(plan.Tasks[0].Sources) != 11 {
				t.Errorf("aantal bronnen = %d, wil 11", len(plan.Tasks[0].Sources))
			}
			if test.plan.Mode == ModeSeparate && len(plan.Tasks) != 11 {
				t.Errorf("aantal taken = %d, wil 11", len(plan.Tasks))
			}
		})
	}
}

func TestParseHelpAndVersion(t *testing.T) {
	fs := newFakeFS()
	for _, test := range []struct {
		args []string
		want error
	}{
		{[]string{"-h"}, ErrHelp},
		{[]string{"--help"}, ErrHelp},
		{[]string{"--version"}, ErrVersion},
	} {
		_, err := Parse(test.args, fs)
		if !errors.Is(err, test.want) {
			t.Errorf("Plannen(%q): %v, wil %v", test.args, err, test.want)
		}
	}
}

func TestOSFileSystem(t *testing.T) {
	dirName := t.TempDir()
	if err := os.WriteFile(filepath.Join(dirName, "bestand"), []byte("inhoud"), 0o600); err != nil {
		t.Fatal(err)
	}
	fs := OSFileSystem{}
	isDir, err := fs.Stat(dirName)
	if err != nil || !isDir {
		t.Fatalf("Bestaat(map) = %v, %v", isDir, err)
	}
	isDir, err = fs.Stat(filepath.Join(dirName, "bestand"))
	if err != nil || isDir {
		t.Fatalf("Bestaat(bestand) = %v, %v", isDir, err)
	}
	names, err := fs.ReadDir(dirName)
	if err != nil || !reflect.DeepEqual(names, []string{"bestand"}) {
		t.Fatalf("LeesMap = %q, %v", names, err)
	}
}

func wikiSources(dirName string) []string {
	return []string{
		filepath.Join(dirName, "10.md"), filepath.Join(dirName, "2.md"), filepath.Join(dirName, "A.md"), filepath.Join(dirName, "B.md"), filepath.Join(dirName, "a.MD"), filepath.Join(dirName, "b.md"), filepath.Join(dirName, "eind.md"), filepath.Join(dirName, "hoofdstuk.md"), filepath.Join(dirName, "index.md"), filepath.Join(dirName, "laatste.md"), filepath.Join(dirName, "z.md"),
	}
}

func separateTasks(dirName, targetDir string) []Task {
	sources := wikiSources(dirName)
	tasks := make([]Task, len(sources))
	for i, source := range sources {
		target := pdfName(source)
		if targetDir != "" {
			target = filepath.Join(targetDir, filepath.Base(target))
		}
		tasks[i] = Task{Sources: []string{source}, Target: target}
	}
	return tasks
}
