// SPDX-License-Identifier: MIT

// Package font carries the TrueType fonts that md2pdf embeds: DejaVu Sans for
// text and DejaVu Sans Mono for code, each in four styles. Their licence is in
// LICENSE-DejaVu.txt next to this file and in THIRD-PARTY-NOTICES.md.
package font

import (
	"bytes"
	"embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

//go:embed *.ttf
var files embed.FS

// Family names as md2pdf uses them.
const (
	Sans = "DejaVuSans"
	Mono = "DejaVuSansMono"
)

// Bytes returns the TrueType data of a family (Sans or Mono) in a style:
// "", "B", "I" or "BI".
func Bytes(family, style string) ([]byte, error) {
	base := family
	if base != Sans && base != Mono {
		return nil, fmt.Errorf("unknown font family %q", family)
	}
	suffix, ok := map[string]string{"": "", "B": "-Bold", "I": "-Oblique", "BI": "-BoldOblique"}[style]
	if !ok {
		return nil, fmt.Errorf("unknown font style %q", style)
	}
	return files.ReadFile(base + suffix + ".ttf")
}

// Files a font directory for --font or --font-mono holds. Only Regular.ttf is
// required; a missing style falls back to it.
var styleFiles = map[string]string{"": "Regular.ttf", "B": "Bold.ttf", "I": "Italic.ttf", "BI": "BoldItalic.ttf"}

// FromDir returns the TrueType data of a style from a font directory, or of
// Regular.ttf when that style is missing.
func FromDir(dir, style string) ([]byte, error) {
	name, ok := styleFiles[style]
	if !ok {
		return nil, fmt.Errorf("unknown font style %q", style)
	}
	data, err := os.ReadFile(filepath.Join(dir, name))
	if errors.Is(err, os.ErrNotExist) && style != "" {
		data, err = os.ReadFile(filepath.Join(dir, styleFiles[""]))
	}
	if err != nil {
		return nil, err
	}
	if err := checkTrueType(data); err != nil {
		return nil, fmt.Errorf("%s: %w", filepath.Join(dir, name), err)
	}
	return data, nil
}

// CheckDir reports whether dir is a usable font directory: it must hold a
// TrueType Regular.ttf, and every other style file present must be TrueType too.
func CheckDir(dir string) error {
	info, err := os.Stat(dir)
	if err != nil {
		return fmt.Errorf("font directory %q: %w", dir, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("font directory %q is not a directory", dir)
	}
	for _, style := range []string{"", "B", "I", "BI"} {
		path := filepath.Join(dir, styleFiles[style])
		data, err := os.ReadFile(path)
		if errors.Is(err, os.ErrNotExist) && style != "" {
			continue
		}
		if err != nil {
			return fmt.Errorf("font directory %q needs Regular.ttf: %w", dir, err)
		}
		if err := checkTrueType(data); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
	}
	return nil
}

// checkTrueType accepts TrueType outlines, the only kind fpdf can embed.
func checkTrueType(data []byte) error {
	switch {
	case bytes.HasPrefix(data, []byte{0, 1, 0, 0}), bytes.HasPrefix(data, []byte("true")):
		return nil
	case bytes.HasPrefix(data, []byte("OTTO")):
		return errors.New("fonts with PostScript outlines (most .otf files) cannot be embedded; use a TrueType .ttf")
	default:
		return errors.New("not a TrueType font")
	}
}
