package reject

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func boundsValidator(t *testing.T, script string) (repo, validator string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the stub validator is a shell script")
	}
	repo = t.TempDir()
	if err := os.WriteFile(filepath.Join(repo, "Model.sysml"), []byte("package P;\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	validator = filepath.Join(repo, "validate-sysml-batch")
	if err := os.WriteFile(validator, []byte("#!/bin/sh\n"+script+"\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	return repo, validator
}

func TestPilotTimeoutDefaultsToAFiniteLimit(t *testing.T) {
	if defaultPilotTimeout <= 0 {
		t.Fatalf("defaultPilotTimeout = %s, want a finite limit", defaultPilotTimeout)
	}
}

func TestPilotErrorsStopsAValidatorThatHangs(t *testing.T) {
	repo, validator := boundsValidator(t, "exec sleep 600")
	start := time.Now()
	_, err := pilotErrors(validator, repo, ".", []string{"Model.sysml"}, 200*time.Millisecond, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "exceeded 200ms") {
		t.Fatalf("err = %v, want the timeout", err)
	}
	if elapsed := time.Since(start); elapsed > time.Minute {
		t.Errorf("the hung validator held the run for %s", elapsed)
	}
}

func TestPilotErrorsCountsUnattributedLinesPastTheCap(t *testing.T) {
	extra := 25
	repo, validator := boundsValidator(t, fmt.Sprintf(
		"i=0; while [ $i -lt %d ]; do echo \"stray $i\" >&2; i=$((i+1)); done; exit 1", maxUnattributed+extra))
	var log bytes.Buffer
	if _, err := pilotErrors(validator, repo, ".", []string{"Model.sysml"}, time.Minute, &log); err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(log.String(), ": stray "); n != maxUnattributed {
		t.Errorf("log shows %d stray lines, want %d", n, maxUnattributed)
	}
	if want := fmt.Sprintf("%d more lines not shown", extra); !strings.Contains(log.String(), want) {
		t.Errorf("log does not say %q", want)
	}
}

func TestPilotErrorsRefusesMoreThanTheErrorCap(t *testing.T) {
	repo, validator := boundsValidator(t, "exec yes 'Model.sysml:1:1: error: boom' >&2")
	_, err := pilotErrors(validator, repo, ".", []string{"Model.sysml"}, 5*time.Minute, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "more than 1000000 errors") {
		t.Fatalf("err = %v, want the error bound", err)
	}
}
