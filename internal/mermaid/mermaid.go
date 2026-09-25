// SPDX-License-Identifier: MIT

// Package mermaid renders Mermaid diagrams with an external program.
package mermaid

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const defaultTimeout = 30 * time.Second

// MaxPNGSize is the largest PNG that is accepted from the renderer, in bytes.
const MaxPNGSize = 64 << 20

// Renderer converts diagram text to a PNG with an external renderer.
type Renderer struct {
	Path    string        // path to or name of the renderer; empty means disabled
	Timeout time.Duration // 0 means the default of 30 seconds
	Scale   float64       // device scale factor passed as -s; 0 leaves the renderer's own default
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
	if path == "off" || path == "none" {
		path = ""
	}
	return Renderer{Path: path}
}

// ChooseTimeout determines the time limit per diagram from flag and environment,
// in that order; empty means 30 seconds. The value is a Go duration such as 45s.
func ChooseTimeout(flag, environment string) (time.Duration, error) {
	text := flag
	if text == "" {
		text = environment
	}
	if text == "" {
		return defaultTimeout, nil
	}
	timeout, err := time.ParseDuration(strings.TrimSpace(text))
	if err != nil || timeout <= 0 {
		return 0, fmt.Errorf("timeout %q must be a positive duration such as 45s or 2m (--mermaid-timeout or MD2PDF_MERMAID_TIMEOUT)", text)
	}
	return timeout, nil
}

// DefaultScale is the device scale factor used when none is chosen: at scale 1 a
// diagram is grainy on paper.
const DefaultScale = 2.0

// ChooseScale determines the scale factor from flag and environment, in that
// order; empty means DefaultScale. The value must be between 1 and 4.
func ChooseScale(flag, environment string) (float64, error) {
	text := flag
	if text == "" {
		text = environment
	}
	if text == "" {
		return DefaultScale, nil
	}
	scale, err := strconv.ParseFloat(strings.TrimSpace(text), 64)
	if err != nil || math.IsNaN(scale) || scale < 1 || scale > 4 {
		return 0, fmt.Errorf("scale %q must be a number between 1 and 4 (--scale or MD2PDF_SCALE)", text)
	}
	return scale, nil
}

// notExecutable reports whether a file lacks the executable permission bit.
// Windows has no such bit: there, a regular file would always look
// non-executable, so exec.LookPath is the only meaningful check and it has
// already consulted PATHEXT by the time this is called.
func notExecutable(info os.FileInfo) bool {
	if runtime.GOOS == "windows" {
		return false
	}
	return info.Mode()&0o111 == 0
}

// ToPNG renders the diagram text and returns the PNG bytes.
func (r Renderer) ToPNG(diagram string) ([]byte, error) {
	if !r.Available() {
		return nil, errors.New("mermaid renderer is disabled")
	}

	path, err := exec.LookPath(r.Path)
	if err != nil {
		if info, statErr := os.Stat(r.Path); statErr == nil && notExecutable(info) {
			return nil, errors.New("mermaid renderer is not executable")
		}
		return nil, fmt.Errorf("mermaid renderer does not exist: %w", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("cannot check mermaid renderer: %w", err)
	}
	if info.IsDir() || notExecutable(info) {
		return nil, errors.New("mermaid renderer is not executable")
	}

	timeout := r.Timeout
	if timeout == 0 {
		timeout = defaultTimeout
	}
	tempDir, err := os.MkdirTemp("", "md2pdf-mermaid-")
	if err != nil {
		return nil, fmt.Errorf("cannot create temporary directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	input := filepath.Join(tempDir, "diagram.mmd")
	output := filepath.Join(tempDir, "diagram.png")
	if err := os.WriteFile(input, []byte(diagram), 0o600); err != nil {
		return nil, fmt.Errorf("cannot write temporary diagram: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	var stderr bytes.Buffer
	arguments := []string{"-i", input, "-o", output, "-b", "white", "-q"}
	if r.Scale > 0 {
		arguments = append(arguments, "-s", strconv.FormatFloat(r.Scale, 'f', -1, 64))
	}
	cmd := exec.CommandContext(ctx, path, arguments...)
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("mermaid renderer timed out: %w", ctx.Err())
		}
		return nil, fmt.Errorf("mermaid renderer failed: %s", lastLine(stderr.String()))
	}
	if ctx.Err() != nil {
		return nil, fmt.Errorf("mermaid renderer timed out: %w", ctx.Err())
	}

	if info, err := os.Stat(output); err == nil && info.Size() > MaxPNGSize {
		return nil, fmt.Errorf("mermaid output is too large (%d MB, limit %d MB)", info.Size()>>20, int64(MaxPNGSize)>>20)
	}
	png, err := os.ReadFile(output)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errors.New("output file is missing")
		}
		return nil, fmt.Errorf("cannot read output file: %w", err)
	}
	if len(png) == 0 {
		return nil, errors.New("output file is empty")
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
