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
	return renderMerged(plan, task, options, read)
}

// renderMerged draws the sources of a task one after another, each on a new
// page, into one PDF; with a table of contents, in two passes.
func renderMerged(plan cli.Plan, task cli.Task, options render.Options, read func(string) (source, error)) error {
	var document *pdfout.Document
	var err error
	if plan.TOC {
		document, err = buildWithTOC(plan, task, options, read)
	} else {
		document, err = build(plan, task, options, read, nil, 0)
	}
	if err != nil {
		return err
	}
	if err := document.Write(task.Target); err != nil {
		return fmt.Errorf("cannot write %q: %w", task.Target, err)
	}
	return nil
}

// build draws a task into a new document, after a table of contents of toc
// whose page numbers are raised by offset when toc is not empty.
func build(plan cli.Plan, task cli.Task, options render.Options, read func(string) (source, error), toc []pdfout.Heading, offset int) (*pdfout.Document, error) {
	document := pdfout.NewWithLayout(layout(plan))
	canvas := document.Canvas()
	for i, path := range task.Sources {
		source, err := read(path)
		if err != nil {
			return nil, err
		}
		if i == 0 {
			document.SetInfo(documentTitle(plan, task, source), documentAuthor(plan, source), plan.Creator)
			document.SetPageNumbers(!plan.NoPageNumbers)
			if len(toc) > 0 {
				document.TOC(toc, offset)
			}
		}
		if i > 0 {
			canvas.NewPage()
		}
		if err := render.Draw(source.blocks, canvas, options); err != nil {
			return nil, err
		}
	}
	return document, nil
}

func layout(plan cli.Plan) pdfout.Layout {
	return pdfout.Layout{Paper: plan.Paper, Margin: plan.Margin, Font: plan.Font, MonoFont: plan.MonoFont}
}

// buildWithTOC draws the document once to learn where the headings fall, then
// again behind a table of contents that points at them. The pages of the table
// shift everything after it, so the second pass is checked against the first.
func buildWithTOC(plan cli.Plan, task cli.Task, options render.Options, read func(string) (source, error)) (*pdfout.Document, error) {
	quiet := options
	quiet.Warn = nil // the second pass reports the warnings
	first, err := build(plan, task, quiet, read, nil, 0)
	if err != nil {
		return nil, err
	}
	depth := plan.TOCDepth
	if depth == 0 {
		depth = 3
	}
	var entries []pdfout.Heading
	for _, heading := range first.Headings() {
		if heading.Level <= depth {
			entries = append(entries, heading)
		}
	}
	if len(entries) == 0 {
		if options.Warn != nil {
			options.Warn(fmt.Sprintf("%q has no headings up to level %d; no table of contents", task.Target, depth))
		}
		return build(plan, task, options, read, nil, 0)
	}
	// Every entry takes one line whatever its page number, so the length of the
	// table is known before the page numbers are.
	offset := pdfout.TOCPages(layout(plan), entries)
	document, err := build(plan, task, options, read, entries, offset)
	if err != nil {
		return nil, err
	}
	// Every heading must be exactly offset pages later than in the first pass.
	placed := document.Headings()
	all := first.Headings()
	if len(placed) != len(all) {
		return nil, fmt.Errorf("table of contents: %d headings in the first pass, %d in the second", len(all), len(placed))
	}
	for i := range all {
		if placed[i].Page != all[i].Page+offset {
			return nil, fmt.Errorf("table of contents: heading %q moved from page %d to %d", all[i].Text, all[i].Page+offset, placed[i].Page)
		}
	}
	return document, nil
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
