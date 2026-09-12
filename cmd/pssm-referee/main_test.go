package main

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/pssm"
)

const repoRoot = "../.."

// The first sentence of -h says what a pass means, in the alignment note's words.
func TestHelpOpensWithTheMeaningOfAPass(t *testing.T) {
	flags := flag.NewFlagSet("pssm-referee", flag.ContinueOnError)
	var out bytes.Buffer
	flags.SetOutput(&out)
	if _, err := parse(flags, []string{"-h"}); err != flag.ErrHelp {
		t.Fatalf("parse -h: %v", err)
	}
	want := "A pass checks that the runtime reproduces UML behavior where the model has a defensible " +
		"SysML v2 mapping, provides a second opinion on the tool-choice rows, and is never evidence " +
		"of SysML v2 conformance."
	if !strings.HasPrefix(out.String(), want+"\n") {
		t.Errorf("help opens with:\n%s", out.String())
	}
}

// An absent suite is a reported skip, exit 0, unless the require variable is set.
func TestAbsentSuite(t *testing.T) {
	empty := t.TempDir()
	t.Setenv(pssm.RequireEnv, "")
	var out, log bytes.Buffer
	if err := run(&out, &log, options{repo: repoRoot, suite: empty, jobs: 1}); err != nil {
		t.Fatalf("absent suite: %v", err)
	}
	if !strings.Contains(out.String(), "download-pssm-suite.sh") {
		t.Errorf("absent suite does not say how to provision it:\n%s", out.String())
	}

	t.Setenv(pssm.RequireEnv, "1")
	err := run(&out, &log, options{repo: repoRoot, suite: empty, jobs: 1})
	if err == nil || !strings.Contains(err.Error(), pssm.RequireEnv) {
		t.Errorf("required absent suite: %v", err)
	}
}

// A suite file whose bytes are not the pinned ones is refused before it is read.
func TestBadChecksum(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, pssm.SuiteFile), []byte("<xmi:XMI/>"), 0o600); err != nil {
		t.Fatal(err)
	}
	var out, log bytes.Buffer
	err := run(&out, &log, options{repo: repoRoot, suite: root, jobs: 1})
	if err == nil || !strings.Contains(err.Error(), "not the pinned") {
		t.Errorf("tampered suite: %v", err)
	}
	if out.Len() != 0 {
		t.Errorf("tampered suite produced a report:\n%s", out.String())
	}
}

// Flag combinations that cannot mean anything are refused.
func TestRefusedOptions(t *testing.T) {
	var out, log bytes.Buffer
	if err := run(&out, &log, options{repo: repoRoot, jobs: 0}); err == nil {
		t.Error("-jobs 0 accepted")
	}
	if err := run(&out, &log, options{repo: repoRoot, jobs: 1, update: true, filter: "x"}); err == nil {
		t.Error("-update with -filter accepted")
	}
	if err := run(&out, &log, options{repo: repoRoot, jobs: 1, develop: "abc"}); err == nil {
		t.Error("-develop without -update accepted")
	}
}
