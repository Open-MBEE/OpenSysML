// Package querytranslate compiles OSLC Query 3.0 text and the SysML v2 API
// Query into document-query plans, so every query surface runs the native
// semantics of internal/doc/queryexec over one loaded model.
package querytranslate

import "fmt"

// Operator is a comparison operator as a query writes it.
type Operator string

const (
	Equal        Operator = "="
	NotEqual     Operator = "!="
	Less         Operator = "<"
	Greater      Operator = ">"
	LessEqual    Operator = "<="
	GreaterEqual Operator = ">="
	In           Operator = "in"
	InstanceOf   Operator = "instanceOf"
)

// Constraint is one node of a query's filter.
type Constraint interface {
	constraint()
}

// Comparison keeps the rows a property of which compares to one of the values.
type Comparison struct {
	Property string
	Operator Operator
	Values   []string
	Inverse  bool
}

// TypeTest keeps the rows of the named metaclasses: their own metaclass when
// Exact, else any conforming one.
type TypeTest struct {
	Types   []string
	Exact   bool
	Inverse bool
}

// Instances selects the objects the session holds whose definition or usage is
// one of the named elements, by qualified name.
type Instances struct {
	Elements []string
}

// Scoped keeps the rows one of whose Property values satisfies Inner.
type Scoped struct {
	Property string
	Inner    Constraint
}

// Conjunction keeps the rows satisfying every term.
type Conjunction struct {
	Terms []Constraint
}

// Disjunction keeps the rows satisfying any term.
type Disjunction struct {
	Terms []Constraint
}

func (Comparison) constraint()  {}
func (TypeTest) constraint()    {}
func (Instances) constraint()   {}
func (Scoped) constraint()      {}
func (Conjunction) constraint() {}
func (Disjunction) constraint() {}

// Selection is one property to report, with the properties of the elements it
// holds when nested.
type Selection struct {
	Property string
	Nested   []Selection
}

// OrderTerm is one sort key. Desc is true for a '-' prefix.
type OrderTerm struct {
	Property string
	Desc     bool
}

// Query is the common representation OSLC text and the API Query parse into.
type Query struct {
	// Scope names the elements considered, with everything nested inside them;
	// empty considers the whole model.
	Scope       []string
	Select      []Selection
	Where       Constraint
	OrderBy     []OrderTerm
	SearchTerms []string
	// Spelling names, per reported column, the name the request asked for it by,
	// where that differs from the column name.
	Spelling map[string]string
}

// SpellingOf returns the name the request asked for column by, so a frontend
// reports a result in terms its caller can ask the next query in.
func (q Query) SpellingOf(column string) string {
	if spelled, ok := q.Spelling[column]; ok {
		return spelled
	}
	return column
}

// Columns are the names the query's selected properties are reported under:
// a property, or a dotted path for a nested selection.
func (q Query) Columns() []string {
	return selectionColumns("", q.Select)
}

func selectionColumns(prefix string, selections []Selection) []string {
	var out []string
	for _, selection := range selections {
		name := prefix + selection.Property
		if len(selection.Nested) == 0 {
			out = append(out, name)
			continue
		}
		out = append(out, selectionColumns(name+".", selection.Nested)...)
	}
	return out
}

// ErrorKind identifies a query the translator refuses.
type ErrorKind int

const (
	// ErrUnknownProperty identifies a property the metamodel does not define.
	ErrUnknownProperty ErrorKind = iota + 1
	// ErrMalformed identifies malformed query syntax or structure.
	ErrMalformed
	// ErrUnorderedProperty identifies an ordered comparison on a property whose
	// values are not ordered, or instanceOf on a property other than the type.
	ErrUnorderedProperty
	// ErrUnparsableValue identifies a value an operator cannot compare.
	ErrUnparsableValue
	// ErrUnknownScope identifies a scope absent from the model.
	ErrUnknownScope
	// ErrNoModel identifies a query without a loaded model.
	ErrNoModel
	// ErrUnsupportedDisjunction identifies `or` in OSLC text, which the
	// language does not define.
	ErrUnsupportedDisjunction
	// ErrUnsupportedLiteral identifies an unsupported literal form.
	ErrUnsupportedLiteral
	// ErrUnsupportedInstanceTerm identifies an instance selection written where
	// it cannot be the root of the query: negated, scoped or under a disjunction.
	ErrUnsupportedInstanceTerm
)

// Error is a typed query refusal. A refusal must not be represented as an
// empty result, which is indistinguishable from a valid no-match.
type Error struct {
	Kind    ErrorKind
	Message string
	// Candidates are the names the query could have written in place of the
	// unknown one, sorted.
	Candidates []string
}

func (e *Error) Error() string { return e.Message }

func errorf(kind ErrorKind, format string, args ...any) *Error {
	return &Error{Kind: kind, Message: fmt.Sprintf(format, args...)}
}
