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
	if options.Diagram == nil && options.Mermaid.Available() {
		// One cache for the whole run: a diagram that appears in several files is rendered once.
		options.Diagram = render.NewDiagramCache(options.Mermaid.ToPNG)
	}
	read := newSourceCache()
	if options.Diagram != nil && options.Mermaid.Available() {
		prerender(plan, options, read)
	}
	switch plan.Mode {
	case cli.ModeSingle:
		if len(plan.Tasks) != 1 || len(plan.Tasks[0].Sources) != 1 {
			return errors.New("invalid output plan for a single file")
		}
		return renderTask(plan, plan.Tasks[0], options, read)
	case cli.ModeMerged:
		if len(plan.Tasks) != 1 || len(plan.Tasks[0].Sources) == 0 {
			return errors.New("invalid output plan for merged files")
		}
		return renderMerged(plan, plan.Tasks[0], options, read)
	case cli.ModeSeparate:
		failed := 0
		for _, task := range plan.Tasks {
			if len(task.Sources) != 1 {
				return errors.New("invalid output plan for separate files")
			}
			err := os.MkdirAll(filepath.Dir(task.Target), 0o755)
			if err != nil {
				err = fmt.Errorf("cannot write %q: %w", task.Target, err)
			} else {
				err = renderTask(plan, task, options, read)
			}
			if err == nil {
				continue
			}
			if plan.Strict {
				return err
			}
			failed++
			if options.Warn != nil {
				options.Warn(fmt.Sprintf("skipped %q: %v", task.Sources[0], err))
			}
		}
		if failed > 0 {
			noun := "files"
			if len(plan.Tasks) == 1 {
				noun = "file"
			}
			return fmt.Errorf("%d of %d %s failed", failed, len(plan.Tasks), noun)
		}
		return nil
	default:
		return errors.New("unknown output mode")
	}
}

func renderTask(plan cli.Plan, task cli.Task, options render.Options, read func(string) (source, error)) error {
	source, err := read(task.Sources[0])
	if err != nil {
		return err
	}
	document := pdfout.NewWithLayout(pdfout.Layout{Paper: plan.Paper, Margin: plan.Margin})
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

func renderMerged(plan cli.Plan, task cli.Task, options render.Options, read func(string) (source, error)) error {
	document := pdfout.NewWithLayout(pdfout.Layout{Paper: plan.Paper, Margin: plan.Margin})
	canvas := document.Canvas()
	for i, path := range task.Sources {
		source, err := read(path)
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

// newSourceCache returns readSource with memory: every file of a run is read
// and parsed once, although diagrams are collected before drawing starts.
func newSourceCache() func(string) (source, error) {
	type result struct {
		source source
		err    error
	}
	results := map[string]result{}
	return func(path string) (source, error) {
		if r, ok := results[path]; ok {
			return r.source, r.err
		}
		s, err := readSource(path)
		results[path] = result{s, err}
		return s, err
	}
}

func readSource(path string) (source, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return source{}, fmt.Errorf("cannot read %q: %w", path, err)
	}
	meta, _ := markdown.SplitFrontMatter(data)
	blocks, err := markdown.ParseIn(data, filepath.Dir(path))
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

// prerender renders the distinct diagrams of all sources of the run at the same
// time, before anything is drawn. A source that cannot be read is skipped here;
// drawing reports it in the usual way.
func prerender(plan cli.Plan, options render.Options, read func(string) (source, error)) {
	seen := map[string]bool{}
	var texts []string
	for _, task := range plan.Tasks {
		for _, path := range task.Sources {
			source, err := read(path)
			if err != nil {
				continue
			}
			for _, text := range render.DiagramTexts(source.blocks) {
				if !seen[text] {
					seen[text] = true
					texts = append(texts, text)
				}
			}
		}
	}
	render.Prerender(options.Diagram, texts, render.Workers())
}
