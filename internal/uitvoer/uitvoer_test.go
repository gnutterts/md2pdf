package uitvoer

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gnutterts/md2pdf/internal/cli"
)

func TestVoerEnkelSchrijftPDF(t *testing.T) {
	doel := filepath.Join(t.TempDir(), "README.pdf")
	plan := cli.Plan{Modus: cli.ModusEnkel, Taken: []cli.Taak{{
		Bronnen: []string{leesPad("README.md")}, Doel: doel,
	}}}
	if err := Voer(plan); err != nil {
		t.Fatal(err)
	}
	inhoud, err := os.ReadFile(doel)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(inhoud, []byte("%PDF-")) {
		t.Fatalf("%q is geen PDF", doel)
	}
}

func TestVoerSamengevoegdSchrijftPaginas(t *testing.T) {
	bronnen := wikiBronnen()
	doel := filepath.Join(t.TempDir(), "wiki.pdf")
	if err := Voer(cli.Plan{Modus: cli.ModusSamengevoegd, Taken: []cli.Taak{{Bronnen: bronnen, Doel: doel}}}); err != nil {
		t.Fatal(err)
	}
	inhoud, err := os.ReadFile(doel)
	if err != nil {
		t.Fatal(err)
	}
	if paginas := aantalPaginas(inhoud); paginas < len(bronnen) {
		t.Fatalf("PDF heeft %d pagina's, wil minstens %d", paginas, len(bronnen))
	}

	enkel := filepath.Join(t.TempDir(), "enkel.pdf")
	if err := Voer(cli.Plan{Modus: cli.ModusEnkel, Taken: []cli.Taak{{Bronnen: bronnen[:1], Doel: enkel}}}); err != nil {
		t.Fatal(err)
	}
	enkeleInhoud, err := os.ReadFile(enkel)
	if err != nil {
		t.Fatal(err)
	}
	if len(inhoud) <= len(enkeleInhoud) {
		t.Fatalf("samengevoegde PDF is %d bytes, enkele %d", len(inhoud), len(enkeleInhoud))
	}
}

func TestVoerLosSchrijftBestanden(t *testing.T) {
	mapnaam := t.TempDir()
	bronnen := wikiBronnen()
	taken := make([]cli.Taak, len(bronnen))
	for i, bron := range bronnen {
		taken[i] = cli.Taak{Bronnen: []string{bron}, Doel: filepath.Join(mapnaam, strings.TrimSuffix(filepath.Base(bron), ".md")+".pdf")}
	}
	if err := Voer(cli.Plan{Modus: cli.ModusLos, Taken: taken}); err != nil {
		t.Fatal(err)
	}
	for _, taak := range taken {
		taak := taak
		t.Run(filepath.Base(taak.Doel), func(t *testing.T) {
			info, err := os.Stat(taak.Doel)
			if err != nil {
				t.Fatal(err)
			}
			if info.Size() == 0 {
				t.Fatalf("%q is leeg", taak.Doel)
			}
		})
	}
}

func TestVoerLosMaaktDoelmap(t *testing.T) {
	doel := filepath.Join(t.TempDir(), "nieuw", "README.pdf")
	plan := cli.Plan{Modus: cli.ModusLos, Taken: []cli.Taak{{Bronnen: []string{leesPad("README.md")}, Doel: doel}}}
	if err := Voer(plan); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Dir(doel)); err != nil {
		t.Fatal(err)
	}
}

func TestVoerOntbrekendeBronGeeftFout(t *testing.T) {
	bron := filepath.Join(t.TempDir(), "ontbreekt.md")
	err := Voer(cli.Plan{Modus: cli.ModusEnkel, Taken: []cli.Taak{{Bronnen: []string{bron}, Doel: filepath.Join(t.TempDir(), "uit.pdf")}}})
	if err == nil || !strings.Contains(err.Error(), bron) {
		t.Fatalf("fout = %v, wil pad %q", err, bron)
	}
}

func TestVoerKanNietSchrijven(t *testing.T) {
	mapnaam := filepath.Join(t.TempDir(), "alleen-lezen")
	if err := os.Mkdir(mapnaam, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(mapnaam, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(mapnaam, 0o755) })

	err := Voer(cli.Plan{Modus: cli.ModusEnkel, Taken: []cli.Taak{{
		Bronnen: []string{leesPad("README.md")}, Doel: filepath.Join(mapnaam, "uit.pdf"),
	}}})
	if err == nil {
		t.Fatal("schrijven in een alleen-lezen map lukte")
	}
}

func aantalPaginas(inhoud []byte) int {
	const kenmerk = "/Type /Page"
	paginas := 0
	for {
		index := bytes.Index(inhoud, []byte(kenmerk))
		if index < 0 {
			return paginas
		}
		inhoud = inhoud[index+len(kenmerk):]
		if len(inhoud) == 0 || inhoud[0] != 's' {
			paginas++
		}
	}
}

func leesPad(naam string) string {
	return filepath.Join("..", "..", "testdata", naam)
}

func wikiBronnen() []string {
	namen := []string{
		"Alpha.md", "Bravo.md", "Charlie.md", "Delta.md", "Echo.md",
		"Foxtrot.md", "Golf.md", "Hotel.md", "India.md",
		"Juliett.md", "Kilo.md",
	}
	bronnen := make([]string, len(namen))
	for i, naam := range namen {
		bronnen[i] = leesPad(filepath.Join("wiki", naam))
	}
	return bronnen
}
