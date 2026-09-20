// Package runner turns output plans into PDF files.
package runner

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

// Run processes all tasks in plan.
func Run(plan cli.Plan, options render.Options) error {
	switch plan.Mode {
	case cli.ModeSingle:
		if len(plan.Tasks) != 1 || len(plan.Tasks[0].Sources) != 1 {
			return errors.New("invalid output plan for a single file")
		}
		return renderTask(plan.Tasks[0], options)
	case cli.ModeMerged:
		if len(plan.Tasks) != 1 || len(plan.Tasks[0].Sources) == 0 {
			return errors.New("invalid output plan for merged files")
		}
		return renderMerged(plan.Tasks[0], options)
	case cli.ModeSeparate:
		for _, task := range plan.Tasks {
			if len(task.Sources) != 1 {
				return errors.New("invalid output plan for separate files")
			}
			if err := os.MkdirAll(filepath.Dir(task.Target), 0o755); err != nil {
				return fmt.Errorf("cannot write %q: %w", task.Target, err)
			}
			if err := renderTask(task, options); err != nil {
				return err
			}
		}
		return nil
	default:
		return errors.New("unknown output mode")
	}
}

func renderTask(task cli.Task, options render.Options) error {
	blocks, err := readBlocks(task.Sources[0])
	if err != nil {
		return err
	}
	document := pdfout.New()
	if err := render.Draw(blocks, document.Canvas(), options); err != nil {
		return err
	}
	if err := document.Write(task.Target); err != nil {
		return fmt.Errorf("cannot write %q: %w", task.Target, err)
	}
	return nil
}

func renderMerged(task cli.Task, options render.Options) error {
	document := pdfout.New()
	canvas := document.Canvas()
	for i, source := range task.Sources {
		blocks, err := readBlocks(source)
		if err != nil {
			return err
		}
		if i > 0 {
			canvas.NewPage()
		}
		if err := render.Draw(blocks, canvas, options); err != nil {
			return err
		}
	}
	if err := document.Write(task.Target); err != nil {
		return fmt.Errorf("cannot write %q: %w", task.Target, err)
	}
	return nil
}

func readBlocks(path string) ([]markdown.Block, error) {
	source, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read %q: %w", path, err)
	}
	return markdown.Parse(source)
}
