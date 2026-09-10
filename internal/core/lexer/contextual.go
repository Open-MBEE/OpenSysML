package lexer

import (
	"sort"

	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

// contextualWords are the words the parser reads as syntax in one position and
// as an ordinary name everywhere else (internal/core/parser/notation.go,
// `atVarPrefix`, the `chain` modifier). Keywords() must
// never list them — reserving them would stop models naming features with them
// — so editor surfaces that want them highlighted or offered read them here.
var contextualWords = []string{
	"chain", "choice", "deep", "defer", "done", "history",
	"junction", "shallow",
}

// contextualKerMLWords are contextual in `.kerml` only: `var` is a literal of
// KerML.xtext's BasicFeaturePrefix (`isVariable ?= 'var'`, `:520`) and of no
// SysML production, and the pilot SysML validator rejects `var attribute x;`.
var contextualKerMLWords = []string{"var"}

// ContextualWords returns the contextual words of a language, in name order.
// KindUnknown gets the union, so a surface with no file name still offers them.
func ContextualWords(kind source.Kind) []string {
	out := make([]string, 0, len(contextualWords)+len(contextualKerMLWords))
	out = append(out, contextualWords...)
	if kind != source.KindSysML {
		out = append(out, contextualKerMLWords...)
	}
	sort.Strings(out)
	return out
}
