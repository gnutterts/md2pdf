// SPDX-License-Identifier: MIT

package cli

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

type fakeFileSystem struct {
	paths  map[string]bool
	dirs   map[string][]string
	errors map[string]error
}

func (n fakeFileSystem) Stat(path string) (bool, error) {
	if err, ok := n.errors[path]; ok {
		return false, err
	}
	isDir, ok := n.paths[path]
	if !ok {
		return false, os.ErrNotExist
	}
	return isDir, nil
}

func (n fakeFileSystem) ReadDir(path string) ([]string, error) {
	if err, ok := n.errors[path]; ok {
		return nil, err
	}
	content, ok := n.dirs[path]
	if !ok {
		return nil, os.ErrNotExist
	}
	return content, nil
}

func newFakeFS() fakeFileSystem {
	return fakeFileSystem{
		paths: map[string]bool{"book.md": false, "note.txt": false, "_Footer.md": false, "wiki": true, "wiki/": true, "empty": true, "navigation": true, "targetdir": true, "target.pdf": false},
		dirs: map[string][]string{
			"wiki":       {"z.md", "_Sidebar.md", "B.md", "a.MD", "_Footer.md", "10.md", "2.md", "A.md", "b.md", "index.md", "end.md", "chapter.md", "last.md", "read.txt", "x.markdown"},
			"wiki/":      {"z.md", "_Sidebar.md", "B.md", "a.MD", "_Footer.md", "10.md", "2.md", "A.md", "b.md", "index.md", "end.md", "chapter.md", "last.md", "read.txt", "x.markdown"},
			"empty":      {"text.txt"},
			"navigation": {"_Footer.md", "_Sidebar.MD"},
		},
		errors: map[string]error{},
	}
}

func TestParseModesAndTargets(t *testing.T) {
	fs := newFakeFS()
	tests := []struct {
		name string
		args []string
		plan Plan
	}{
		{"single markdown", []string{"book.md"}, Plan{Mode: ModeSingle, Tasks: []Task{{Sources: []string{"book.md"}, Target: "book.pdf"}}}},
		{"single without markdown extension", []string{"note.txt"}, Plan{Mode: ModeSingle, Tasks: []Task{{Sources: []string{"note.txt"}, Target: "note.txt.pdf"}}}},
		{"single with output after path", []string{"book.md", "-o", "custom.pdf"}, Plan{Mode: ModeSingle, Tasks: []Task{{Sources: []string{"book.md"}, Target: "custom.pdf"}}}},
		{"mermaid", []string{"--mermaid", "eigen-mmdc", "book.md"}, Plan{Mode: ModeSingle, Tasks: []Task{{Sources: []string{"book.md"}, Target: "book.pdf"}}, Mermaid: "eigen-mmdc"}},
		{"mermaid after input", []string{"book.md", "--mermaid", "eigen-mmdc"}, Plan{Mode: ModeSingle, Tasks: []Task{{Sources: []string{"book.md"}, Target: "book.pdf"}}, Mermaid: "eigen-mmdc"}},
		{"mermaid for directory", []string{"--mermaid", "off", "wiki"}, Plan{Mode: ModeMerged, Tasks: []Task{{Sources: wikiSources("wiki"), Target: "wiki.pdf"}}, Mermaid: "off"}},
		{"last mermaid wins", []string{"--mermaid", "first", "--mermaid", "second", "book.md"}, Plan{Mode: ModeSingle, Tasks: []Task{{Sources: []string{"book.md"}, Target: "book.pdf"}}, Mermaid: "second"}},
		{"mermaid with separate", []string{"--separate", "--mermaid", "path", "wiki"}, Plan{Mode: ModeSeparate, Tasks: separateTasks("wiki", ""), Mermaid: "path"}},
		{"input after double dash", []string{"--", "book.md"}, Plan{Mode: ModeSingle, Tasks: []Task{{Sources: []string{"book.md"}, Target: "book.pdf"}}}},
		{"mermaid before output", []string{"--mermaid", "path", "-o", "custom.pdf", "book.md"}, Plan{Mode: ModeSingle, Tasks: []Task{{Sources: []string{"book.md"}, Target: "custom.pdf"}}, Mermaid: "path"}},

		{"merged", []string{"wiki"}, Plan{Mode: ModeMerged, Tasks: []Task{{Sources: wikiSources("wiki"), Target: "wiki.pdf"}}}},
		{"merged with trailing slash", []string{"wiki/"}, Plan{Mode: ModeMerged, Tasks: []Task{{Sources: wikiSources("wiki/"), Target: "wiki.pdf"}}}},
		{"merged with output", []string{"-o", "book.pdf", "wiki"}, Plan{Mode: ModeMerged, Tasks: []Task{{Sources: wikiSources("wiki"), Target: "book.pdf"}}}},
		{"separate", []string{"wiki", "--separate"}, Plan{Mode: ModeSeparate, Tasks: separateTasks("wiki", "")}},
		{"separate short with directory target", []string{"-s", "wiki", "-o", "off"}, Plan{Mode: ModeSeparate, Tasks: separateTasks("wiki", "off")}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			plan, err := Parse(test.args, fs)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(plan, test.plan) {
				t.Errorf("plan = %#v, want %#v", plan, test.plan)
			}
		})
	}
}

