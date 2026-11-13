package main

import (
	"bytes"
	"context"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/tools/referee/hideo"
)

// -h says what a pass means before it lists the flags.
func TestHelpSaysWhatAPassMeans(t *testing.T) {
	flags := flag.NewFlagSet("hideo-suite", flag.ContinueOnError)
	var out bytes.Buffer
	flags.SetOutput(&out)
	if _, err := parse(flags, []string{"-h"}); err != flag.ErrHelp {
		t.Fatalf("parse -h: %v", err)
	}
	for _, want := range []string{"never evidence of conformance", "-check", "-update", "-accept-regressions", "-no-run"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("help lacks %q:\n%s", want, out.String())
		}
	}
}

// An absent suite is a reported skip, exit 0, unless the require variable is set.
func TestAbsentSuite(t *testing.T) {
	empty := t.TempDir()
	t.Setenv(hideo.RequireEnv, "")
	var out, log bytes.Buffer
	if err := run(context.Background(), &out, &log, options{suite: empty, check: true}); err != nil {
		t.Fatalf("absent suite: %v", err)
	}
	if !strings.Contains(out.String(), "download-hideo-suite.sh") {
		t.Errorf("absent suite does not say how to provision it:\n%s", out.String())
	}

	t.Setenv(hideo.RequireEnv, "1")
	err := run(context.Background(), &out, &log, options{suite: empty, check: true})
	if err == nil || !strings.Contains(err.Error(), hideo.RequireEnv) {
		t.Errorf("required absent suite: %v", err)
	}
}

// Flag combinations that cannot mean anything are refused.
func TestRefusedOptions(t *testing.T) {
	var out, log bytes.Buffer
	if err := run(context.Background(), &out, &log, options{check: true, update: true}); err == nil {
		t.Error("-check with -update accepted")
	}
	if err := run(context.Background(), &out, &log, options{check: true, acceptRegressions: true}); err == nil {
		t.Error("-accept-regressions without -update accepted")
	}
}

// Every reader running the same cases, none got worse.
func runOf(ids ...string) hideo.Run {
	run := hideo.Run{}
	for _, suite := range hideo.Suites {
		run[suite] = map[string]hideo.Result{}
		for _, id := range ids {
			run[suite][id] = hideo.Result{Outcome: hideo.Pass}
		}
	}
	return run
}

// At one pin, a run in which every reader lost the same cases is refused, and nothing is rewritten.
func TestUpdateRefusesARunThatLostCasesAtTheSamePin(t *testing.T) {
	dir := t.TempDir()
	committed, doc := filepath.Join(dir, "baseline.tsv"), filepath.Join(dir, "page.md")
	page := "<!-- hideo-suite:begin counts -->\n<!-- hideo-suite:end counts -->\n"
	if err := os.WriteFile(doc, []byte(page), 0o600); err != nil {
		t.Fatal(err)
	}
	pin := hideo.Pin{Repository: "https://example.org/suite.git", Commit: strings.Repeat("a", 40)}
	var out bytes.Buffer
	if err := record(&out, committed, doc, pin, runOf("A01-001", "A01-002"), false); err != nil {
		t.Fatalf("first record: %v", err)
	}
	before, err := os.ReadFile(committed)
	if err != nil {
		t.Fatal(err)
	}

	out.Reset()
	err = record(&out, committed, doc, pin, runOf("A01-001"), true)
	if err == nil || !strings.Contains(out.String(), "A01-002") {
		t.Fatalf("a run short of A01-002: err = %v, output:\n%s", err, out.String())
	}
	if after, _ := os.ReadFile(committed); !bytes.Equal(before, after) {
		t.Errorf("the refused run rewrote the baseline:\n%s", after)
	}

	moved := pin
	moved.Commit = strings.Repeat("b", 40)
	if err := record(&out, committed, doc, moved, runOf("A01-001"), false); err != nil {
		t.Errorf("another pin brings its own cases: %v", err)
	}
}
