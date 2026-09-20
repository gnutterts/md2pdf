// Package cli maakt een uitvoerplan voor md2pdf.
package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Modus bepaalt hoe de opgegeven invoer wordt verwerkt.
type Modus int

const (
	// ModusEnkel maakt één PDF van één bestand.
	ModusEnkel Modus = iota
	// ModusSamengevoegd maakt één PDF van alle Markdown-bestanden in een map.
	ModusSamengevoegd
	// ModusLos maakt één PDF per Markdown-bestand in een map.
	ModusLos
)

// Taak beschrijft bronnen en hun doel.
type Taak struct {
	Bronnen []string
	Doel    string
}

// Plan is de volledige, geordende verwerking.
type Plan struct {
	Modus   Modus
	Taken   []Taak
	Mermaid string
}

// Bestandssysteem bevat de leesbewerkingen die voor plannen nodig zijn.
type Bestandssysteem interface {
	Bestaat(pad string) (isMap bool, err error)
	LeesMap(pad string) ([]string, error)
}

// OSBestandssysteem gebruikt het lokale bestandssysteem.
type OSBestandssysteem struct{}

// Bestaat meldt of pad een map is.
func (OSBestandssysteem) Bestaat(pad string) (bool, error) {
	info, err := os.Stat(pad)
	if err != nil {
		return false, err
	}
	return info.IsDir(), nil
}

// LeesMap leest de namen direct in een map.
func (OSBestandssysteem) LeesMap(pad string) ([]string, error) {
	items, err := os.ReadDir(pad)
	if err != nil {
		return nil, err
	}
	namen := make([]string, len(items))
	for i, item := range items {
		namen[i] = item.Name()
	}
	return namen, nil
}

var (
	// ErrHulp vraagt de entrypoint om de gebruikstekst af te drukken.
	ErrHulp = errors.New("hulp gevraagd")
	// ErrVersie vraagt de entrypoint om de versie af te drukken.
	ErrVersie = errors.New("versie gevraagd")
)

// Gebruik is de korte gebruikstekst voor de opdracht.
const Gebruik = "Gebruik: md2pdf [-o pad] [--los|-l] [--mermaid pad] <bestand-of-map>"

// Plannen zet argumenten om in een volledig uitvoerplan.
func Plannen(args []string, fs Bestandssysteem) (Plan, error) {
	los, uitvoer, mermaid, posities, err := ontleed(args)
	if err != nil {
		return Plan{}, err
	}
	if len(posities) == 0 {
		return Plan{}, fmt.Errorf("geen invoer opgegeven\n%s", Gebruik)
	}
	if len(posities) != 1 {
		return Plan{}, errors.New("precies één invoerpad is vereist")
	}

	invoer := posities[0]
	isMap, err := fs.Bestaat(invoer)
	if err != nil {
		return Plan{}, fmt.Errorf("kan %q niet lezen: %w", invoer, err)
	}
	if !isMap {
		return planBestand(invoer, uitvoer, mermaid, los, fs)
	}
	return planMap(invoer, uitvoer, mermaid, los, fs)
}

// ontleed splitst vlaggen van positionele argumenten. ErrHulp en ErrVersie
// komen als fout terug; de aanroeper herkent ze met errors.Is.
func ontleed(args []string) (bool, string, string, []string, error) {
	var los bool
	var uitvoer, mermaid string
	var posities []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "-h", "--help":
			return false, "", "", nil, ErrHulp
		case "--version":
			return false, "", "", nil, ErrVersie
		case "--los", "-l":
			los = true
		case "-o":
			if i+1 == len(args) {
				return false, "", "", nil, errors.New("-o verwacht een pad")
			}
			i++
			uitvoer = args[i]
		case "--mermaid":
			if i+1 == len(args) {
				return false, "", "", nil, errors.New("--mermaid verwacht een pad")
			}
			i++
			mermaid = args[i]
		case "--":
			posities = append(posities, args[i+1:]...)
			i = len(args)
		default:
			if strings.HasPrefix(arg, "-") {
				return false, "", "", nil, fmt.Errorf("onbekende vlag: %s", arg)
			}
			posities = append(posities, arg)
		}
	}
	return los, uitvoer, mermaid, posities, nil
}

