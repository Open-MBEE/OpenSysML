package hostcap

import (
	"errors"
	"strings"
	"testing"
)

// TestUnsupportedErrorNamesSubjectAndCause checks the message a WebAssembly build
// reports where an external process would have been started, and the sentinel a
// caller tests for beneath whatever its own layer wraps it in.
func TestUnsupportedErrorNamesSubjectAndCause(t *testing.T) {
	err := &UnsupportedError{Subject: "z3"}

	if got, want := err.Error(), "z3 cannot run: "+ErrSpawnUnsupported.Error(); got != want {
		t.Errorf("message = %q, want %q", got, want)
	}
	if !errors.Is(err, ErrSpawnUnsupported) {
		t.Errorf("errors.Is(%v, ErrSpawnUnsupported) = false, want true", err)
	}
	var unsupported *UnsupportedError
	if !errors.As(err, &unsupported) || unsupported.Subject != "z3" {
		t.Errorf("errors.As to *UnsupportedError = %+v, want subject z3", unsupported)
	}
}

// TestUnsupportedErrorWithoutSubject checks the message when a caller has no one
// thing to name: the cause alone, never a message opening on nothing.
func TestUnsupportedErrorWithoutSubject(t *testing.T) {
	err := &UnsupportedError{}
	if got := err.Error(); !strings.Contains(got, ErrSpawnUnsupported.Error()) {
		t.Errorf("message = %q, want it to carry %q", got, ErrSpawnUnsupported)
	}
	if strings.HasPrefix(err.Error(), " cannot") {
		t.Errorf("message = %q, want no leading subject", err.Error())
	}
}

// TestNativeHostsAreNeverRefused pins the other half of the capability: a native
// build refuses nothing, so every caller keeps looking the tool up and running it.
// A failure here would mean a native build lost process support.
func TestNativeHostsAreNeverRefused(t *testing.T) {
	for _, subject := range []string{"z3", "weasyprint", "go"} {
		if err := CheckSpawn(subject); err != nil {
			t.Errorf("CheckSpawn(%q) = %v, want nil on a native build", subject, err)
		}
	}
}
