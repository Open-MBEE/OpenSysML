package runtime

import (
	"sort"
	"strings"
	"unicode"
)

// Builtin is one library function this runtime implements directly, described
// for a caller that lists them: the unqualified name a call may use, the
// library declaration that name denotes, and the parameter names of that
// declaration when the registry knows them.
type Builtin struct {
	Name   string
	FQN    string
	Params []string
	// Collection is true for the operations over sequences, collections and
	// bodies, which are the ones written in the postfix `x->name()` form.
	Collection bool
	// Package is the library package that declares the function, which a model
	// imports for a call by the unqualified name to be legal: the name resolves
	// as any other does, so `import RealFunctions::*;` is what makes a bare
	// `sqrt(x)` denote RealFunctions::sqrt.
	Package string
}

// Builtins returns every library function this build implements that a model
// calls by name, in name order, each with the package a call by that name needs
// imported. It is the registry behind the REPL's %builtins listing, so what the
// REPL advertises is what this build implements. A declaration registered only
// to report itself as not evaluable is left out, as is one named as an operator,
// which a model writes as that operator rather than as a call.
func Builtins() []Builtin {
	out := make([]Builtin, 0, len(libraryFunctions)+len(builtins))
	for fqn, fn := range libraryFunctions {
		if fn.unevaluable {
			continue
		}
		if b, ok := builtinNamed(fqn); ok {
			b.Params = fn.paramNames()
			out = append(out, b)
		}
	}
	for fqn := range builtins {
		if b, ok := builtinNamed(fqn); ok {
			b.Collection = true
			out = append(out, b)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Name != out[j].Name {
			return out[i].Name < out[j].Name
		}
		return out[i].FQN < out[j].FQN
	})
	return out
}

// builtinNamed splits fqn into the package that declares it and the name a call
// writes, reporting false for a declaration named as an operator.
func builtinNamed(fqn string) (Builtin, bool) {
	cut := strings.LastIndex(fqn, "::")
	if cut < 0 {
		return Builtin{}, false
	}
	name := fqn[cut+len("::"):]
	for _, r := range name {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' {
			return Builtin{}, false
		}
	}
	return Builtin{Name: name, FQN: fqn, Package: fqn[:cut]}, true
}
