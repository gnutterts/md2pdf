// SPDX-License-Identifier: MIT

// Package cli creates an output plan for md2pdf.
package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Mode determines how the supplied input is processed.
type Mode int

const (
	// ModeSingle creates one PDF from one file.
	ModeSingle Mode = iota
	// ModeMerged creates one PDF from all Markdown files in a directory.
	ModeMerged
	// ModeSeparate creates one PDF per Markdown file in a directory.
	ModeSeparate
)

// Task describes sources and their target.
type Task struct {
	Sources []string
	Target  string
}

// Plan is the complete, ordered processing.
type Plan struct {
	Mode    Mode
	Tasks   []Task
	Mermaid string
	// Title and Author come from --title and --author; empty means unset.
	Title  string
	Author string
	// NoPageNumbers is set by --no-page-numbers; by default every page is numbered.
	NoPageNumbers bool
	// Creator names the program in the PDF; the entry point sets it.
	Creator string
}

// FileSystem contains the read operations needed for planning.
type FileSystem interface {
	Stat(path string) (isDir bool, err error)
	ReadDir(path string) ([]string, error)
}

// OSFileSystem uses the local file system.
type OSFileSystem struct{}

// Stat reports whether path is a directory.
func (OSFileSystem) Stat(path string) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		return false, err
	}
	return info.IsDir(), nil
}

// ReadDir reads the names directly in a directory.
func (OSFileSystem) ReadDir(path string) ([]string, error) {
	items, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	names := make([]string, len(items))
	for i, item := range items {
		names[i] = item.Name()
	}
	return names, nil
}

var (
	// ErrHelp asks the entry point to print the usage text.
	ErrHelp = errors.New("help requested")
	// ErrVersion asks the entry point to print the version.
	ErrVersion = errors.New("version requested")
)

// Usage is the short usage text for the command.
const Usage = "Usage: md2pdf [-o path] [--separate|-s] [--mermaid path] [--title text] [--author text] [--no-page-numbers] <file-or-dir>"

// Parse turns arguments into a complete output plan.
func Parse(args []string, fs FileSystem) (Plan, error) {
	f, err := parseArgs(args)
	if err != nil {
		return Plan{}, err
	}
	positions := f.positions
	if len(positions) == 0 {
		return Plan{}, fmt.Errorf("no input given\n%s", Usage)
	}
	if len(positions) != 1 {
		return Plan{}, errors.New("exactly one input path is required")
	}

	input := positions[0]
	isDir, err := fs.Stat(input)
	if err != nil {
		return Plan{}, fmt.Errorf("cannot read %q: %w", input, err)
	}
	var plan Plan
	if !isDir {
		plan, err = planFile(input, f.output, f.mermaid, f.separate, fs)
	} else {
		plan, err = planDir(input, f.output, f.mermaid, f.separate, fs)
	}
	if err != nil {
		return Plan{}, err
	}
	plan.Title, plan.Author = f.title, f.author
	plan.NoPageNumbers = f.noNumbers
	return plan, nil
}

// flags are the parsed command-line arguments.
type flags struct {
	separate  bool
	output    string
	mermaid   string
	title     string
	author    string
	noNumbers bool
	positions []string
}

// parseArgs separates flags from positional arguments. ErrHelp and ErrVersion
// are returned as errors; the caller recognizes them with errors.Is.
func parseArgs(args []string) (flags, error) {
	var f flags
	value := func(i *int, name string) (string, error) {
		if *i+1 == len(args) {
			return "", fmt.Errorf("%s expects a value", name)
		}
		*i++
		return args[*i], nil
	}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		var err error
		switch arg {
		case "-h", "--help":
			return flags{}, ErrHelp
		case "--version":
			return flags{}, ErrVersion
		case "--separate", "-s":
			f.separate = true
		case "-o":
			if i+1 == len(args) {
				return flags{}, errors.New("-o expects a path")
			}
			i++
			f.output = args[i]
		case "--mermaid":
			if i+1 == len(args) {
				return flags{}, errors.New("--mermaid expects a path")
			}
			i++
			f.mermaid = args[i]
		case "--no-page-numbers":
			f.noNumbers = true
		case "--title":
			f.title, err = value(&i, "--title")
		case "--author":
			f.author, err = value(&i, "--author")
		case "--":
			f.positions = append(f.positions, args[i+1:]...)
			i = len(args)
		default:
			if strings.HasPrefix(arg, "-") {
				return flags{}, fmt.Errorf("unknown flag: %s", arg)
			}
			f.positions = append(f.positions, arg)
		}
		if err != nil {
			return flags{}, err
		}
	}
	return f, nil
}

