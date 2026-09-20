package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/gnutterts/md2pdf/internal/cli"
	"github.com/gnutterts/md2pdf/internal/mermaid"
	"github.com/gnutterts/md2pdf/internal/render"
	"github.com/gnutterts/md2pdf/internal/runner"
)

const version = "md2pdf 0.1.0"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	plan, err := cli.Parse(args, cli.OSFileSystem{})
	if errors.Is(err, cli.ErrHelp) {
		fmt.Println(cli.Usage)
		return nil
	}
	if errors.Is(err, cli.ErrVersion) {
		fmt.Println(version)
		return nil
	}
	if err != nil {
		return err
	}
	options := render.Options{
		Mermaid: mermaid.Choose(plan.Mermaid, os.Getenv("MD2PDF_MERMAID")),
		Warn: func(message string) {
			fmt.Fprintln(os.Stderr, "warning:", message)
		},
	}
	return runner.Run(plan, options)
}