func TestParseErrors(t *testing.T) {
	fs := newFakeFS()
	tests := []struct {
		name string
		args []string
		text string
	}{
		{"no arguments", nil, "no input given\n" + Usage},
		{"multiple arguments", []string{"book.md", "note.txt"}, "exactly one input path is required"},
		{"unknown flag", []string{"--other", "book.md"}, "unknown flag: --other"},
		{"missing output", []string{"book.md", "-o"}, "-o expects a path"},
		{"missing mermaid", []string{"book.md", "--mermaid"}, "--mermaid expects a path"},
		{"mermaid is not abbreviated", []string{"book.md", "--mer"}, "unknown flag: --mer"},
		{"mermaid without input", []string{"--mermaid", "path"}, "no input given"},
		{"mermaid with unknown flag", []string{"--mermaid", "path", "--other", "book.md"}, "unknown flag: --other"},
		{"does not exist", []string{"gone.md"}, "cannot read \"gone.md\""},
		{"separate with file", []string{"--separate", "book.md"}, "--separate only works on a directory"},
		{"empty directory", []string{"empty"}, "directory \"empty\" contains no Markdown files"},
		{"file target is directory", []string{"book.md", "-o", "targetdir"}, "-o points to a directory, but a file name is required here"},
		{"separate target is file", []string{"wiki", "--separate", "-o", "target.pdf"}, "-o points to a file, but --separate requires a directory"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := Parse(test.args, fs)
			if err == nil || !strings.Contains(err.Error(), test.text) {
				t.Fatalf("error = %v, want %q", err, test.text)
			}
		})
	}
}

