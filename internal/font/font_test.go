// SPDX-License-Identifier: MIT

package font

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEveryStyleIsEmbedded(t *testing.T) {
	for _, family := range []string{Sans, Mono} {
		for _, style := range []string{"", "B", "I", "BI"} {
			data, err := Bytes(family, style)
			// A TrueType file starts with the version 1.0 tag 00 01 00 00.
			if err != nil || len(data) < 100_000 || !bytes.HasPrefix(data, []byte{0, 1, 0, 0}) {
				t.Errorf("%s %q: %d bytes, err = %v", family, style, len(data), err)
			}
		}
	}
	if _, err := Bytes("Serif", ""); err == nil {
		t.Error("an unknown family gave no error")
	}
	if _, err := Bytes(Sans, "U"); err == nil {
		t.Error("an unknown style gave no error")
	}
}

func fontDir(t *testing.T, files map[string][]byte) string {
	t.Helper()
	dir := t.TempDir()
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestFontDirectories(t *testing.T) {
	regular, _ := Bytes(Sans, "")
	bold, _ := Bytes(Sans, "B")
	full := fontDir(t, map[string][]byte{"Regular.ttf": regular, "Bold.ttf": bold})
	if err := CheckDir(full); err != nil {
		t.Fatal(err)
	}
	if data, err := FromDir(full, "B"); err != nil || !bytes.Equal(data, bold) {
		t.Fatalf("Bold: err = %v", err)
	}
	if data, err := FromDir(full, "I"); err != nil || !bytes.Equal(data, regular) {
		t.Fatalf("a missing Italic falls back to Regular: err = %v", err)
	}
	for name, dir := range map[string]string{
		"no Regular.ttf": fontDir(t, map[string][]byte{"Bold.ttf": bold}),
		"not TrueType":   fontDir(t, map[string][]byte{"Regular.ttf": []byte("hello")}),
		"OpenType CFF":   fontDir(t, map[string][]byte{"Regular.ttf": []byte("OTTO....")}),
		"bad Bold.ttf":   fontDir(t, map[string][]byte{"Regular.ttf": regular, "Bold.ttf": []byte("x")}),
		"missing":        filepath.Join(t.TempDir(), "nothing"),
	} {
		if err := CheckDir(dir); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
	if err := CheckDir(fontDir(t, map[string][]byte{"Regular.ttf": []byte("OTTO....")})); err == nil || !strings.Contains(err.Error(), ".otf") {
		t.Errorf("an OpenType font should explain the .otf problem: %v", err)
	}
}

func TestErrorsNameTheFileThatIsWrong(t *testing.T) {
	regular, _ := Bytes(Sans, "")
	dir := fontDir(t, map[string][]byte{"Regular.ttf": regular, "Italic.ttf": []byte("junk")})
	if err := CheckDir(dir); err == nil || !strings.Contains(err.Error(), "Italic.ttf") {
		t.Fatalf("err = %v, want it to name Italic.ttf", err)
	}
	broken := fontDir(t, map[string][]byte{"Regular.ttf": []byte("junk")})
	if _, err := FromDir(broken, "B"); err == nil || !strings.Contains(err.Error(), "Regular.ttf") {
		t.Fatalf("fallback err = %v, want it to name Regular.ttf", err)
	}
}
