// SPDX-License-Identifier: MIT

// Package font carries the TrueType fonts that md2pdf embeds: DejaVu Sans for
// text and DejaVu Sans Mono for code, each in four styles. Their licence is in
// LICENSE-DejaVu.txt next to this file and in THIRD-PARTY-NOTICES.md.
package font

import (
	"embed"
	"fmt"
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
