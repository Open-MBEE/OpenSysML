package derived

import (
	"bytes"
	"strings"
	"testing"
)

func TestMainRefusesRootsItCannotMeasure(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := Main([]string{"-roots", "nowhere"}, &stdout, &stderr); code != 2 {
		t.Fatalf("exit %d, want 2: %s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), `unknown root "nowhere"`) {
		t.Errorf("stderr names no unknown root: %s", stderr.String())
	}
}

func TestMainRefusesRootsWithCheck(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := Main([]string{"-roots", "examples", "-check"}, &stdout, &stderr); code != 2 {
		t.Fatalf("exit %d, want 2: %s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "-check compares every root") {
		t.Errorf("stderr does not refuse -roots with -check: %s", stderr.String())
	}
}
