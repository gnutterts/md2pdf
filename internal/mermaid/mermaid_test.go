// SPDX-License-Identifier: MIT

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
arguments="$@"
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
		t.Fatalf("png = %q, want %q", png, minimalPNG)
	}
}

func TestToPNGPassesCorrectArguments(t *testing.T) {
	data := filepath.Join(t.TempDir(), "data")
	script := writeScript(t, paths()+`{
printf '%s\n' $arguments
printf '%s\n' '--content--'
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
	for _, argument := range []string{"-i\n", "-o\n", "-b\n", "white\n", "-q\n", "--content--\n" + diagram} {
		if !strings.Contains(text, argument) {
			t.Errorf("%q is missing in %q", argument, text)
		}
	}
}

func TestToPNGExitCodeMessage(t *testing.T) {
	script := writeScript(t, `echo first >&2
echo last error >&2
exit 7`)
	_, err := (Renderer{Path: script}).ToPNG("")
	if err == nil || !strings.Contains(err.Error(), "last error") {
		t.Fatalf("error = %v", err)
	}
}

func TestToPNGMissingOutput(t *testing.T) {
	script := writeScript(t, "exit 0")
	_, err := (Renderer{Path: script}).ToPNG("")
	if err == nil || !strings.Contains(err.Error(), "is missing") {
		t.Fatalf("error = %v", err)
	}
}

func TestToPNGEmptyOutput(t *testing.T) {
	script := writeScript(t, paths()+`: > "$out"`)
	_, err := (Renderer{Path: script}).ToPNG("")
	if err == nil || !strings.Contains(err.Error(), "empty") {
		t.Fatalf("error = %v", err)
	}
}

func TestToPNGRendererDoesNotExist(t *testing.T) {
	_, err := (Renderer{Path: filepath.Join(t.TempDir(), "nergens")}).ToPNG("")
	if err == nil || !strings.Contains(err.Error(), "does not exist") {
		t.Fatalf("error = %v", err)
	}
}

func TestToPNGRendererNotExecutable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "no-output")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := (Renderer{Path: path}).ToPNG("")
	if err == nil || !strings.Contains(err.Error(), "not executable") {
		t.Fatalf("error = %v", err)
	}
}

func TestToPNGTimeout(t *testing.T) {
	script := writeScript(t, "exec sleep 5")
	start := time.Now()
	_, err := (Renderer{Path: script, Timeout: 100 * time.Millisecond}).ToPNG("")
	if err == nil || !strings.Contains(err.Error(), "timed out") || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error = %v", err)
	}
	if timeout := time.Since(start); timeout >= time.Second {
		t.Fatalf("timeout took %v", timeout)
	}
}

func TestToPNGRemovesFiles(t *testing.T) {
	for _, caseTest := range []struct {
		name string
		body string
	}{
		{"succeeds", `printf x > "$out"`},
		{"fails", "echo broken >&2\nexit 1"},
	} {
		t.Run(caseTest.name, func(t *testing.T) {
			record := filepath.Join(t.TempDir(), "directory")
			script := writeScript(t, paths()+`dirname "$out" > `+shellText(record)+"\n"+caseTest.body)
			_, _ = (Renderer{Path: script}).ToPNG("")
			tempDir, err := os.ReadFile(record)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(strings.TrimSpace(string(tempDir))); !os.IsNotExist(err) {
				t.Fatalf("temporary directory still exists: %v", err)
			}
		})
	}
}

func TestChoose(t *testing.T) {
	for _, caseTest := range []struct {
		name, flag, environment, want string
	}{
		{"flag wins", "flag", "environment", "flag"},
		{"environment wins", "", "environment", "environment"},
		{"default", "", "", "mmdc"},
		{"off", "off", "environment", ""},
		{"none", "none", "environment", ""},
	} {
		t.Run(caseTest.name, func(t *testing.T) {
			if got := Choose(caseTest.flag, caseTest.environment).Path; got != caseTest.want {
				t.Fatalf("Path = %q, want %q", got, caseTest.want)
			}
		})
	}
}

func TestDisabled(t *testing.T) {
	if (Renderer{}).Available() {
		t.Fatal("empty renderer is available")
	}
	_, err := (Renderer{}).ToPNG("")
	if err == nil || !strings.Contains(err.Error(), "disabled") {
		t.Fatalf("error = %v", err)
	}
}

func shellText(text string) string {
	return "'" + strings.ReplaceAll(text, "'", "'\\\"'\\\"'") + "'"
}

func TestToPNGPassesTheScaleOnlyWhenSet(t *testing.T) {
	for _, test := range []struct {
		scale float64
		want  string // the arguments after -q, or "" for none
	}{{0, ""}, {2, "-s\n2\n"}, {1.5, "-s\n1.5\n"}} {
		data := filepath.Join(t.TempDir(), "data")
		script := writeScript(t, paths()+`printf '%s\n' $arguments > `+shellText(data)+`
printf x > "$out"`)
		if _, err := (Renderer{Path: script, Scale: test.scale}).ToPNG("graph TD"); err != nil {
			t.Fatal(err)
		}
		content, _ := os.ReadFile(data)
		text := string(content)
		if test.want == "" && strings.Contains(text, "-s\n") {
			t.Errorf("scale %v: -s passed anyway: %q", test.scale, text)
		}
		if test.want != "" && !strings.HasSuffix(text, test.want) {
			t.Errorf("scale %v: arguments %q do not end with %q", test.scale, text, test.want)
		}
	}
}

func TestChooseScale(t *testing.T) {
	for _, test := range []struct {
		flag, environment string
		want              float64
		bad               bool
	}{
		{"", "", 2, false}, {"3", "", 3, false}, {"1.5", "", 1.5, false}, {"", "3", 3, false},
		{"1", "4", 1, false}, {"", "abc", 0, true}, {"9", "", 0, true}, {"0.5", "", 0, true},
		{"NaN", "", 0, true}, {"Inf", "", 0, true}, {"-1", "", 0, true},
	} {
		got, err := ChooseScale(test.flag, test.environment)
		if test.bad {
			if err == nil {
				t.Errorf("ChooseScale(%q, %q) = %v, want an error", test.flag, test.environment, got)
			}
			continue
		}
		if err != nil || got != test.want {
			t.Errorf("ChooseScale(%q, %q) = %v, %v, want %v", test.flag, test.environment, got, err, test.want)
		}
	}
}
