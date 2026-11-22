//go:build !sysml_re && !sysml_nofmi

package convert_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
)

// An .fmu input imports as a calc def, then converts on to any output format.
func TestConvertFMUToNotationAndTurtle(t *testing.T) {
	data := fmuBytes(t, ballXML)
	dir := t.TempDir()
	name := filepath.Join(dir, "ball.fmu")
	if err := os.WriteFile(name, data, 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := convert.Convert(name, data, convert.FormatFMU, convert.FormatSysML)
	if err != nil {
		t.Fatalf("Convert fmu to sysml: %v", err)
	}
	text := string(out)
	for _, want := range []string{"calc def BouncingBall", `toolName = "fmi"`, "file://" + name} {
		if !strings.Contains(text, want) {
			t.Errorf("notation lacks %q:\n%s", want, text)
		}
	}
	ttl, err := convert.Convert(name, data, convert.FormatFMU, convert.FormatTurtle)
	if err != nil {
		t.Fatalf("Convert fmu to ttl: %v", err)
	}
	if !strings.Contains(string(ttl), "BouncingBall") {
		t.Errorf("turtle lacks the calc def:\n%s", ttl)
	}
}

// An FMU that names no file — standard input, an inline document — cannot
// carry a uri: the import is refused instead of writing an unusable one.
func TestConvertFMUWithNoFileIsRefused(t *testing.T) {
	data := fmuBytes(t, ballXML)
	var nf *convert.FMUNotAFileError
	if _, err := convert.Convert("<stdin>", data, convert.FormatFMU, convert.FormatSysML); !errors.As(err, &nf) {
		t.Fatalf("Convert from <stdin> = %v, want FMUNotAFileError", err)
	}
	if _, err := convert.Convert("<stdin>", data, convert.FormatFMU, convert.FormatSysML); !errors.Is(err, convert.ErrFMUNotAFile) {
		t.Fatalf("Convert from <stdin> = %v, want ErrFMUNotAFile", err)
	}
}

// A name on disk that no longer holds the converted bytes is refused: the uri
// would name an archive the notation never read.
func TestConvertFMURefusesAChangedArchive(t *testing.T) {
	const other = `<fmiModelDescription fmiVersion="2.0" modelName="Other" guid="{2}"><CoSimulation modelIdentifier="Other"/><ModelVariables><ScalarVariable name="x" valueReference="1" causality="output"><Real/></ScalarVariable></ModelVariables></fmiModelDescription>`
	name := filepath.Join(t.TempDir(), "ball.fmu")
	if err := os.WriteFile(name, fmuBytes(t, ballXML), 0o644); err != nil {
		t.Fatal(err)
	}
	var changed *convert.FMUChangedError
	_, err := convert.Convert(name, fmuBytes(t, other), convert.FormatFMU, convert.FormatSysML)
	if !errors.As(err, &changed) || !errors.Is(err, convert.ErrFMUChanged) {
		t.Fatalf("Convert of different bytes = %v, want FMUChangedError", err)
	}
}

// The uri the notation carries percent-escapes the path's URL metacharacters.
func TestConvertFMUEscapesTheFileURI(t *testing.T) {
	data := fmuBytes(t, ballXML)
	for _, tc := range []struct{ file, want string }{
		{"gear#2.fmu", "gear%232.fmu"},
		{"a?b.fmu", "a%3Fb.fmu"},
	} {
		name := filepath.Join(t.TempDir(), tc.file)
		if err := os.WriteFile(name, data, 0o644); err != nil {
			t.Fatal(err)
		}
		out, err := convert.Convert(name, data, convert.FormatFMU, convert.FormatSysML)
		if err != nil {
			t.Fatalf("Convert: %v", err)
		}
		if !strings.Contains(string(out), tc.want) {
			t.Errorf("notation lacks the escaped %q:\n%s", tc.want, out)
		}
	}
}
