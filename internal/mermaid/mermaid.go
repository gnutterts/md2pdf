// Package mermaid rendert Mermaid-diagrammen met een extern programma.
package mermaid

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const standaardTimeout = 30 * time.Second

// Renderer zet diagramtekst om in een PNG met een externe renderer.
type Renderer struct {
	Pad     string        // pad naar of naam van de renderer; leeg betekent uitgeschakeld
	Timeout time.Duration // 0 betekent de standaard van 30 seconden
}

// Beschikbaar meldt of er een renderer is aangewezen.
func (r Renderer) Beschikbaar() bool {
	return r.Pad != ""
}

// Kies bepaalt de renderer uit vlag en omgeving, in die volgorde.
func Kies(vlag, omgeving string) Renderer {
	pad := vlag
	if pad == "" {
		pad = omgeving
	}
	if pad == "" {
		pad = "mmdc"
	}
	if pad == "uit" || pad == "none" {
		pad = ""
	}
	return Renderer{Pad: pad}
}

// NaarPNG rendert de diagramtekst en geeft de PNG-bytes terug.
func (r Renderer) NaarPNG(diagram string) ([]byte, error) {
	if !r.Beschikbaar() {
		return nil, errors.New("mermaid-renderer is uitgeschakeld")
	}

	pad, err := exec.LookPath(r.Pad)
	if err != nil {
		if info, statErr := os.Stat(r.Pad); statErr == nil && (info.IsDir() || info.Mode()&0o111 == 0) {
			return nil, errors.New("mermaid-renderer is niet uitvoerbaar")
		}
		return nil, fmt.Errorf("mermaid-renderer bestaat niet: %w", err)
	}
	info, err := os.Stat(pad)
	if err != nil {
		return nil, fmt.Errorf("kan mermaid-renderer niet controleren: %w", err)
	}
	if info.IsDir() || info.Mode()&0o111 == 0 {
		return nil, errors.New("mermaid-renderer is niet uitvoerbaar")
	}

	tijd := r.Timeout
	if tijd == 0 {
		tijd = standaardTimeout
	}
	mapje, err := os.MkdirTemp("", "md2pdf-mermaid-")
	if err != nil {
		return nil, fmt.Errorf("kan tijdelijke map niet maken: %w", err)
	}
	defer os.RemoveAll(mapje)

	invoer := filepath.Join(mapje, "diagram.mmd")
	uitvoer := filepath.Join(mapje, "diagram.png")
	if err := os.WriteFile(invoer, []byte(diagram), 0o600); err != nil {
		return nil, fmt.Errorf("kan tijdelijk diagram niet schrijven: %w", err)
	}

	ctx, annuleer := context.WithTimeout(context.Background(), tijd)
	defer annuleer()
	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, pad, "-i", invoer, "-o", uitvoer, "-b", "white", "-q")
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("mermaid-renderer timeout: %w", ctx.Err())
		}
		return nil, fmt.Errorf("mermaid-renderer faalde: %s", laatsteRegel(stderr.String()))
	}
	if ctx.Err() != nil {
		return nil, fmt.Errorf("mermaid-renderer timeout: %w", ctx.Err())
	}

	png, err := os.ReadFile(uitvoer)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errors.New("uitvoerbestand ontbreekt")
		}
		return nil, fmt.Errorf("kan uitvoerbestand niet lezen: %w", err)
	}
	if len(png) == 0 {
		return nil, errors.New("uitvoerbestand is leeg")
	}
	return png, nil
}

func laatsteRegel(tekst string) string {
	tekst = strings.TrimRight(tekst, "\r\n")
	if i := strings.LastIndexByte(tekst, '\n'); i >= 0 {
		tekst = tekst[i+1:]
	}
	tekens := []rune(strings.TrimSuffix(tekst, "\r"))
	if len(tekens) > 200 {
		return string(tekens[:200])
	}
	return string(tekens)
}
