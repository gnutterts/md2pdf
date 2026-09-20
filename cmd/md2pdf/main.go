package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/gnutterts/md2pdf/internal/cli"
	"github.com/gnutterts/md2pdf/internal/mermaid"
	"github.com/gnutterts/md2pdf/internal/render"
	"github.com/gnutterts/md2pdf/internal/uitvoer"
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
	opties := render.Opties{
		Mermaid: mermaid.Choose(plan.Mermaid, os.Getenv("MD2PDF_MERMAID")),
		Waarschuw: func(melding string) {
			fmt.Fprintln(os.Stderr, "waarschuwing:", melding)
		},
	}
	return uitvoer.Voer(plan, opties)
}
