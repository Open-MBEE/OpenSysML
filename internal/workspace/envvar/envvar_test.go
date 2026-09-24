package envvar

import (
	"fmt"
	"strings"
	"testing"
)

const (
	name       = "OPENSYSML_ENVVAR_TEST"
	legacyName = "SYSML_ENVVAR_TEST"
)

// capture routes warnings into a buffer and clears the once-per-variable
// record, so each test observes its own warnings.
func capture(t *testing.T) *strings.Builder {
	t.Helper()
	var buf strings.Builder
	orig := warnf
	warnf = func(format string, args ...any) { fmt.Fprintf(&buf, format, args...) }
	t.Cleanup(func() {
		warnf = orig
		warned.Delete(name)
	})
	return &buf
}

func TestLookupNewName(t *testing.T) {
	buf := capture(t)
	t.Setenv(name, "new")
	if got := Lookup(name); got != "new" {
		t.Errorf("Lookup(%s) = %q, want %q", name, got, "new")
	}
	if buf.Len() != 0 {
		t.Errorf("new name warned: %q", buf.String())
	}
}

func TestLookupLegacyFallback(t *testing.T) {
	buf := capture(t)
	t.Setenv(legacyName, "legacy")
	if got := Lookup(name); got != "legacy" {
		t.Errorf("Lookup(%s) = %q, want %q", name, got, "legacy")
	}
	warning := buf.String()
	if !strings.Contains(warning, legacyName) || !strings.Contains(warning, name) {
		t.Errorf("warning %q does not name both %s and %s", warning, legacyName, name)
	}
	buf.Reset()
	Lookup(name)
	if buf.Len() != 0 {
		t.Errorf("second lookup warned again: %q", buf.String())
	}
}

func TestLookupBothSetNewWins(t *testing.T) {
	buf := capture(t)
	t.Setenv(name, "new")
	t.Setenv(legacyName, "legacy")
	if got := Lookup(name); got != "new" {
		t.Errorf("Lookup(%s) = %q, want the OPENSYSML_ value %q", name, got, "new")
	}
	if buf.Len() != 0 {
		t.Errorf("warned although the new name is set: %q", buf.String())
	}
}

func TestLookupEmptyNewNameFallsBack(t *testing.T) {
	capture(t)
	t.Setenv(name, "")
	t.Setenv(legacyName, "legacy")
	if got := Lookup(name); got != "legacy" {
		t.Errorf("Lookup(%s) = %q, want fallback %q", name, got, "legacy")
	}
}

func TestLookupNeitherSet(t *testing.T) {
	capture(t)
	t.Setenv(name, "")
	t.Setenv(legacyName, "")
	if got := Lookup(name); got != "" {
		t.Errorf("Lookup(%s) = %q, want empty", name, got)
	}
}

func TestLegacy(t *testing.T) {
	if got := Legacy("OPENSYSML_MAX_STEPS"); got != "SYSML_MAX_STEPS" {
		t.Errorf("Legacy = %q, want SYSML_MAX_STEPS", got)
	}
}

func TestLookupRejectsUnprefixedName(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("Lookup accepted a name without the OPENSYSML_ prefix")
		}
	}()
	Lookup("SYSML_MAX_STEPS")
}
