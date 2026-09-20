package mermaid

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var minimalPNG = []byte{'\x89', 'P', 'N', 'G', '\r', '\n', '\x1a', '\n', 'v', 'u', 'l'}

func writeScript(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "renderer.sh")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+content), 0o700); err != nil {
		t.Fatal(err)
	}
	return path
}

func paths() string {
	return `
argumenten="$@"
in=""
out=""
while [ "$#" -gt 0 ]; do
	case "$1" in
	-i) in="$2"; shift 2 ;;
	-o) out="$2"; shift 2 ;;
	*) shift ;;
	esac
done
`
}

func TestToPNGSuccessful(t *testing.T) {
	script := writeScript(t, paths()+`printf '\211PNG\r\n\032\nvul' > "$out"`)
	png, err := (Renderer{Path: script}).ToPNG("graph TD\n A-->B")
	if err != nil {
		t.Fatal(err)
	}
	if string(png) != string(minimalPNG) {
		t.Fatalf("png = %q, wil %q", png, minimalPNG)
	}
}

func TestToPNGPassesCorrectArguments(t *testing.T) {
	data := filepath.Join(t.TempDir(), "gegevens")
	script := writeScript(t, paths()+`{
printf '%s\n' $argumenten
printf '%s\n' '--inhoud--'
cat "$in"
} > `+shellText(data)+`
printf x > "$out"`)
	diagram := "graph TD\n  A-->B"
	if _, err := (Renderer{Path: script}).ToPNG(diagram); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(data)
	if err != nil {
		t.Fatal(err)
	}
	text := string(content)
	for _, argument := range []string{"-i\n", "-o\n", "-b\n", "white\n", "-q\n", "--inhoud--\n" + diagram} {
		if !strings.Contains(text, argument) {
			t.Errorf("%q ontbreekt in %q", argument, text)
		}
	}
}

func TestToPNGExitCodeMessage(t *testing.T) {
	script := writeScript(t, `echo eerste >&2
echo laatste fout >&2
exit 7`)
	_, err := (Renderer{Path: script}).ToPNG("")
	if err == nil || !strings.Contains(err.Error(), "laatste fout") {
		t.Fatalf("fout = %v", err)
	}
}

func TestToPNGMissingOutput(t *testing.T) {
	script := writeScript(t, "exit 0")
	_, err := (Renderer{Path: script}).ToPNG("")
	if err == nil || !strings.Contains(err.Error(), "ontbreekt") {
		t.Fatalf("fout = %v", err)
	}
}

func TestToPNGEmptyOutput(t *testing.T) {
	script := writeScript(t, paths()+`: > "$out"`)
	_, err := (Renderer{Path: script}).ToPNG("")
	if err == nil || !strings.Contains(err.Error(), "leeg") {
		t.Fatalf("fout = %v", err)
	}
}

func TestToPNGRendererDoesNotExist(t *testing.T) {
	_, err := (Renderer{Path: filepath.Join(t.TempDir(), "nergens")}).ToPNG("")
	if err == nil || !strings.Contains(err.Error(), "bestaat niet") {
		t.Fatalf("fout = %v", err)
	}
}

func TestToPNGRendererNotExecutable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "geen-uitvoer")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := (Renderer{Path: path}).ToPNG("")
	if err == nil || !strings.Contains(err.Error(), "niet uitvoerbaar") {
		t.Fatalf("fout = %v", err)
	}
}

func TestToPNGTimeout(t *testing.T) {
	script := writeScript(t, "exec sleep 5")
	start := time.Now()
	_, err := (Renderer{Path: script, Timeout: 100 * time.Millisecond}).ToPNG("")
	if err == nil || !strings.Contains(err.Error(), "timeout") || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("fout = %v", err)
	}
	if timeout := time.Since(start); timeout >= time.Second {
		t.Fatalf("timeout duurde %v", timeout)
	}
}

func TestToPNGRemovesFiles(t *testing.T) {
	for _, caseTest := range []struct {
		name string
		body string
	}{
		{"geslaagd", `printf x > "$out"`},
		{"mislukt", "echo stuk >&2\nexit 1"},
	} {
		t.Run(caseTest.name, func(t *testing.T) {
			record := filepath.Join(t.TempDir(), "map")
			script := writeScript(t, paths()+`dirname "$out" > `+shellText(record)+"\n"+caseTest.body)
			_, _ = (Renderer{Path: script}).ToPNG("")
			tempDir, err := os.ReadFile(record)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(strings.TrimSpace(string(tempDir))); !os.IsNotExist(err) {
				t.Fatalf("tijdelijke map bestaat nog: %v", err)
			}
		})
	}
}

func TestChoose(t *testing.T) {
	for _, caseTest := range []struct {
		name, flag, environment, want string
	}{
		{"vlag wint", "vlag", "omgeving", "vlag"},
		{"omgeving wint", "", "omgeving", "omgeving"},
		{"standaard", "", "", "mmdc"},
		{"uit", "uit", "omgeving", ""},
		{"none", "none", "omgeving", ""},
	} {
		t.Run(caseTest.name, func(t *testing.T) {
			if got := Choose(caseTest.flag, caseTest.environment).Path; got != caseTest.want {
				t.Fatalf("Pad = %q, wil %q", got, caseTest.want)
			}
		})
	}
}

func TestDisabled(t *testing.T) {
	if (Renderer{}).Available() {
		t.Fatal("lege renderer is beschikbaar")
	}
	_, err := (Renderer{}).ToPNG("")
	if err == nil || !strings.Contains(err.Error(), "uitgeschakeld") {
		t.Fatalf("fout = %v", err)
	}
}

func shellText(text string) string {
	return "'" + strings.ReplaceAll(text, "'", "'\\\"'\\\"'") + "'"
}
