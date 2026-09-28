package fmi

import (
	"archive/zip"
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// writeFMU writes a zip archive under name in the test's directory holding
// modelDescription.xml with xml's contents and one file per platform named
// under binaries/<platform>/.
func writeFMU(t *testing.T, name, xml string, platforms ...string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	entry, err := w.Create("modelDescription.xml")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entry.Write([]byte(xml)); err != nil {
		t.Fatal(err)
	}
	for _, p := range platforms {
		f, err := w.Create("binaries/" + p + "/lib.so")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.Write([]byte{0}); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func fixtureXML(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestReadsFMI2Description(t *testing.T) {
	d, err := ParseModelDescription([]byte(fixtureXML(t, "bouncingball-2.0.xml")))
	if err != nil {
		t.Fatalf("ParseModelDescription: %v", err)
	}
	if d.FMIVersion != Version2 || d.ModelName != "BouncingBall" || d.Description != "A bouncing ball." {
		t.Fatalf("description = %+v", d)
	}
	if d.InstantiationToken != "{8c4e810f-3df3-4a00-8276-176fa3c9f000}" {
		t.Fatalf("InstantiationToken = %q, want the guid", d.InstantiationToken)
	}
	if d.CoSimulation == nil || d.CoSimulation.ModelIdentifier != "BouncingBall" ||
		!d.CoSimulation.NeedsExecutionTool || !d.CoSimulation.CanHandleVariableCommunicationStepSize {
		t.Fatalf("CoSimulation = %+v", d.CoSimulation)
	}
	if d.ModelExchange != nil || d.ScheduledExecution != nil {
		t.Fatalf("ModelExchange = %+v, ScheduledExecution = %+v", d.ModelExchange, d.ScheduledExecution)
	}
	if d.DefaultExperiment == nil || !d.DefaultExperiment.HasStart || !d.DefaultExperiment.HasStop ||
		!d.DefaultExperiment.HasStepSize || !d.DefaultExperiment.HasTolerance ||
		d.DefaultExperiment.StopTime != 3.0 || d.DefaultExperiment.StepSize != 0.01 {
		t.Fatalf("DefaultExperiment = %+v", d.DefaultExperiment)
	}
	if len(d.Units) != 3 {
		t.Fatalf("Units = %+v", d.Units)
	}
	u, ok := d.Unit("m/s2")
	if !ok || u.Base == nil || u.Base.M != 1 || u.Base.S != -2 || u.Base.Factor != 1.0 {
		t.Fatalf("Unit(m/s2) = %+v", u)
	}
	if len(d.Variables) != 5 {
		t.Fatalf("Variables = %+v", d.Variables)
	}
	h, ok := d.Variable("h")
	if !ok {
		t.Fatal("variable h not found")
	}
	if h.Causality != CausalityOutput || h.Kind != KindReal || h.FMIType != "Real" ||
		h.Unit != "m" || h.DeclaredType != "Position" || !h.HasStart || h.Start != "1.0" {
		t.Fatalf("h = %+v", h)
	}
	g, _ := d.Variable("g")
	if g.Causality != CausalityParameter || g.Variability != VariabilityFixed || g.Unit != "m/s2" {
		t.Fatalf("g = %+v", g)
	}
	if len(d.Inputs()) != 2 || len(d.Outputs()) != 3 {
		t.Fatalf("Inputs = %+v, Outputs = %+v", d.Inputs(), d.Outputs())
	}
}

func TestReadsFMI3Description(t *testing.T) {
	d, err := ParseModelDescription([]byte(fixtureXML(t, "mixed-3.0.xml")))
	if err != nil {
		t.Fatalf("ParseModelDescription: %v", err)
	}
	if d.FMIVersion != Version3 || d.InstantiationToken != "token-abc-123" {
		t.Fatalf("description = %+v", d)
	}
	if d.CoSimulation == nil || d.ModelExchange == nil || d.ScheduledExecution == nil {
		t.Fatal("expected all three interfaces")
	}
	kinds := map[string]Kind{}
	for _, v := range d.Variables {
		kinds[v.Name] = v.Kind
	}
	want := map[string]Kind{
		"m": KindReal, "count": KindInteger, "ok": KindBoolean, "label": KindString,
		"mode": KindEnumeration, "tick": KindClock, "blob": KindBinary,
		"grid": KindReal, "n": KindInteger, "t": KindReal,
	}
	for name, k := range want {
		if kinds[name] != k {
			t.Errorf("%s kind = %s, want %s", name, kinds[name], k)
		}
	}
	m, _ := d.Variable("m")
	if m.Unit != "kg" || m.DeclaredType != "mass" {
		t.Fatalf("m = %+v, want the declaredType's unit", m)
	}
	grid, _ := d.Variable("grid")
	if !grid.Array {
		t.Fatal("grid should read as an array")
	}
	tv, _ := d.Variable("t")
	if tv.Causality != CausalityIndependent || tv.Variability != VariabilityContinuous {
		t.Fatalf("t = %+v", tv)
	}
}

func TestReadsFMI1Description(t *testing.T) {
	d, err := ParseModelDescription([]byte(fixtureXML(t, "legacy-1.0.xml")))
	if err != nil {
		t.Fatalf("ParseModelDescription: %v", err)
	}
	if d.FMIVersion != Version1 {
		t.Fatalf("FMIVersion = %s", d.FMIVersion)
	}
	if d.CoSimulation == nil || d.CoSimulation.ModelIdentifier != "legacy_cs" || !d.CoSimulation.NeedsExecutionTool {
		t.Fatalf("CoSimulation = %+v", d.CoSimulation)
	}
	want := map[string]Causality{"x": CausalityInput, "y": CausalityOutput, "z": CausalityLocal, "w": CausalityLocal}
	for name, c := range want {
		v, ok := d.Variable(name)
		if !ok || v.Causality != c {
			t.Errorf("%s causality = %v, want %s", name, v.Causality, c)
		}
	}
}

func TestFMI1WithoutImplementationIsModelExchange(t *testing.T) {
	xml := `<fmiModelDescription fmiVersion="1.0" modelName="ME" modelIdentifier="me"/>`
	d, err := ParseModelDescription([]byte(xml))
	if err != nil {
		t.Fatalf("ParseModelDescription: %v", err)
	}
	if d.ModelExchange == nil || d.ModelExchange.ModelIdentifier != "me" || d.CoSimulation != nil {
		t.Fatalf("ModelExchange = %+v", d.ModelExchange)
	}
}

func TestReadAndReadArchiveFindPlatforms(t *testing.T) {
	path := writeFMU(t, "ball.fmu", fixtureXML(t, "bouncingball-2.0.xml"), "x86_64-linux", "win64")
	d, err := Read(path)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if len(d.Platforms) != 2 || d.Platforms[0] != "win64" && d.Platforms[0] != "x86_64-linux" {
		t.Fatalf("Platforms = %v", d.Platforms)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	d2, err := ReadArchive(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("ReadArchive: %v", err)
	}
	if d2.ModelName != "BouncingBall" {
		t.Fatalf("ModelName = %q", d2.ModelName)
	}
}

func TestRefusesNonFMU(t *testing.T) {
	if _, err := ParseModelDescription([]byte("not xml")); err == nil {
		t.Fatal("expected an error for non-XML")
	}
	text := filepath.Join(t.TempDir(), "text.fmu")
	if err := os.WriteFile(text, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Read(text); !errors.Is(err, ErrNotFMU) {
		t.Fatalf("Read(text file) = %v, want ErrNotFMU", err)
	}
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	if _, err := w.Create("readme.txt"); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadArchive(bytes.NewReader(buf.Bytes()), int64(buf.Len())); !errors.Is(err, ErrNotFMU) {
		t.Fatalf("ReadArchive(zip without modelDescription) = %v, want ErrNotFMU", err)
	}
}

func TestRefusesUnsupportedVersion(t *testing.T) {
	for _, v := range []string{"3.0-beta.1", "2.1", "9.9"} {
		xml := `<fmiModelDescription fmiVersion="` + v + `" modelName="x"/>`
		var unsupported *UnsupportedVersionError
		if _, err := ParseModelDescription([]byte(xml)); !errors.As(err, &unsupported) {
			t.Fatalf("fmiVersion %s = %v, want UnsupportedVersionError", v, err)
		}
	}
}

func TestRefusesMalformedDescriptions(t *testing.T) {
	cases := map[string]string{
		"duplicate name":  `<fmiModelDescription fmiVersion="2.0" modelName="x"><ModelVariables><ScalarVariable name="a" valueReference="1"><Real/></ScalarVariable><ScalarVariable name="a" valueReference="2"><Real/></ScalarVariable></ModelVariables></fmiModelDescription>`,
		"missing name":    `<fmiModelDescription fmiVersion="2.0" modelName="x"><ModelVariables><ScalarVariable valueReference="1"><Real/></ScalarVariable></ModelVariables></fmiModelDescription>`,
		"missing vr":      `<fmiModelDescription fmiVersion="2.0" modelName="x"><ModelVariables><ScalarVariable name="a"><Real/></ScalarVariable></ModelVariables></fmiModelDescription>`,
		"bad causality":   `<fmiModelDescription fmiVersion="2.0" modelName="x"><ModelVariables><ScalarVariable name="a" valueReference="1" causality="sideways"><Real/></ScalarVariable></ModelVariables></fmiModelDescription>`,
		"unknown type":    `<fmiModelDescription fmiVersion="2.0" modelName="x"><ModelVariables><ScalarVariable name="a" valueReference="1"><Frob/></ScalarVariable></ModelVariables></fmiModelDescription>`,
		"bad causality 3": `<fmiModelDescription fmiVersion="3.0" modelName="x"><ModelVariables><Float64 name="a" valueReference="1" causality="sideways"/></ModelVariables></fmiModelDescription>`,
		"unknown type 3":  `<fmiModelDescription fmiVersion="3.0" modelName="x"><ModelVariables><Frob name="a" valueReference="1"/></ModelVariables></fmiModelDescription>`,
	}
	for name, xml := range cases {
		t.Run(name, func(t *testing.T) {
			var malformed *ModelDescriptionError
			if _, err := ParseModelDescription([]byte(xml)); !errors.As(err, &malformed) {
				t.Fatalf("ParseModelDescription = %v, want ModelDescriptionError", err)
			}
		})
	}
}
