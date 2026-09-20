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
const Usage = "Usage: md2pdf [-o path] [--separate|-s] [--mermaid path] <file-or-dir>"

// Parse turns arguments into a complete output plan.
func Parse(args []string, fs FileSystem) (Plan, error) {
	separate, output, mermaid, positions, err := parseArgs(args)
	if err != nil {
		return Plan{}, err
	}
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
	if !isDir {
		return planFile(input, output, mermaid, separate, fs)
	}
	return planDir(input, output, mermaid, separate, fs)
}

// parseArgs separates flags from positional arguments. ErrHelp and ErrVersion
// are returned as errors; the caller recognizes them with errors.Is.
func parseArgs(args []string) (bool, string, string, []string, error) {
	var separate bool
	var output, mermaid string
	var positions []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "-h", "--help":
			return false, "", "", nil, ErrHelp
		case "--version":
			return false, "", "", nil, ErrVersion
		case "--separate", "-s":
			separate = true
		case "-o":
			if i+1 == len(args) {
				return false, "", "", nil, errors.New("-o expects a path")
			}
			i++
			output = args[i]
		case "--mermaid":
			if i+1 == len(args) {
				return false, "", "", nil, errors.New("--mermaid expects a path")
			}
			i++
			mermaid = args[i]
		case "--":
			positions = append(positions, args[i+1:]...)
			i = len(args)
		default:
			if strings.HasPrefix(arg, "-") {
				return false, "", "", nil, fmt.Errorf("unknown flag: %s", arg)
			}
			positions = append(positions, arg)
		}
	}
	return separate, output, mermaid, positions, nil
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
