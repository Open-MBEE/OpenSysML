package exec

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func stubLauncher(t *testing.T, script string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the stub launcher is a shell script")
	}
	path := filepath.Join(t.TempDir(), "eval-sysml")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+script+"\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestRunPilotStopsALauncherThatHangs(t *testing.T) {
	launcher := stubLauncher(t, "exec sleep 600")
	start := time.Now()
	_, err := runPilot(launcher, nil, []execCase{{ID: "a"}}, 200*time.Millisecond)
	if err == nil || !strings.Contains(err.Error(), "exceeded 200ms") {
		t.Fatalf("err = %v, want the timeout", err)
	}
	if elapsed := time.Since(start); elapsed > time.Minute {
		t.Errorf("the hung launcher held the run for %s", elapsed)
	}
}

func TestRunPilotStopsALauncherThatFloodsAStream(t *testing.T) {
	for _, stream := range []string{"", " >&2"} {
		launcher := stubLauncher(t, "exec yes 0123456789abcdef"+stream)
		_, err := runPilot(launcher, nil, []execCase{{ID: "a"}}, 5*time.Minute)
		if err == nil || !strings.Contains(err.Error(), "wrote more than") {
			t.Fatalf("stream %q: err = %v, want the output bound", stream, err)
		}
	}
}

func TestRunPilotStillReportsALauncherFailurePerCase(t *testing.T) {
	launcher := stubLauncher(t, "echo broken >&2; exit 3")
	got, err := runPilot(launcher, nil, []execCase{{ID: "a"}}, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if got["a"] != "ERROR:launcher: broken" {
		t.Errorf("result = %q", got["a"])
	}
}

func TestMainRefusesANonPositiveTimeout(t *testing.T) {
	var stderr strings.Builder
	if code := Main([]string{"-timeout", "0"}, &stderr); code != 2 {
		t.Errorf("exit = %d, want 2; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "-timeout must be positive") {
		t.Errorf("stderr = %q", stderr.String())
	}
}