func TestParseUnderscoredNames(t *testing.T) {
	fs := newFakeFS()
	tests := []struct {
		name string
		args []string
		plan Plan
		text string
	}{
		{"merged skips navigation", []string{"wiki"}, Plan{Mode: ModeMerged, Tasks: []Task{{Sources: wikiSources("wiki"), Target: "wiki.pdf"}}}, ""},
		{"separate skips navigation", []string{"--separate", "wiki"}, Plan{Mode: ModeSeparate, Tasks: separateTasks("wiki", "")}, ""},
		{"only navigation returns an error", []string{"navigation"}, Plan{}, "directory \"navigation\" contains no Markdown files (names starting with _ are skipped)"},
		{"explicit navigation file", []string{"_Footer.md"}, Plan{Mode: ModeSingle, Tasks: []Task{{Sources: []string{"_Footer.md"}, Target: "_Footer.pdf"}}}, ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			plan, err := Parse(test.args, fs)
			if test.text != "" {
				if err == nil || err.Error() != test.text {
					t.Fatalf("error = %v, want %q", err, test.text)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(plan, test.plan) {
				t.Errorf("plan = %#v, want %#v", plan, test.plan)
			}
			if test.plan.Mode == ModeMerged && len(plan.Tasks[0].Sources) != 11 {
				t.Errorf("source count = %d, want 11", len(plan.Tasks[0].Sources))
			}
			if test.plan.Mode == ModeSeparate && len(plan.Tasks) != 11 {
				t.Errorf("task count = %d, want 11", len(plan.Tasks))
			}
		})
	}
}

func TestParseHelpAndVersion(t *testing.T) {
	fs := newFakeFS()
	for _, test := range []struct {
		args []string
		want error
	}{
		{[]string{"-h"}, ErrHelp},
		{[]string{"--help"}, ErrHelp},
		{[]string{"--version"}, ErrVersion},
	} {
		_, err := Parse(test.args, fs)
		if !errors.Is(err, test.want) {
			t.Errorf("Plan(%q): %v, want %v", test.args, err, test.want)
		}
	}
}

func TestOSFileSystem(t *testing.T) {
	dirName := t.TempDir()
	if err := os.WriteFile(filepath.Join(dirName, "file"), []byte("content"), 0o600); err != nil {
		t.Fatal(err)
	}
	fs := OSFileSystem{}
	isDir, err := fs.Stat(dirName)
	if err != nil || !isDir {
		t.Fatalf("Stat(directory) = %v, %v", isDir, err)
	}
	isDir, err = fs.Stat(filepath.Join(dirName, "file"))
	if err != nil || isDir {
		t.Fatalf("Stat(file) = %v, %v", isDir, err)
	}
	names, err := fs.ReadDir(dirName)
	if err != nil || !reflect.DeepEqual(names, []string{"file"}) {
		t.Fatalf("ReadDir = %q, %v", names, err)
	}
}

func wikiSources(dirName string) []string {
	return []string{
		filepath.Join(dirName, "10.md"), filepath.Join(dirName, "2.md"), filepath.Join(dirName, "A.md"), filepath.Join(dirName, "B.md"), filepath.Join(dirName, "a.MD"), filepath.Join(dirName, "b.md"), filepath.Join(dirName, "chapter.md"), filepath.Join(dirName, "end.md"), filepath.Join(dirName, "index.md"), filepath.Join(dirName, "last.md"), filepath.Join(dirName, "z.md"),
	}
}

func separateTasks(dirName, targetDir string) []Task {
	sources := wikiSources(dirName)
	tasks := make([]Task, len(sources))
	for i, source := range sources {
		target := pdfName(source)
		if targetDir != "" {
			target = filepath.Join(targetDir, filepath.Base(target))
		}
		tasks[i] = Task{Sources: []string{source}, Target: target}
	}
	return tasks
}

func TestParseTitleAndAuthor(t *testing.T) {
	fs := fakeFileSystem{paths: map[string]bool{"notes.md": false}}
	plan, err := Parse([]string{"--title", "My Title", "--author", "Ann", "notes.md"}, fs)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Title != "My Title" || plan.Author != "Ann" {
		t.Fatalf("Title=%q Author=%q", plan.Title, plan.Author)
	}
	for _, flag := range []string{"--title", "--author"} {
		if _, err := Parse([]string{"notes.md", flag}, fs); err == nil || !strings.Contains(err.Error(), flag) {
			t.Fatalf("%s without a value: err = %v", flag, err)
		}
	}
}

func TestParsePageNumbers(t *testing.T) {
	fs := fakeFileSystem{paths: map[string]bool{"notes.md": false}}
	plan, err := Parse([]string{"notes.md"}, fs)
	if err != nil || plan.NoPageNumbers {
		t.Fatalf("default: NoPageNumbers = %v, err = %v", plan.NoPageNumbers, err)
	}
	plan, err = Parse([]string{"--no-page-numbers", "notes.md"}, fs)
	if err != nil || !plan.NoPageNumbers {
		t.Fatalf("--no-page-numbers: NoPageNumbers = %v, err = %v", plan.NoPageNumbers, err)
	}
}

func TestParseLength(t *testing.T) {
	for _, test := range []struct {
		in   string
		want float64
		bad  bool
	}{
		{"20mm", 56.692913, false}, {"1in", 72, false}, {"40", 40, false}, {"56pt", 56, false},
		{"0.75IN", 54, false}, {"abc", 0, true}, {"-5pt", 0, true}, {"0", 0, true}, {"mm", 0, true}, {"NaN", 0, true}, {"nanpt", 0, true}, {"Inf", 0, true}, {"1e999mm", 0, true},
	} {
		got, err := ParseLength(test.in)
		if test.bad {
			if err == nil {
				t.Errorf("ParseLength(%q) = %v, want an error", test.in, got)
			}
			continue
		}
		if err != nil || got < test.want-0.001 || got > test.want+0.001 {
			t.Errorf("ParseLength(%q) = %v, %v, want %v", test.in, got, err, test.want)
		}
	}
}

func TestParsePaperAndMargin(t *testing.T) {
	fs := fakeFileSystem{paths: map[string]bool{"notes.md": false}}
	plan, err := Parse([]string{"--paper", "LETTER", "--margin", "1in", "notes.md"}, fs)
	if err != nil || plan.Paper != "letter" || plan.Margin != 72 {
		t.Fatalf("Paper=%q Margin=%v err=%v", plan.Paper, plan.Margin, err)
	}
	if plan, err = Parse([]string{"--paper", " a4 ", "notes.md"}, fs); err != nil || plan.Paper != "a4" {
		t.Fatalf("padded paper name: Paper=%q err=%v", plan.Paper, err)
	}
	plan, err = Parse([]string{"notes.md"}, fs)
	if err != nil || plan.Paper != "" || plan.Margin != 0 {
		t.Fatalf("defaults: Paper=%q Margin=%v err=%v", plan.Paper, plan.Margin, err)
	}
	for _, test := range []struct {
		args    []string
		message string
	}{
		{[]string{"--paper", "b5", "notes.md"}, `unknown paper size "b5"`},
		{[]string{"--margin", "abc", "notes.md"}, "--margin"},
		{[]string{"--margin", "200pt", "notes.md"}, "too large"},
		{[]string{"--paper", "a5", "--margin", "110pt", "notes.md"}, "too large"},
		{[]string{"notes.md", "--paper"}, "--paper expects a value"},
	} {
		if _, err := Parse(test.args, fs); err == nil || !strings.Contains(err.Error(), test.message) {
			t.Errorf("Parse(%v) error = %v, want %q", test.args, err, test.message)
		}
	}
}

func TestParseStrict(t *testing.T) {
	fs := fakeFileSystem{paths: map[string]bool{"notes.md": false}}
	if plan, err := Parse([]string{"notes.md"}, fs); err != nil || plan.Strict {
		t.Fatalf("default: Strict=%v err=%v", plan.Strict, err)
	}
	if plan, err := Parse([]string{"--strict", "notes.md"}, fs); err != nil || !plan.Strict {
		t.Fatalf("--strict: Strict=%v err=%v", plan.Strict, err)
	}
}

func TestParseProtectsExistingOutput(t *testing.T) {
	fs := fakeFileSystem{
		paths: map[string]bool{"notes.md": false, "other.md": false, "keep.txt": false, "old.pdf": false, "docs": true},
		dirs:  map[string][]string{"docs": {"a.md", "b.md"}},
	}
	for _, test := range []struct {
		name    string
		args    []string
		message string // empty means no error
	}{
		{"overwriting a PDF is normal", []string{"-o", "old.pdf", "notes.md"}, ""},
		{"a new file is fine", []string{"-o", "new.txt", "notes.md"}, ""},
		{"existing non-PDF is refused", []string{"-o", "keep.txt", "notes.md"}, "exists and is not a PDF"},
		{"existing non-PDF with --force", []string{"--force", "-o", "keep.txt", "notes.md"}, ""},
		{"the input itself", []string{"-o", "notes.md", "notes.md"}, "is also an input"},
		{"the input itself, unnormalised", []string{"-o", "./notes.md", "notes.md"}, "is also an input"},
		{"the input itself with --force", []string{"--force", "-o", "notes.md", "notes.md"}, "is also an input"},
		{"a source of a merged directory", []string{"-o", "docs/a.md", "docs"}, "is also an input"},
		{"existing non-PDF for a merged directory", []string{"-o", "keep.txt", "docs"}, "exists and is not a PDF"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := Parse(test.args, fs)
			if test.message == "" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), test.message) {
				t.Fatalf("err = %v, want %q", err, test.message)
			}
		})
	}
}

