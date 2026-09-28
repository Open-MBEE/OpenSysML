package convert_test

import (
	"archive/zip"
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
)

// fmuBytes zips one modelDescription.xml as an .fmu's contents.
func fmuBytes(t *testing.T, xml string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	entry, err := w.Create("modelDescription.xml")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entry.Write([]byte(xml)); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

const ballXML = `<fmiModelDescription fmiVersion="2.0" modelName="BouncingBall" guid="{1}" description="A ball.">
	<CoSimulation modelIdentifier="BouncingBall"/>
	<ModelVariables>
		<ScalarVariable name="g" valueReference="1" causality="parameter"><Real start="-9.81"/></ScalarVariable>
		<ScalarVariable name="h" valueReference="2" causality="output"><Real start="1.0"/></ScalarVariable>
	</ModelVariables>
	<DefaultExperiment startTime="0.0" stopTime="3.0"/>
</fmiModelDescription>`

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

// A .fmu extension names the format; the name resolves on the command line.
func TestFMUFormatSurfaces(t *testing.T) {
	if f, err := convert.FormatOfPath("model.fmu"); err != nil || f != convert.FormatFMU {
		t.Fatalf("FormatOfPath(model.fmu) = %v, %v", f, err)
	}
	if f, err := convert.ParseFormat("fmu"); err != nil || f != convert.FormatFMU {
		t.Fatalf("ParseFormat(fmu) = %v, %v", f, err)
	}
	if convert.FormatFMU.Writable() {
		t.Fatal("fmu is an input format only")
	}
	var nw *convert.NotWritableError
	if _, err := convert.Convert("x.sysml", []byte("package P {}"), convert.FormatSysML, convert.FormatFMU); !errors.As(err, &nw) {
		t.Fatalf("convert to fmu = %v, want NotWritableError", err)
	}
	if !strings.Contains(convert.FormatList, "fmu") {
		t.Fatalf("FormatList %q lacks fmu", convert.FormatList)
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

// A file that is not an FMU fails the conversion, not as notation.
func TestConvertRejectsNonFMU(t *testing.T) {
	if _, err := convert.Convert("text.fmu", []byte("hello"), convert.FormatFMU, convert.FormatSysML); err == nil {
		t.Fatal("expected a refusal for non-FMU input")
	}
}
