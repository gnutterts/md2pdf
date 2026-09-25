// SPDX-License-Identifier: MIT

package pdfout

import (
	"bytes"
	"hash/crc32"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/gnutterts/md2pdf/internal/markdown"
	"github.com/gnutterts/md2pdf/internal/render"
)

func picture(width, height int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for x := 0; x < width; x++ {
		img.Set(x, x*height/width, color.RGBA{200, 0, 0, 255})
	}
	return img
}

func writeImage(t *testing.T, name string, encode func(*bytes.Buffer) error) string {
	t.Helper()
	var data bytes.Buffer
	if err := encode(&data); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, data.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// placement draws the image and returns the page content and its placed size.
func placement(t *testing.T, path string) (stream []byte, width, height float64, err error) {
	t.Helper()
	document := New()
	canvas := document.Canvas()
	canvas.Style("Helvetica", false, false, 11)
	canvas.Text("above")
	canvas.LineBreak(15)
	if err = canvas.Image(path); err != nil {
		return nil, 0, 0, err
	}
	canvas.Text("below")
	target := filepath.Join(t.TempDir(), "image.pdf")
	if werr := document.Write(target); werr != nil {
		t.Fatal(werr)
	}
	content, _ := os.ReadFile(target)
	stream = contentStream(t, content)
	match := regexp.MustCompile(`([\d.]+) 0 0 ([\d.]+) [\d.]+ [\d.]+ cm`).FindSubmatch(stream)
	if match == nil {
		t.Fatalf("no image placed:\n%s", stream)
	}
	width, _ = strconv.ParseFloat(string(match[1]), 64)
	height, _ = strconv.ParseFloat(string(match[2]), 64)
	return stream, width, height, nil
}

func TestImageFormatsAreDrawn(t *testing.T) {
	for name, encode := range map[string]func(*bytes.Buffer) error{
		"a.png": func(b *bytes.Buffer) error { return png.Encode(b, picture(120, 60)) },
		"a.jpg": func(b *bytes.Buffer) error { return jpeg.Encode(b, picture(120, 60), nil) },
		"a.gif": func(b *bytes.Buffer) error { return gif.Encode(b, picture(120, 60), nil) },
	} {
		t.Run(name, func(t *testing.T) {
			_, width, height, err := placement(t, writeImage(t, name, encode))
			if err != nil {
				t.Fatal(err)
			}
			// 120 x 60 pixels at 96 per inch is 90 x 45 points.
			if width != 90 || height != 45 {
				t.Fatalf("placed at %vx%v, want 90x45", width, height)
			}
		})
	}
}

func TestAWideImageIsScaledToTheTextWidth(t *testing.T) {
	path := writeImage(t, "wide.png", func(b *bytes.Buffer) error { return png.Encode(b, picture(2000, 500)) })
	_, width, height, err := placement(t, path)
	if err != nil {
		t.Fatal(err)
	}
	text := 595.28 - 2*DefaultMargin
	if width < text-0.01 || width > text+0.01 || height < text/4-0.01 || height > text/4+0.01 {
		t.Fatalf("placed at %vx%v, want %.2fx%.2f", width, height, text, text/4)
	}
}

func TestAnImageSitsBetweenTheTextAroundIt(t *testing.T) {
	path := writeImage(t, "p.png", func(b *bytes.Buffer) error { return png.Encode(b, picture(200, 100)) })
	stream, _, height, err := placement(t, path)
	if err != nil {
		t.Fatal(err)
	}
	image := regexp.MustCompile(`[\d.]+ 0 0 [\d.]+ [\d.]+ ([\d.]+) cm`).FindSubmatch(stream)
	y := func(word string) float64 {
		m := regexp.MustCompile(`BT [\d.]+ ([\d.]+) Td \(` + word + `\)`).FindSubmatch(stream)
		if m == nil {
			t.Fatalf("no %q in the stream", word)
		}
		value, _ := strconv.ParseFloat(string(m[1]), 64)
		return value
	}
	bottom, _ := strconv.ParseFloat(string(image[1]), 64) // PDF y of the image's lower edge
	top := bottom + height
	if !(y("above") > top && top > bottom && bottom > y("below")) {
		t.Fatalf("above at %v, image %v..%v, below at %v", y("above"), bottom, top, y("below"))
	}
}

func TestAnInterlacedPNGIsConvertedInsteadOfBreakingTheDocument(t *testing.T) {
	// A minimal interlaced PNG: png.Encode never interlaces, so flip the flag
	// in a real file and fix its checksum.
	var data bytes.Buffer
	if err := png.Encode(&data, picture(8, 8)); err != nil {
		t.Fatal(err)
	}
	raw := data.Bytes()
	// IHDR data starts at 16; the interlace byte is the 13th byte of the data.
	raw[16+12] = 1
	fixCRC(raw[12 : 12+4+13+4])
	path := filepath.Join(t.TempDir(), "interlaced.png")
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	document := New()
	canvas := document.Canvas()
	// The decoder of the standard library may reject this hand-made file; what
	// matters is that the document stays usable either way.
	_ = canvas.Image(path)
	canvas.Style("Helvetica", false, false, 11)
	canvas.Text("still fine")
	if err := document.Write(filepath.Join(t.TempDir(), "out.pdf")); err != nil {
		t.Fatalf("the document broke: %v", err)
	}
}

func TestImagesThatCannotBeDrawnReturnAnError(t *testing.T) {
	dir := t.TempDir()
	text := filepath.Join(dir, "notes.txt")
	os.WriteFile(text, []byte("not an image"), 0o600)
	for path, want := range map[string]string{
		filepath.Join(dir, "missing.png"): "not found",
		text:                              "unsupported format",
		dir:                               "is a directory",
	} {
		document := New()
		err := document.Canvas().Image(path)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: err = %v, want %q", filepath.Base(path), err, want)
		}
		if document.pdf.Error() != nil {
			t.Errorf("%s: fpdf is left in an error state", filepath.Base(path))
		}
	}
}

func TestAMissingImageShowsItsAltText(t *testing.T) {
	document := New()
	blocks := []markdown.Block{{Kind: markdown.Image, Path: "missing.png", Spans: []markdown.Span{{Text: "The alt"}}}}
	var warnings []string
	if err := render.Draw(blocks, document.Canvas(), render.Options{Warn: func(m string) { warnings = append(warnings, m) }}); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "alt.pdf")
	if err := document.Write(target); err != nil {
		t.Fatal(err)
	}
	content, _ := os.ReadFile(target)
	if !bytes.Contains(contentStream(t, content), []byte("(The alt)")) {
		t.Fatal("alt text not drawn")
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "not found") {
		t.Fatalf("warnings = %q", warnings)
	}
}

// fixCRC recomputes the CRC of a PNG chunk given as length, type, data and CRC.
func fixCRC(chunk []byte) {
	n := len(chunk)
	sum := crc32.ChecksumIEEE(chunk[4 : n-4])
	chunk[n-4], chunk[n-3], chunk[n-2], chunk[n-1] = byte(sum>>24), byte(sum>>16), byte(sum>>8), byte(sum)
}

func TestASixteenBitPNGIsDrawn(t *testing.T) {
	path := writeImage(t, "deep.png", func(b *bytes.Buffer) error {
		img := image.NewRGBA64(image.Rect(0, 0, 40, 20))
		img.Set(3, 3, color.RGBA64{0xffff, 0, 0, 0xffff})
		return png.Encode(b, img)
	})
	if _, width, _, err := placement(t, path); err != nil || width != 30 {
		t.Fatalf("err = %v, width = %v, want 30", err, width)
	}
}
