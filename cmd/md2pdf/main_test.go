// SPDX-License-Identifier: MIT

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gnutterts/md2pdf/internal/cli"
	"github.com/gnutterts/md2pdf/internal/render"
)

func sourceFile(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "notes.md")
	if err := os.WriteFile(path, []byte("# Notes\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestAPanicBecomesAnError(t *testing.T) {
	err := runWith([]string{sourceFile(t)}, func(cli.Plan, render.Options) error { panic("boom") })
	if err == nil || err.Error() != "internal error: boom" {
		t.Fatalf("err = %v", err)
	}
}

func TestAPanicWithAnErrorValueBecomesAnError(t *testing.T) {
	err := runWith([]string{sourceFile(t)}, func(cli.Plan, render.Options) error {
		var missing map[string]int
		missing["x"] = 1 // assignment to a nil map
		return nil
	})
	if err == nil || !strings.HasPrefix(err.Error(), "internal error: ") {
		t.Fatalf("err = %v", err)
	}
}

func TestANormalErrorPassesThrough(t *testing.T) {
	err := runWith([]string{sourceFile(t)}, func(cli.Plan, render.Options) error { return os.ErrClosed })
	if err != os.ErrClosed {
		t.Fatalf("err = %v", err)
	}
}

func TestTheExecutorReceivesThePlanAndTheVersion(t *testing.T) {
	var got cli.Plan
	err := runWith([]string{"--strict", sourceFile(t)}, func(plan cli.Plan, options render.Options) error {
		got = plan
		if !options.Strict {
			t.Error("options.Strict is not set")
		}
		return nil
	})
	if err != nil || got.Creator != version {
		t.Fatalf("err = %v, Creator = %q", err, got.Creator)
	}
}

func TestVersionAndHelpAreNotErrors(t *testing.T) {
	for _, args := range [][]string{{"--version"}, {"--help"}} {
		if err := runWith(args, func(cli.Plan, render.Options) error { t.Fatal("executor called"); return nil }); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
	}
}

func TestAnInvalidScaleIsRefused(t *testing.T) {
	err := runWith([]string{"--scale", "9", sourceFile(t)}, func(cli.Plan, render.Options) error { t.Fatal("executor called"); return nil })
	if err == nil || !strings.Contains(err.Error(), "between 1 and 4") {
		t.Fatalf("err = %v", err)
	}
}

func TestTheScaleComesFromTheEnvironment(t *testing.T) {
	t.Setenv("MD2PDF_SCALE", "3")
	var got float64
	err := runWith([]string{sourceFile(t)}, func(_ cli.Plan, options render.Options) error { got = options.Mermaid.Scale; return nil })
	if err != nil || got != 3 {
		t.Fatalf("err = %v, Scale = %v", err, got)
	}
}

func TestTheScaleDefaultsToTwoAndTheFlagBeatsTheEnvironment(t *testing.T) {
	var got float64
	capture := func(_ cli.Plan, options render.Options) error { got = options.Mermaid.Scale; return nil }
	if err := runWith([]string{sourceFile(t)}, capture); err != nil || got != 2 {
		t.Fatalf("default: err = %v, Scale = %v", err, got)
	}
	t.Setenv("MD2PDF_SCALE", "3")
	if err := runWith([]string{"--scale", "1.5", sourceFile(t)}, capture); err != nil || got != 1.5 {
		t.Fatalf("flag: err = %v, Scale = %v", err, got)
	}
}
