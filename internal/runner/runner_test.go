// SPDX-License-Identifier: MIT

package runner

import (
	"bytes"
	"image"
	"image/png"
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
		Sources: []string{readPath("sample.md")}, Target: target,
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
	sources := pageSources()
	target := filepath.Join(t.TempDir(), "pages.pdf")
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
	sources := pageSources()
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
	plan := cli.Plan{Mode: cli.ModeSeparate, Tasks: []cli.Task{{Sources: []string{readPath("sample.md")}, Target: target}}}
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
		Sources: []string{readPath("sample.md")}, Target: filepath.Join(dirName, "out.pdf"),
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

// pageSources lists the eleven pages that a folder read would yield: the
// two files whose name starts with an underscore are deliberately absent.
func pageSources() []string {
	names := []string{
		"Alpha.md", "Bravo.md", "Charlie.md", "Delta.md", "Echo.md",
		"Foxtrot.md", "Golf.md", "Hotel.md", "India.md", "Juliett.md", "Kilo.md",
	}
	sources := make([]string, len(names))
	for i, name := range names {
		sources[i] = readPath(filepath.Join("pages", name))
	}
	return sources
}

func pdfInfo(t *testing.T, path, key string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	marker := []byte("/" + key + " (\xfe\xff")
	at := bytes.LastIndex(content, marker)
	if at < 0 {
		return ""
	}
	raw := content[at+len(marker):]
	raw = raw[:bytes.IndexByte(raw, ')')]
	var out []rune
	for i := 0; i+1 < len(raw); i += 2 {
		out = append(out, rune(raw[i])<<8|rune(raw[i+1]))
	}
	return string(out)
}

func TestRunDocumentTitleAndAuthorPrecedence(t *testing.T) {
	dir := t.TempDir()
	write := func(name, content string) string {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	cases := []struct {
		name, content, flagTitle, flagAuthor string
		wantTitle, wantAuthor                string
	}{
		{"flag.md", "---\ntitle: Front\nauthor: Ann\n---\n# Head\n", "Flag", "Bob", "Flag", "Bob"},
		{"front.md", "---\ntitle: Front\nauthor: Ann\n---\n# Head\n", "", "", "Front", "Ann"},
		{"head.md", "# Head *one*\n\ntext\n", "", "", "Head one", ""},
		{"plain.md", "just text\n", "", "", "plain", ""},
	}
	for _, c := range cases {
		source := write(c.name, c.content)
		target := filepath.Join(dir, c.name+".pdf")
		plan := cli.Plan{
			Mode: cli.ModeSingle, Tasks: []cli.Task{{Sources: []string{source}, Target: target}},
			Title: c.flagTitle, Author: c.flagAuthor, Creator: "md2pdf test",
		}
		if err := Run(plan, render.Options{}); err != nil {
			t.Fatal(err)
		}
		if got := pdfInfo(t, target, "Title"); got != c.wantTitle {
			t.Errorf("%s: Title = %q, want %q", c.name, got, c.wantTitle)
		}
		if got := pdfInfo(t, target, "Author"); got != c.wantAuthor {
			t.Errorf("%s: Author = %q, want %q", c.name, got, c.wantAuthor)
		}
		if got := pdfInfo(t, target, "Creator"); got != "md2pdf test" {
			t.Errorf("%s: Creator = %q", c.name, got)
		}
	}
}

func TestRunMergedTitleFallsBackToDirectoryName(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "handbook")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(dir, "a.md")
	if err := os.WriteFile(source, []byte("just text\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "out.pdf")
	plan := cli.Plan{Mode: cli.ModeMerged, Tasks: []cli.Task{{Sources: []string{source}, Target: target}}}
	if err := Run(plan, render.Options{}); err != nil {
		t.Fatal(err)
	}
	if got := pdfInfo(t, target, "Title"); got != "handbook" {
		t.Fatalf("Title = %q", got)
	}
}

func TestRunStrictWritesNoPDFWhenADiagramFails(t *testing.T) {
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
	options := render.Options{Mermaid: mermaid.Renderer{Path: script}, Strict: true}
	err := Run(cli.Plan{Mode: cli.ModeSingle, Strict: true, Tasks: []cli.Task{{Sources: []string{source}, Target: target}}}, options)
	if err == nil || !strings.Contains(err.Error(), "(--strict)") {
		t.Fatalf("err = %v", err)
	}
	if _, statErr := os.Stat(target); statErr == nil {
		t.Fatal("a PDF was written although --strict failed the run")
	}
}

// separatePlan writes two readable sources and puts a missing one between them.
func separatePlan(t *testing.T, strict bool) (cli.Plan, []string) {
	t.Helper()
	dirName := t.TempDir()
	var tasks []cli.Task
	var targets []string
	for i, name := range []string{"a.md", "missing.md", "c.md"} {
		source := filepath.Join(dirName, name)
		if i != 1 {
			if err := os.WriteFile(source, []byte("# "+name+"\n"), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		target := filepath.Join(dirName, "out", strings.TrimSuffix(name, ".md")+".pdf")
		targets = append(targets, target)
		tasks = append(tasks, cli.Task{Sources: []string{source}, Target: target})
	}
	return cli.Plan{Mode: cli.ModeSeparate, Strict: strict, Tasks: tasks}, targets
}

func TestRunSeparateGoesOnAfterABrokenFile(t *testing.T) {
	plan, targets := separatePlan(t, false)
	var warnings []string
	err := Run(plan, render.Options{Warn: func(message string) { warnings = append(warnings, message) }})
	if err == nil || err.Error() != "1 of 3 files failed" {
		t.Fatalf("err = %v", err)
	}
	for _, target := range []string{targets[0], targets[2]} {
		if _, statErr := os.Stat(target); statErr != nil {
			t.Errorf("%s was not written: %v", target, statErr)
		}
	}
	if _, statErr := os.Stat(targets[1]); statErr == nil {
		t.Error("a PDF exists for the broken file")
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "missing.md") || !strings.HasPrefix(warnings[0], "skipped") {
		t.Fatalf("warnings = %q", warnings)
	}
}

func TestRunSeparateStrictStopsAtTheFirstFailure(t *testing.T) {
	plan, targets := separatePlan(t, true)
	if err := Run(plan, render.Options{}); err == nil || !strings.Contains(err.Error(), "missing.md") {
		t.Fatalf("err = %v", err)
	}
	if _, statErr := os.Stat(targets[2]); statErr == nil {
		t.Error("the file after the failure was written although --strict stops the batch")
	}
}

func TestRunSeparateCountsASingleFile(t *testing.T) {
	plan, _ := separatePlan(t, false)
	plan.Tasks = plan.Tasks[1:2]
	if err := Run(plan, render.Options{}); err == nil || err.Error() != "1 of 1 file failed" {
		t.Fatalf("err = %v", err)
	}
}

func TestMergedFilesShareTheDiagramCache(t *testing.T) {
	dir := t.TempDir()
	counter := filepath.Join(dir, "calls")
	script := filepath.Join(dir, "renderer")
	pngPath := filepath.Join(dir, "one.png")
	var pngBytes bytes.Buffer
	if err := png.Encode(&pngBytes, image.NewRGBA(image.Rect(0, 0, 20, 10))); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pngPath, pngBytes.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	body := "#!/bin/sh\necho x >> '" + counter + "'\ncp '" + pngPath + "' \"$4\"\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	diagram := "```mermaid\ngraph TD\nA-->B\n```\n"
	var sources []string
	for i, text := range []string{diagram + diagram, diagram, "```mermaid\ngraph TD\nC-->D\n```\n"} {
		source := filepath.Join(dir, string(rune('a'+i))+".md")
		if err := os.WriteFile(source, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
		sources = append(sources, source)
	}
	target := filepath.Join(dir, "out.pdf")
	options := render.Options{Mermaid: mermaid.Renderer{Path: script}}
	if err := Run(cli.Plan{Mode: cli.ModeMerged, Tasks: []cli.Task{{Sources: sources, Target: target}}}, options); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(counter)
	if err != nil {
		t.Fatal(err)
	}
	if calls := strings.Count(string(data), "x"); calls != 2 {
		t.Fatalf("the renderer ran %d times, want 2 (one per distinct diagram)", calls)
	}
}
