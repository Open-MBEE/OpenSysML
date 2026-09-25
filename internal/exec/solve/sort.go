package solve

import (
	"regexp"
	"strings"
)

// SortKind is the kind of value a sort holds. The set is closed: a construct
// whose values fit none of these kinds is not translatable.
type SortKind int

const (
	// SortBool is the boolean sort, SMT-LIB Bool.
	SortBool SortKind = iota
	// SortInt is the integer sort, SMT-LIB Int.
	SortInt
	// SortReal is the real sort, SMT-LIB Real.
	SortReal
	// SortString is the string sort, SMT-LIB String.
	SortString
	// SortDatatype is a finite enumerated sort, declared as an SMT-LIB datatype
	// with one nullary constructor per value: an enumeration definition's
	// literals, or a variation point's variants.
	SortDatatype
)

// Sort is the sort of a term: one of the four scalar sorts, or a finite datatype
// declared by the query.
type Sort struct {
	// Kind is which kind of sort this is.
	Kind SortKind

	// Name is the sort's SMT-LIB name, unquoted: "Bool", "Int", "Real",
	// "String", or the qualified name of the definition a datatype comes from.
	Name string

	// Values are a datatype's constructors, in declaration order, unquoted; nil
	// for a scalar sort.
	Values []string

	// Origin is the enumeration definition or variation point a datatype sort
	// was declared for; nil for a scalar sort.
	Origin string

	// Variation marks a datatype sort declared for a variation point, whose
	// values are its variants rather than an enumeration's literals.
	Variation bool
}

// The scalar sorts, which every query may use without declaring them.
var (
	Bool   = Sort{Kind: SortBool, Name: "Bool"}
	Int    = Sort{Kind: SortInt, Name: "Int"}
	Real   = Sort{Kind: SortReal, Name: "Real"}
	String = Sort{Kind: SortString, Name: "String"}
)

// String returns the sort as it is written in a script.
func (s Sort) String() string { return smtSymbol(s.Name) }

// Equal reports whether two sorts are the same sort.
func (s Sort) Equal(other Sort) bool { return s.Kind == other.Kind && s.Name == other.Name }

// Numeric reports whether the sort is Int or Real.
func (s Sort) Numeric() bool { return s.Kind == SortInt || s.Kind == SortReal }

// simpleSymbol matches an SMT-LIB simple symbol, which needs no quoting.
var simpleSymbol = regexp.MustCompile(`^[a-zA-Z~!@$%^&*_\-+=<>.?/][a-zA-Z0-9~!@$%^&*_\-+=<>.?/]*$`)

// smtSymbol renders a name as an SMT-LIB symbol, quoting one that is not a
// simple symbol — a qualified name, which contains ':'.
func smtSymbol(name string) string {
	if name == "" {
		return "||"
	}
	// '|' and '\' are escaped through '!' before quoting is decided, since quoting
	// is only lexical: |a!pb| and a!pb would otherwise be one symbol.
	replaced := strings.NewReplacer("!", "!!", "|", "!p", "\\", "!b").Replace(name)
	if replaced == name && simpleSymbol.MatchString(name) {
		return name
	}
	return "|" + replaced + "|"
}

// smtName recovers the name an SMT-LIB symbol was rendered from, its `|` bars
// already stripped: it is the inverse of smtSymbol's escaping.
func smtName(symbol string) string {
	var b strings.Builder
	for i := 0; i < len(symbol); i++ {
		if symbol[i] != '!' || i+1 >= len(symbol) {
			b.WriteByte(symbol[i])
			continue
		}
		switch symbol[i+1] {
		case '!':
			b.WriteByte('!')
		case 'p':
			b.WriteByte('|')
		case 'b':
			b.WriteByte('\\')
		default:
			b.WriteByte('!')
			b.WriteByte(symbol[i+1])
		}
		i++
	}
	return b.String()
}
