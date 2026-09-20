package runner

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gnutterts/md2pdf/internal/cli"
	"github.com/gnutterts/md2pdf/internal/mermaid"
	"github.com/gnutterts/md2pdf/internal/render"
)

func TestRunMermaidErrorWarnsAndContinues(t *testing.T) {
	dirName := t.TempDir()
	source := filepath.Join(dirName, "diagram.md")
	if err := os.WriteFile(source, []byte("```mermaid\ngraph TD\nA-->B\n```\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(dirName, "fails")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(dirName, "diagram.pdf")
	warnings := 0
	options := render.Options{Mermaid: mermaid.Renderer{Path: script}, Warn: func(string) { warnings++ }}
	if err := Run(cli.Plan{Mode: cli.ModeSingle, Tasks: []cli.Task{{Sources: []string{source}, Target: target}}}, options); err != nil {
		t.Fatal(err)
	}
	if warnings != 1 {
		t.Fatalf("warnings = %d, want 1", warnings)
	}
	if content, err := os.ReadFile(target); err != nil || !bytes.HasPrefix(content, []byte("%PDF-")) {
		t.Fatalf("PDF is missing of is invalid: %v", err)
	}
}

func TestRunSingleWritesPDF(t *testing.T) {
	target := filepath.Join(t.TempDir(), "README.pdf")
	plan := cli.Plan{Mode: cli.ModeSingle, Tasks: []cli.Task{{
		Sources: []string{readPath("README.md")}, Target: target,
	}}}
	if err := Run(plan, render.Options{}); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(content, []byte("%PDF-")) {
		t.Fatalf("%q is not a PDF", target)
	}
}

func TestRunMergedWritesPages(t *testing.T) {
	sources := wikiSources()
	target := filepath.Join(t.TempDir(), "wiki.pdf")
	if err := Run(cli.Plan{Mode: cli.ModeMerged, Tasks: []cli.Task{{Sources: sources, Target: target}}}, render.Options{}); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if pages := pageCount(content); pages < len(sources) {
		t.Fatalf("PDF heeft %d pages, want minstens %d", pages, len(sources))
	}

	single := filepath.Join(t.TempDir(), "single.pdf")
	if err := Run(cli.Plan{Mode: cli.ModeSingle, Tasks: []cli.Task{{Sources: sources[:1], Target: single}}}, render.Options{}); err != nil {
		t.Fatal(err)
	}
	singleContent, err := os.ReadFile(single)
	if err != nil {
		t.Fatal(err)
	}
	if len(content) <= len(singleContent) {
		t.Fatalf("mergede PDF is %d bytes, singlee %d", len(content), len(singleContent))
	}
}

func TestRunSeparateWritesFiles(t *testing.T) {
	dirName := t.TempDir()
	sources := wikiSources()
	tasks := make([]cli.Task, len(sources))
	for i, source := range sources {
		tasks[i] = cli.Task{Sources: []string{source}, Target: filepath.Join(dirName, strings.TrimSuffix(filepath.Base(source), ".md")+".pdf")}
	}
	if err := Run(cli.Plan{Mode: cli.ModeSeparate, Tasks: tasks}, render.Options{}); err != nil {
		t.Fatal(err)
	}
	for _, task := range tasks {
		task := task
		t.Run(filepath.Base(task.Target), func(t *testing.T) {
			info, err := os.Stat(task.Target)
			if err != nil {
				t.Fatal(err)
			}
			if info.Size() == 0 {
				t.Fatalf("%q is empty", task.Target)
			}
		})
	}
}

func TestRunSeparateCreatesTargetDir(t *testing.T) {
	target := filepath.Join(t.TempDir(), "nieuw", "README.pdf")
	plan := cli.Plan{Mode: cli.ModeSeparate, Tasks: []cli.Task{{Sources: []string{readPath("README.md")}, Target: target}}}
	if err := Run(plan, render.Options{}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Dir(target)); err != nil {
		t.Fatal(err)
	}
}

func TestRunMissingSourceReturnsError(t *testing.T) {
	source := filepath.Join(t.TempDir(), "is missing.md")
	err := Run(cli.Plan{Mode: cli.ModeSingle, Tasks: []cli.Task{{Sources: []string{source}, Target: filepath.Join(t.TempDir(), "out.pdf")}}}, render.Options{})
	if err == nil || !strings.Contains(err.Error(), source) {
		t.Fatalf("error = %v, want path %q", err, source)
	}
}

func TestRunCannotWrite(t *testing.T) {
	dirName := filepath.Join(t.TempDir(), "read-only")
	if err := os.Mkdir(dirName, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dirName, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dirName, 0o755) })

	err := Run(cli.Plan{Mode: cli.ModeSingle, Tasks: []cli.Task{{
		Sources: []string{readPath("README.md")}, Target: filepath.Join(dirName, "out.pdf"),
	}}}, render.Options{})
	if err == nil {
		t.Fatal("writing to a read-only directory succeeded")
	}
}

func pageCount(content []byte) int {
	const marker = "/Type /Page"
	pages := 0
	for {
		index := bytes.Index(content, []byte(marker))
		if index < 0 {
			return pages
		}
		content = content[index+len(marker):]
		if len(content) == 0 || content[0] != 's' {
			pages++
		}
	}
}

func readPath(name string) string {
	return filepath.Join("..", "..", "testdata", name)
}

func wikiSources() []string {
	names := []string{
		"Alpha.md", "Bravo.md", "Charlie.md", "Delta.md", "Echo.md",
		"Foxtrot.md", "Golf.md", "Hotel.md", "India.md",
		"Juliett.md", "Kilo.md",
	}
	sources := make([]string, len(names))
	for i, name := range names {
		sources[i] = readPath(filepath.Join("wiki", name))
	}
	return sources
}