func planFile(input, output, mermaid string, separate bool, fs FileSystem) (Plan, error) {
	if separate {
		return Plan{}, errors.New("--separate only works on a directory")
	}
	if output == "" {
		output = pdfName(input)
	} else if err := fileTarget(output, fs); err != nil {
		return Plan{}, err
	}
	return Plan{Mode: ModeSingle, Tasks: []Task{{Sources: []string{input}, Target: output}}, Mermaid: mermaid}, nil
}

func planDir(input, output, mermaid string, separate bool, fs FileSystem) (Plan, error) {
	sources, err := markdownFiles(input, fs)
	if err != nil {
		return Plan{}, err
	}
	if separate {
		if output != "" {
			if err := dirTarget(output, fs); err != nil {
				return Plan{}, err
			}
		}
		tasks := make([]Task, len(sources))
		for i, source := range sources {
			target := pdfName(source)
			if output != "" {
				target = filepath.Join(output, filepath.Base(target))
			}
			tasks[i] = Task{Sources: []string{source}, Target: target}
		}
		return Plan{Mode: ModeSeparate, Tasks: tasks, Mermaid: mermaid}, nil
	}
	if output == "" {
		output = filepath.Clean(input) + ".pdf"
	} else if err := fileTarget(output, fs); err != nil {
		return Plan{}, err
	}
	return Plan{Mode: ModeMerged, Tasks: []Task{{Sources: sources, Target: output}}, Mermaid: mermaid}, nil
}

func markdownFiles(dirName string, fs FileSystem) ([]string, error) {
	names, err := fs.ReadDir(dirName)
	if err != nil {
		return nil, fmt.Errorf("cannot read directory %q: %w", dirName, err)
	}
	var markdown []string
	var hasMarkdown bool
	for _, name := range names {
		if strings.HasSuffix(strings.ToLower(name), ".md") {
			hasMarkdown = true
			if !strings.HasPrefix(name, "_") {
				markdown = append(markdown, name)
			}
		}
	}
	sort.Strings(markdown)
	if len(markdown) == 0 {
		if hasMarkdown {
			return nil, fmt.Errorf("directory %q contains no Markdown files (names starting with _ are skipped)", dirName)
		}
		return nil, fmt.Errorf("directory %q contains no Markdown files", dirName)
	}
	sources := make([]string, len(markdown))
	for i, name := range markdown {
		sources[i] = filepath.Join(dirName, name)
	}
	return sources, nil
}

func fileTarget(target string, fs FileSystem) error {
	isDir, err := fs.Stat(target)
	if err == nil && isDir {
		return errors.New("-o points to a directory, but a file name is required here")
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("cannot read output target %q: %w", target, err)
	}
	return nil
}

func dirTarget(target string, fs FileSystem) error {
	isDir, err := fs.Stat(target)
	if err == nil && !isDir {
		return errors.New("-o points to a file, but --separate requires a directory")
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("cannot read output target %q: %w", target, err)
	}
	return nil
}

func pdfName(source string) string {
	extension := filepath.Ext(source)
	if strings.EqualFold(extension, ".md") {
		return strings.TrimSuffix(source, extension) + ".pdf"
	}
	return source + ".pdf"
}
