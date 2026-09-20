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

var minimalePNG = []byte{'\x89', 'P', 'N', 'G', '\r', '\n', '\x1a', '\n', 'v', 'u', 'l'}

func schrijfScript(t *testing.T, inhoud string) string {
	t.Helper()
	pad := filepath.Join(t.TempDir(), "renderer.sh")
	if err := os.WriteFile(pad, []byte("#!/bin/sh\n"+inhoud), 0o700); err != nil {
		t.Fatal(err)
	}
	return pad
}

func paden() string {
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

func TestNaarPNGGeslaagd(t *testing.T) {
	script := schrijfScript(t, paden()+`printf '\211PNG\r\n\032\nvul' > "$out"`)
	png, err := (Renderer{Pad: script}).NaarPNG("graph TD\n A-->B")
	if err != nil {
		t.Fatal(err)
	}
	if string(png) != string(minimalePNG) {
		t.Fatalf("png = %q, wil %q", png, minimalePNG)
	}
}

func TestNaarPNGGeeftJuisteArgumentenDoor(t *testing.T) {
	gegevens := filepath.Join(t.TempDir(), "gegevens")
	script := schrijfScript(t, paden()+`{
printf '%s\n' $argumenten
printf '%s\n' '--inhoud--'
cat "$in"
} > `+shellTekst(gegevens)+`
printf x > "$out"`)
	diagram := "graph TD\n  A-->B"
	if _, err := (Renderer{Pad: script}).NaarPNG(diagram); err != nil {
		t.Fatal(err)
	}
	inhoud, err := os.ReadFile(gegevens)
	if err != nil {
		t.Fatal(err)
	}
	tekst := string(inhoud)
	for _, argument := range []string{"-i\n", "-o\n", "-b\n", "white\n", "-q\n", "--inhoud--\n" + diagram} {
		if !strings.Contains(tekst, argument) {
			t.Errorf("%q ontbreekt in %q", argument, tekst)
		}
	}
}

func TestNaarPNGExitcodeMelding(t *testing.T) {
	script := schrijfScript(t, `echo eerste >&2
echo laatste fout >&2
exit 7`)
	_, err := (Renderer{Pad: script}).NaarPNG("")
	if err == nil || !strings.Contains(err.Error(), "laatste fout") {
		t.Fatalf("fout = %v", err)
	}
}

func TestNaarPNGOntbrekendeUitvoer(t *testing.T) {
	script := schrijfScript(t, "exit 0")
	_, err := (Renderer{Pad: script}).NaarPNG("")
	if err == nil || !strings.Contains(err.Error(), "ontbreekt") {
		t.Fatalf("fout = %v", err)
	}
}

func TestNaarPNGLegeUitvoer(t *testing.T) {
	script := schrijfScript(t, paden()+`: > "$out"`)
	_, err := (Renderer{Pad: script}).NaarPNG("")
	if err == nil || !strings.Contains(err.Error(), "leeg") {
		t.Fatalf("fout = %v", err)
	}
}

func TestNaarPNGRendererBestaatNiet(t *testing.T) {
	_, err := (Renderer{Pad: filepath.Join(t.TempDir(), "nergens")}).NaarPNG("")
	if err == nil || !strings.Contains(err.Error(), "bestaat niet") {
		t.Fatalf("fout = %v", err)
	}
}

func TestNaarPNGRendererNietUitvoerbaar(t *testing.T) {
	pad := filepath.Join(t.TempDir(), "geen-uitvoer")
	if err := os.WriteFile(pad, []byte("#!/bin/sh\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := (Renderer{Pad: pad}).NaarPNG("")
	if err == nil || !strings.Contains(err.Error(), "niet uitvoerbaar") {
		t.Fatalf("fout = %v", err)
	}
}

func TestNaarPNGTimeout(t *testing.T) {
	script := schrijfScript(t, "exec sleep 5")
	begin := time.Now()
	_, err := (Renderer{Pad: script, Timeout: 100 * time.Millisecond}).NaarPNG("")
	if err == nil || !strings.Contains(err.Error(), "timeout") || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("fout = %v", err)
	}
	if tijd := time.Since(begin); tijd >= time.Second {
		t.Fatalf("timeout duurde %v", tijd)
	}
}

func TestNaarPNGRuimtOp(t *testing.T) {
	for _, proef := range []struct {
		naam string
		body string
	}{
		{"geslaagd", `printf x > "$out"`},
		{"mislukt", "echo stuk >&2\nexit 1"},
	} {
		t.Run(proef.naam, func(t *testing.T) {
			registratie := filepath.Join(t.TempDir(), "map")
			script := schrijfScript(t, paden()+`dirname "$out" > `+shellTekst(registratie)+"\n"+proef.body)
			_, _ = (Renderer{Pad: script}).NaarPNG("")
			mapje, err := os.ReadFile(registratie)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(strings.TrimSpace(string(mapje))); !os.IsNotExist(err) {
				t.Fatalf("tijdelijke map bestaat nog: %v", err)
			}
		})
	}
}

func TestKies(t *testing.T) {
	for _, proef := range []struct {
		naam, vlag, omgeving, wil string
	}{
		{"vlag wint", "vlag", "omgeving", "vlag"},
		{"omgeving wint", "", "omgeving", "omgeving"},
		{"standaard", "", "", "mmdc"},
		{"uit", "uit", "omgeving", ""},
		{"none", "none", "omgeving", ""},
	} {
		t.Run(proef.naam, func(t *testing.T) {
			if kreeg := Kies(proef.vlag, proef.omgeving).Pad; kreeg != proef.wil {
				t.Fatalf("Pad = %q, wil %q", kreeg, proef.wil)
			}
		})
	}
}

func TestUitgeschakeld(t *testing.T) {
	if (Renderer{}).Beschikbaar() {
		t.Fatal("lege renderer is beschikbaar")
	}
	_, err := (Renderer{}).NaarPNG("")
	if err == nil || !strings.Contains(err.Error(), "uitgeschakeld") {
		t.Fatalf("fout = %v", err)
	}
}

func shellTekst(tekst string) string {
	return "'" + strings.ReplaceAll(tekst, "'", "'\\\"'\\\"'") + "'"
}
