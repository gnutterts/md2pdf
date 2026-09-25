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
