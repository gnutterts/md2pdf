// Package mermaid renders Mermaid diagrams with an external program.
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

const defaultTimeout = 30 * time.Second

// Renderer converts diagram text to a PNG with an external renderer.
type Renderer struct {
	Path    string        // path to or name of the renderer; empty means disabled
	Timeout time.Duration // 0 means the default of 30 seconds
}

// Available reports whether a renderer is specified.
func (r Renderer) Available() bool {
	return r.Path != ""
}

// Choose determines the renderer from flag and environment, in that order.
func Choose(flag, environment string) Renderer {
	path := flag
	if path == "" {
		path = environment
	}
	if path == "" {
		path = "mmdc"
	}
	if path == "uit" || path == "none" {
		path = ""
	}
	return Renderer{Path: path}
}

// ToPNG renders the diagram text and returns the PNG bytes.
func (r Renderer) ToPNG(diagram string) ([]byte, error) {
	if !r.Available() {
		return nil, errors.New("mermaid-renderer is uitgeschakeld")
	}

	path, err := exec.LookPath(r.Path)
	if err != nil {
		if info, statErr := os.Stat(r.Path); statErr == nil && (info.IsDir() || info.Mode()&0o111 == 0) {
			return nil, errors.New("mermaid-renderer is niet uitvoerbaar")
		}
		return nil, fmt.Errorf("mermaid-renderer bestaat niet: %w", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("kan mermaid-renderer niet controleren: %w", err)
	}
	if info.IsDir() || info.Mode()&0o111 == 0 {
		return nil, errors.New("mermaid-renderer is niet uitvoerbaar")
	}

	timeout := r.Timeout
	if timeout == 0 {
		timeout = defaultTimeout
	}
	tempDir, err := os.MkdirTemp("", "md2pdf-mermaid-")
	if err != nil {
		return nil, fmt.Errorf("kan tijdelijke map niet maken: %w", err)
	}
	defer os.RemoveAll(tempDir)

	input := filepath.Join(tempDir, "diagram.mmd")
	output := filepath.Join(tempDir, "diagram.png")
	if err := os.WriteFile(input, []byte(diagram), 0o600); err != nil {
		return nil, fmt.Errorf("kan tijdelijk diagram niet schrijven: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, path, "-i", input, "-o", output, "-b", "white", "-q")
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("mermaid-renderer timeout: %w", ctx.Err())
		}
		return nil, fmt.Errorf("mermaid-renderer faalde: %s", lastLine(stderr.String()))
	}
	if ctx.Err() != nil {
		return nil, fmt.Errorf("mermaid-renderer timeout: %w", ctx.Err())
	}

	png, err := os.ReadFile(output)
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

func lastLine(text string) string {
	text = strings.TrimRight(text, "\r\n")
	if i := strings.LastIndexByte(text, '\n'); i >= 0 {
		text = text[i+1:]
	}
	runes := []rune(strings.TrimSuffix(text, "\r"))
	if len(runes) > 200 {
		return string(runes[:200])
	}
	return string(runes)
}
