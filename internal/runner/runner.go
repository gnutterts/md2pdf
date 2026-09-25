// SPDX-License-Identifier: MIT

// Package runner turns output plans into PDF files.
package runner

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
		return renderTask(plan, plan.Tasks[0], options)
	case cli.ModeMerged:
		if len(plan.Tasks) != 1 || len(plan.Tasks[0].Sources) == 0 {
			return errors.New("invalid output plan for merged files")
		}
		return renderMerged(plan, plan.Tasks[0], options)
	case cli.ModeSeparate:
		for _, task := range plan.Tasks {
			if len(task.Sources) != 1 {
				return errors.New("invalid output plan for separate files")
			}
			if err := os.MkdirAll(filepath.Dir(task.Target), 0o755); err != nil {
				return fmt.Errorf("cannot write %q: %w", task.Target, err)
			}
			if err := renderTask(plan, task, options); err != nil {
				return err
			}
		}
		return nil
	default:
		return errors.New("unknown output mode")
	}
}

func renderTask(plan cli.Plan, task cli.Task, options render.Options) error {
	source, err := readSource(task.Sources[0])
	if err != nil {
		return err
	}
	document := pdfout.New()
	document.SetInfo(documentTitle(plan, task, source), documentAuthor(plan, source), plan.Creator)
	document.SetPageNumbers(!plan.NoPageNumbers)
	if err := render.Draw(source.blocks, document.Canvas(), options); err != nil {
		return err
	}
	if err := document.Write(task.Target); err != nil {
		return fmt.Errorf("cannot write %q: %w", task.Target, err)
	}
	return nil
}

func renderMerged(plan cli.Plan, task cli.Task, options render.Options) error {
	document := pdfout.New()
	canvas := document.Canvas()
	for i, path := range task.Sources {
		source, err := readSource(path)
		if err != nil {
			return err
		}
		if i == 0 {
			document.SetInfo(documentTitle(plan, task, source), documentAuthor(plan, source), plan.Creator)
			document.SetPageNumbers(!plan.NoPageNumbers)
		}
		if i > 0 {
			canvas.NewPage()
		}
		if err := render.Draw(source.blocks, canvas, options); err != nil {
			return err
		}
	}
	if err := document.Write(task.Target); err != nil {
		return fmt.Errorf("cannot write %q: %w", task.Target, err)
	}
	return nil
}

// source is a parsed Markdown file with its front matter fields.
type source struct {
	path   string
	meta   markdown.Metadata
	blocks []markdown.Block
}

func readSource(path string) (source, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return source{}, fmt.Errorf("cannot read %q: %w", path, err)
	}
	meta, _ := markdown.SplitFrontMatter(data)
	blocks, err := markdown.Parse(data)
	if err != nil {
		return source{}, err
	}
	return source{path: path, meta: meta, blocks: blocks}, nil
}

// documentTitle is --title, else the front matter title of the first file,
// else its first level 1 heading, else the file name (or the directory name
// when files are merged).
func documentTitle(plan cli.Plan, task cli.Task, first source) string {
	if plan.Title != "" {
		return plan.Title
	}
	if title := first.meta.Get("title"); title != "" {
		return title
	}
	if title := markdown.FirstHeading(first.blocks); title != "" {
		return title
	}
	if plan.Mode == cli.ModeMerged {
		if abs, err := filepath.Abs(filepath.Dir(first.path)); err == nil {
			return filepath.Base(abs)
		}
	}
	name := filepath.Base(first.path)
	return strings.TrimSuffix(name, filepath.Ext(name))
}

// documentAuthor is --author, else the front matter author of the first file.
func documentAuthor(plan cli.Plan, first source) string {
	if plan.Author != "" {
		return plan.Author
	}
	return first.meta.Get("author")
}
