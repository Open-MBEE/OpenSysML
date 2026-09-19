package view

import (
	"slices"
	"testing"
)

func TestPseudoViewsIncludeEverySupportedKnownKind(t *testing.T) {
	known := map[Kind]bool{}
	for _, kinds := range []map[string]Kind{standardRenderings, standardViewDefinitions} {
		for _, kind := range kinds {
			if kind != "" {
				known[kind] = true
			}
		}
	}
	for kind := range known {
		got, ok := PseudoViewKind(string(kind))
		if ok != kind.Supported() {
			t.Errorf("%q offered = %t, want Supported() = %t", kind, ok, kind.Supported())
		}
		if ok && got != kind {
			t.Errorf("%q resolves to %q", kind, got)
		}
	}
}

func TestPseudoViewsOmitUnsupportedKinds(t *testing.T) {
	tested := false
	for _, kinds := range []map[string]Kind{standardRenderings, standardViewDefinitions} {
		for _, kind := range kinds {
			if kind == "" || kind.Supported() {
				continue
			}
			tested = true
			if _, ok := PseudoViewKind(string(kind)); ok {
				t.Errorf("unsupported kind %q is offered", kind)
			}
			if slices.Contains(PseudoViewSpecs(), PseudoViewPrefix+string(kind)) {
				t.Errorf("unsupported spec %q is listed", PseudoViewPrefix+string(kind))
			}
		}
	}
	if !tested {
		t.Fatal("the known rendering vocabulary contains no unsupported kind")
	}
}

func TestParsePseudoViewAndSortedSpecs(t *testing.T) {
	kind, target, ok := ParsePseudoView("#state:Machines::Vehicle")
	if !ok || kind != KindState || target != "Machines::Vehicle" {
		t.Errorf("ParsePseudoView = %q, %q, %t", kind, target, ok)
	}
	specs := PseudoViewSpecs()
	if !slices.IsSorted(specs) {
		t.Errorf("specs are not sorted: %v", specs)
	}
	for _, spec := range specs {
		kind, target, ok := ParsePseudoView(spec)
		if !ok || target != "" || !kind.Supported() {
			t.Errorf("ParsePseudoView(%q) = %q, %q, %t", spec, kind, target, ok)
		}
	}
}