func planBestand(invoer, uitvoer, mermaid string, los bool, fs Bestandssysteem) (Plan, error) {
	if los {
		return Plan{}, errors.New("--los werkt alleen op een map")
	}
	if uitvoer == "" {
		uitvoer = pdfNaam(invoer)
	} else if err := bestandDoel(uitvoer, fs); err != nil {
		return Plan{}, err
	}
	return Plan{Modus: ModusEnkel, Taken: []Taak{{Bronnen: []string{invoer}, Doel: uitvoer}}, Mermaid: mermaid}, nil
}

func planMap(invoer, uitvoer, mermaid string, los bool, fs Bestandssysteem) (Plan, error) {
	bronnen, err := markdownBestanden(invoer, fs)
	if err != nil {
		return Plan{}, err
	}
	if los {
		if uitvoer != "" {
			if err := mapDoel(uitvoer, fs); err != nil {
				return Plan{}, err
			}
		}
		taken := make([]Taak, len(bronnen))
		for i, bron := range bronnen {
			doel := pdfNaam(bron)
			if uitvoer != "" {
				doel = filepath.Join(uitvoer, filepath.Base(doel))
			}
			taken[i] = Taak{Bronnen: []string{bron}, Doel: doel}
		}
		return Plan{Modus: ModusLos, Taken: taken, Mermaid: mermaid}, nil
	}
	if uitvoer == "" {
		uitvoer = filepath.Clean(invoer) + ".pdf"
	} else if err := bestandDoel(uitvoer, fs); err != nil {
		return Plan{}, err
	}
	return Plan{Modus: ModusSamengevoegd, Taken: []Taak{{Bronnen: bronnen, Doel: uitvoer}}, Mermaid: mermaid}, nil
}

func markdownBestanden(mapnaam string, fs Bestandssysteem) ([]string, error) {
	namen, err := fs.LeesMap(mapnaam)
	if err != nil {
		return nil, fmt.Errorf("kan map %q niet lezen: %w", mapnaam, err)
	}
	var markdown []string
	var heeftMarkdown bool
	for _, naam := range namen {
		if strings.HasSuffix(strings.ToLower(naam), ".md") {
			heeftMarkdown = true
			if !strings.HasPrefix(naam, "_") {
				markdown = append(markdown, naam)
			}
		}
	}
	sort.Strings(markdown)
	if len(markdown) == 0 {
		if heeftMarkdown {
			return nil, fmt.Errorf("map %q bevat geen Markdown-bestanden (namen die met _ beginnen worden overgeslagen)", mapnaam)
		}
		return nil, fmt.Errorf("map %q bevat geen Markdown-bestanden", mapnaam)
	}
	bronnen := make([]string, len(markdown))
	for i, naam := range markdown {
		bronnen[i] = filepath.Join(mapnaam, naam)
	}
	return bronnen, nil
}

func bestandDoel(doel string, fs Bestandssysteem) error {
	isMap, err := fs.Bestaat(doel)
	if err == nil && isMap {
		return errors.New("-o verwijst naar een map, maar hier is een bestandsnaam nodig")
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("kan uitvoerdoel %q niet lezen: %w", doel, err)
	}
	return nil
}

func mapDoel(doel string, fs Bestandssysteem) error {
	isMap, err := fs.Bestaat(doel)
	if err == nil && !isMap {
		return errors.New("-o verwijst naar een bestand, maar bij --los is een map nodig")
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("kan uitvoerdoel %q niet lezen: %w", doel, err)
	}
	return nil
}

func pdfNaam(bron string) string {
	extensie := filepath.Ext(bron)
	if strings.EqualFold(extensie, ".md") {
		return strings.TrimSuffix(bron, extensie) + ".pdf"
	}
	return bron + ".pdf"
}
