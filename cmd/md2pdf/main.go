package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/gnutterts/md2pdf/internal/cli"
	"github.com/gnutterts/md2pdf/internal/markdown"
	"github.com/gnutterts/md2pdf/internal/pdfout"
	"github.com/gnutterts/md2pdf/internal/render"
)

const versie = "md2pdf 0.1.0"

func main() {
	if err := uitvoeren(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "fout:", err)
		os.Exit(1)
	}
}

func uitvoeren(args []string) error {
	plan, err := cli.Plannen(args, cli.OSBestandssysteem{})
	if errors.Is(err, cli.ErrHulp) {
		fmt.Println(cli.Gebruik)
		return nil
	}
	if errors.Is(err, cli.ErrVersie) {
		fmt.Println(versie)
		return nil
	}
	if err != nil {
		return err
	}
	return renderen(plan)
}

func renderen(plan cli.Plan) error {
	if plan.Modus != cli.ModusEnkel {
		return errors.New("nog niet geïmplementeerd: samengevoegde en losse uitvoer volgen in de volgende mijlpaal")
	}
	if len(plan.Taken) != 1 || len(plan.Taken[0].Bronnen) != 1 {
		return errors.New("ongeldig uitvoerplan voor één bestand")
	}
	taak := plan.Taken[0]
	bron, err := os.ReadFile(taak.Bronnen[0])
	if err != nil {
		return err
	}
	blokken, err := markdown.Ontleed(bron)
	if err != nil {
		return err
	}
	document := pdfout.Nieuw()
	if err := render.Teken(blokken, document.Vel()); err != nil {
		return err
	}
	return document.Schrijf(taak.Doel)
}
