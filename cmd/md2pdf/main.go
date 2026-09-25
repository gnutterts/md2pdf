// SPDX-License-Identifier: MIT

package main

import (
	"errors"
	"fmt"
	"os"
	"runtime/debug"

	"github.com/gnutterts/md2pdf/internal/cli"
	"github.com/gnutterts/md2pdf/internal/mermaid"
	"github.com/gnutterts/md2pdf/internal/render"
	"github.com/gnutterts/md2pdf/internal/runner"
)

const version = "md2pdf 0.3.0"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(args []string) error { return runWith(args, runner.Run) }

// runWith runs md2pdf with the given executor. A panic anywhere below becomes
// an ordinary error ("internal error: ..."); MD2PDF_DEBUG=1 adds the stack.
func runWith(args []string, execute func(cli.Plan, render.Options) error) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("internal error: %v", recovered)
			if os.Getenv("MD2PDF_DEBUG") != "" {
				fmt.Fprintln(os.Stderr, string(debug.Stack()))
			}
		}
	}()
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
	scale, err := mermaid.ChooseScale(plan.Scale, os.Getenv("MD2PDF_SCALE"))
	if err != nil {
		return err
	}
	timeout, err := mermaid.ChooseTimeout(plan.MermaidTimeout, os.Getenv("MD2PDF_MERMAID_TIMEOUT"))
	if err != nil {
		return err
	}
	renderer := mermaid.Choose(plan.Mermaid, os.Getenv("MD2PDF_MERMAID"))
	renderer.Scale = scale
	renderer.Timeout = timeout
	plan.Creator = version
	options := render.Options{
		Strict:  plan.Strict,
		Mermaid: renderer,
		Warn: func(message string) {
			fmt.Fprintln(os.Stderr, "warning:", message)
		},
	}
	return execute(plan, options)
}
