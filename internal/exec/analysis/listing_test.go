package analysis

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Listings names an external engine's program without starting it; Probed starts each
// external engine once and reports the handshake — a mismatch as the typed status.
func TestListingsSpawnNothingAndProbedSpawnsEachOnce(t *testing.T) {
	record := filepath.Join(t.TempDir(), "record")
	t.Setenv(engineStandinRecord, record)
	t.Setenv(engineStandinDescribe, `{"name":"standin","version":"1.0.0","protocol":1,"answers":["holds"]}`)
	r := Default()
	if err := r.Register(NewEngine(standinEntry(t))); err != nil {
		t.Fatalf("register: %v", err)
	}

	l := listing(t, r.Listings(), "standin")
	if _, err := os.Stat(record); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Listings started the stand-in: record %v", err)
	}
	want := "ready (standin 1.0.0 at " + engineStandin(t) + ")"
	if got := l.StatusText(); got != want {
		t.Errorf("Listings status = %q, want %q", got, want)
	}
	if l.Origin.Kind != KindEngine || l.Origin.Protocol != 1 || l.Origin.Transport != TransportStdio {
		t.Errorf("origin = %+v, want the engine entry's", l.Origin)
	}

	l = listing(t, r.Probed(), "standin")
	if n := processes(t, record); n != 1 {
		t.Fatalf("Probed started %d processes, want 1", n)
	}
	if got := l.StatusText(); got != want[:len(want)-1]+"; describe agrees)" {
		t.Errorf("Probed status = %q", got)
	}

	t.Setenv(engineStandinDescribe, `{"name":"standin","version":"2.0.0","protocol":1,"answers":["holds"]}`)
	l = listing(t, r.Probed(), "standin")
	if n := processes(t, record); n != 2 {
		t.Fatalf("a second probe started %d processes in all, want 2", n)
	}
	var mismatch *HandshakeError
	if !errors.As(l.Status.Err, &mismatch) || mismatch.Field != "version" || l.Ready() {
		t.Fatalf("Probed status = %v, want a HandshakeError on version", l.Status.Err)
	}
	if got := l.StatusText(); !strings.HasPrefix(got, "unavailable: ") || !strings.Contains(got, "version") {
		t.Errorf("status text = %q, want the mismatch as unavailable", got)
	}
}
