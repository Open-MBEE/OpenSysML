package main

import (
	"bytes"
	"context"
	"flag"
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
