package semantics

import (
	"sort"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

const enumSrc = `
enum def Color { red; green; }
enum def PaintColor specializes Color { white; }
enum def GradePoints { A = 4.0; }
part def Car { attribute c : Color; }
`

// TestEnumerationOwning: a literal is an enumeration usage declared in an
// enumeration definition's body, and nothing else is.
func TestEnumerationOwning(t *testing.T) {
	m, root := buildModel(t, enumSrc)
	color := sym(t, root, "Color")

	red := memberNamed(t, m, color, "red")
	if got := EnumerationOwning(red); got != color {
		t.Errorf("EnumerationOwning(red) = %v, want Color", got)
	}
	if got := EnumerationOwning(color); got != nil {
		t.Errorf("EnumerationOwning(Color) = %v, want nil", got)
	}
	car := sym(t, root, "Car")
	if got := EnumerationOwning(memberNamed(t, m, car, "c")); got != nil {
		t.Errorf("EnumerationOwning(Car::c) = %v, want nil — an enum-typed attribute is no literal", got)
	}
	if got := EnumerationOwning(nil); got != nil {
		t.Errorf("EnumerationOwning(nil) = %v, want nil", got)
	}
}

// TestLiteralValue: a literal of an enumeration specializing a scalar type
// declares its value; one identified by itself declares none.
func TestLiteralValue(t *testing.T) {
	m, root := buildModel(t, enumSrc)
	if got := LiteralValue(memberNamed(t, m, sym(t, root, "Color"), "red")); got != nil {
		t.Errorf("LiteralValue(red) = %v, want nil", got)
	}
	if LiteralValue(memberNamed(t, m, sym(t, root, "GradePoints"), "A")) == nil {
		t.Error("LiteralValue(A) = nil, want the declared 4.0")
	}
}

// TestLiteralsOf: an enumeration's literals include the ones it inherits, and
// only literals.
func TestLiteralsOf(t *testing.T) {
	m, root := buildModel(t, enumSrc)
	if got := literalNames(m, sym(t, root, "Color")); got != "green red" {
		t.Errorf("LiteralsOf(Color) = %q, want \"green red\"", got)
	}
	if got := literalNames(m, sym(t, root, "PaintColor")); got != "green red white" {
		t.Errorf("LiteralsOf(PaintColor) = %q, want \"green red white\"", got)
	}
	if got := m.LiteralsOf(sym(t, root, "Car")); got != nil {
		t.Errorf("LiteralsOf(Car) = %v, want nil — a part def declares no literal", got)
	}
}

// memberNamed returns the member of owner named name.
func memberNamed(t *testing.T, m *Model, owner *symbols.Symbol, name string) *symbols.Symbol {
	t.Helper()
	for _, member := range m.MembersOf(owner) {
		if member.Name == name {
			return member
		}
	}
	t.Fatalf("%s has no member %q", owner.Name, name)
	return nil
}

// literalNames renders an enumeration's literals in a stable order.
func literalNames(m *Model, enum *symbols.Symbol) string {
	var names []string
	for _, literal := range m.LiteralsOf(enum) {
		names = append(names, literal.Name)
	}
	sort.Strings(names)
	return strings.Join(names, " ")
}
