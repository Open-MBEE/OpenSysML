package analysis

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

// servingRegistry is the default registry with two manifest engines and one tool, as a
// service starts it, serving the names given.
func servingRegistry(t *testing.T, names ...string) (*Registry, error) {
	t.Helper()
	dir := writeManifest(t, map[string]string{
		"spin.json":     engineJSON("spin-bridge", "spin-bridge"),
		"priority.json": `{"kind":"policy","name":"priority","command":["priority-policy"],"protocol":1}`,
		"mc.json":       `{"toolName":"MC","executable":"mc","variables":["x"]}`,
	})
	writeExecutable(t, filepath.Join(dir, "spin-bridge"))
	writeExecutable(t, filepath.Join(dir, "priority-policy"))
	t.Setenv(ToolsEnv, "")
	t.Setenv(EnginesEnv, dir)
	r, err := DefaultFromEnv()
	if err != nil {
		t.Fatalf("DefaultFromEnv: %v", err)
	}
	return r.Serving(names)
}

// listing finds the engine's listing by name.
func listing(t *testing.T, listings []Listing, name string) Listing {
	t.Helper()
	for _, l := range listings {
		if l.Engine == name {
			return l
		}
	}
	t.Fatalf("%s is not listed in %v", name, listings)
	return Listing{}
}

// A registry serving nothing lists every manifest engine withheld: found, described, not
// ready; naming one is the typed refusal, and auto and all do not consult it. Tools and
// built-ins are served as they are.
func TestServingNothingWithholdsEveryManifestEngine(t *testing.T) {
	r, err := servingRegistry(t)
	if err != nil {
		t.Fatalf("Serving(nil): %v", err)
	}
	listings := r.Listings()
	spin := listing(t, listings, "spin-bridge")
	if spin.Served() || spin.Ready() || spin.Status.Err != nil || spin.Status.Process == "" ||
		spin.Origin.Kind != KindEngine || spin.Authority != Bounded {
		t.Errorf("spin-bridge = %+v, want found, described and withheld", spin)
	}
	if got := spin.StatusText(); got != "withheld: engine 'spin-bridge' is not served by this service" {
		t.Errorf("status = %q, want the withholding", got)
	}
	if priority := listing(t, listings, "priority"); priority.Served() || !errors.Is(priority.Status.Err, ErrNotServed) {
		t.Errorf("priority = %+v, want withheld and not served in this build", priority)
	}
	if mc := listing(t, listings, "tool:MC"); !mc.Served() {
		t.Errorf("tool:MC = %+v, want a tool served as it is", mc)
	}
	if check := listing(t, listings, "check"); !check.Served() || !check.Ready() {
		t.Errorf("check = %+v, want served and ready", check)
	}

	_, err = r.Select("spin-bridge")
	var withheld *EngineWithheldError
	if !errors.Is(err, ErrEngineWithheld) || !errors.As(err, &withheld) || withheld.Name != "spin-bridge" {
		t.Errorf("Select(spin-bridge) = %v, want the typed withholding", err)
	}
	if _, err := r.Select("check"); err != nil {
		t.Errorf("Select(check) = %v, want a built-in selected", err)
	}
	if _, err := r.Select("tool:MC"); err != nil {
		t.Errorf("Select(tool:MC) = %v, want a tool selected", err)
	}
	for _, selection := range []Selection{Auto(), All()} {
		engines, err := r.candidates(Holds, selection)
		if err != nil {
			t.Fatalf("candidates %v: %v", selection, err)
		}
		for _, e := range engines {
			if e.Name() == "spin-bridge" {
				t.Errorf("%v consults the withheld spin-bridge: %v", selection, engines)
			}
		}
	}
}

// Naming an engine serves it and it alone; all serves every manifest engine; a name that is
// not a manifest engine is a typed error naming the ones that are.
func TestServingNamesTheManifestEnginesToRun(t *testing.T) {
	r, err := servingRegistry(t, "spin-bridge")
	if err != nil {
		t.Fatalf("Serving(spin-bridge): %v", err)
	}
	listings := r.Listings()
	if spin := listing(t, listings, "spin-bridge"); !spin.Served() || !spin.Ready() {
		t.Errorf("spin-bridge = %+v, want served and ready", spin)
	}
	if priority := listing(t, listings, "priority"); priority.Served() {
		t.Errorf("priority = %+v, want withheld beside the one served", priority)
	}
	if _, err := r.Select("spin-bridge"); err != nil {
		t.Errorf("Select(spin-bridge) = %v, want it served", err)
	}
	if engines, _ := r.candidates(Holds, Auto()); len(engines) == 0 || engines[len(engines)-1].Name() != "spin-bridge" {
		t.Errorf("auto consults %v, want spin-bridge reached after every built-in", engines)
	}

	all, err := servingRegistry(t, ServeAll)
	if err != nil {
		t.Fatalf("Serving(all): %v", err)
	}
	for _, l := range all.Listings() {
		if !l.Served() {
			t.Errorf("%s is withheld under all: %s", l.Engine, l.StatusText())
		}
	}

	_, err = servingRegistry(t, "check")
	var notExternal *NotExternalError
	if !errors.Is(err, ErrNotExternal) || !errors.As(err, &notExternal) || notExternal.Name != "check" ||
		strings.Join(notExternal.Known, ",") != "priority,spin-bridge" {
		t.Errorf("Serving(check) = %v, want the typed refusal naming the manifest engines", err)
	}
	if _, err := servingRegistry(t, "tool:MC"); !errors.Is(err, ErrNotExternal) {
		t.Errorf("Serving(tool:MC) = %v, want a tool refused as not an external engine", err)
	}
}
