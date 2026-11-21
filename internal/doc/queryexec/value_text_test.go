package queryexec

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// ValueText prints the zero value as nothing and, given no naming function,
// an element by its qualified name, a nil element included.
func TestValueTextToleratesNilAndZeroValues(t *testing.T) {
	if got := ValueText(Value{}, nil); got != "" {
		t.Errorf("zero value = %q, want empty", got)
	}
	if got := ValueText(ElementValue(nil), nil); got != "" {
		t.Errorf("nil element = %q, want empty", got)
	}
	element := &symbols.Symbol{Name: "Bench"}
	if got, want := ValueText(ElementValue(element), nil), symbols.FQNOf(element); got != want {
		t.Errorf("element without a naming function = %q, want %q", got, want)
	}
	if got := ValueText(ConnectorEndValue(element, 0, "camera.usb"), nil); got != "camera.usb" {
		t.Errorf("connector end = %q, want its label", got)
	}
}