func TestParseRefusesAnOutputThatIsTheSameFileOnDisk(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "notes.md")
	if err := os.WriteFile(source, []byte("# x\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link.txt")
	if err := os.Symlink(source, link); err != nil {
		t.Skipf("no symbolic links here: %v", err)
	}
	hard := filepath.Join(dir, "hard.txt")
	if err := os.Link(source, hard); err != nil {
		t.Fatal(err)
	}
	cases := map[string]string{"symbolic link": link, "hard link": hard, "relative spelling": filepath.Join(dir, ".", "sub", "..", "notes.md")}
	// On a case-insensitive file system another spelling is the same file too.
	if _, err := os.Stat(filepath.Join(dir, "NOTES.MD")); err == nil {
		cases["other case"] = filepath.Join(dir, "NOTES.MD")
	}
	for name, output := range cases {
		t.Run(name, func(t *testing.T) {
			for _, extra := range [][]string{nil, {"--force"}} {
				args := append(append(extra, "-o", output), source)
				if _, err := Parse(args, OSFileSystem{}); err == nil || !strings.Contains(err.Error(), "is also an input") {
					t.Fatalf("Parse(%v) error = %v, want \"is also an input\"", args, err)
				}
			}
		})
	}
}

func TestParseScale(t *testing.T) {
	fs := fakeFileSystem{paths: map[string]bool{"notes.md": false}}
	if plan, err := Parse([]string{"--scale", "3", "notes.md"}, fs); err != nil || plan.Scale != "3" {
		t.Fatalf("Scale=%q err=%v", plan.Scale, err)
	}
	if _, err := Parse([]string{"notes.md", "--scale"}, fs); err == nil || !strings.Contains(err.Error(), "--scale") {
		t.Fatalf("missing value: err = %v", err)
	}
}

func TestParseMermaidTimeout(t *testing.T) {
	fs := fakeFileSystem{paths: map[string]bool{"notes.md": false}}
	if plan, err := Parse([]string{"--mermaid-timeout", "45s", "notes.md"}, fs); err != nil || plan.MermaidTimeout != "45s" {
		t.Fatalf("MermaidTimeout=%q err=%v", plan.MermaidTimeout, err)
	}
	if _, err := Parse([]string{"notes.md", "--mermaid-timeout"}, fs); err == nil || !strings.Contains(err.Error(), "--mermaid-timeout") {
		t.Fatalf("missing value: err = %v", err)
	}
}
