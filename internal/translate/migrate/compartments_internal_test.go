package migrate

import (
	"reflect"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/xmi/sysmlv1"
)

func TestHiddenCompartments(t *testing.T) {
	sym := &sysmlv1.Symbol{Compartments: map[string]bool{
		"SUPPRESS_PARTS_COMPARTMENT":          false,
		"SUPPRESS_VALUES_COMPARTMENT":         true,
		"SUPPRESS_CLASS_ATTRIBUTES":           true,
		"SUPPRESS_PROXYPORTS_COMPARTMENT":     true,
		"SUPPRESS_FULLPORTS_COMPARTMENT":      false,
		"SUPPRESS_FLOWPROPERTIES_COMPARTMENT": true,
		"SUPPRESS_STRUCTURE":                  true,
		"SUPPRESS_BEHAVIORS":                  false,
	}}
	// Values and class attributes both suppressed hide the attributes; a
	// proxy ports compartment suppressed beside a full ports one shown leaves
	// the ports listed; parts shown stay.
	hidden, unsupported := hiddenCompartments(sym)
	if want := []string{"Attributes", "Items"}; !reflect.DeepEqual(hidden, want) {
		t.Errorf("hidden = %q, want %q", hidden, want)
	}
	if want := []string{"SUPPRESS_STRUCTURE"}; !reflect.DeepEqual(unsupported, want) {
		t.Errorf("unsupported = %q, want %q", unsupported, want)
	}
	if hidden, unsupported := hiddenCompartments(&sysmlv1.Symbol{}); len(hidden) != 0 || len(unsupported) != 0 {
		t.Errorf("a symbol without settings hides %q, unsupported %q; want none", hidden, unsupported)
	}
}
