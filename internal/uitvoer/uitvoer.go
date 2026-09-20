// Package uitvoer zet uitvoerplannen om in PDF-bestanden.
package uitvoer

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/gnutterts/md2pdf/internal/cli"
	"github.com/gnutterts/md2pdf/internal/markdown"
	"github.com/gnutterts/md2pdf/internal/pdfout"
	"github.com/gnutterts/md2pdf/internal/render"
)

// Voer verwerkt alle taken in plan.
func Voer(plan cli.Plan, opties render.Opties) error {
	switch plan.Modus {
	case cli.ModusEnkel:
		if len(plan.Taken) != 1 || len(plan.Taken[0].Bronnen) != 1 {
			return errors.New("ongeldig uitvoerplan voor één bestand")
		}
		return tekenTaak(plan.Taken[0], opties)
	case cli.ModusSamengevoegd:
		if len(plan.Taken) != 1 || len(plan.Taken[0].Bronnen) == 0 {
			return errors.New("ongeldig uitvoerplan voor samengevoegde bestanden")
		}
		return tekenSamengevoegd(plan.Taken[0], opties)
	case cli.ModusLos:
		for _, taak := range plan.Taken {
			if len(taak.Bronnen) != 1 {
				return errors.New("ongeldig uitvoerplan voor losse bestanden")
			}
			if err := os.MkdirAll(filepath.Dir(taak.Doel), 0o755); err != nil {
				return fmt.Errorf("kan %q niet schrijven: %w", taak.Doel, err)
			}
			if err := tekenTaak(taak, opties); err != nil {
				return err
			}
		}
		return nil
	default:
		return errors.New("onbekende uitvoermodus")
	}
}

func tekenTaak(taak cli.Taak, opties render.Opties) error {
	blokken, err := leesBlokken(taak.Bronnen[0])
	if err != nil {
		return err
	}
	document := pdfout.Nieuw()
	if err := render.Teken(blokken, document.Vel(), opties); err != nil {
		return err
	}
	if err := document.Schrijf(taak.Doel); err != nil {
		return fmt.Errorf("kan %q niet schrijven: %w", taak.Doel, err)
	}
	return nil
}

func tekenSamengevoegd(taak cli.Taak, opties render.Opties) error {
	document := pdfout.Nieuw()
	vel := document.Vel()
	for i, bron := range taak.Bronnen {
		blokken, err := leesBlokken(bron)
		if err != nil {
			return err
		}
		if i > 0 {
			vel.NieuwePagina()
		}
		if err := render.Teken(blokken, vel, opties); err != nil {
			return err
		}
	}
	if err := document.Schrijf(taak.Doel); err != nil {
		return fmt.Errorf("kan %q niet schrijven: %w", taak.Doel, err)
	}
	return nil
}

func leesBlokken(pad string) ([]markdown.Block, error) {
	bron, err := os.ReadFile(pad)
	if err != nil {
		return nil, fmt.Errorf("kan %q niet lezen: %w", pad, err)
	}
	return markdown.Parse(bron)
}
