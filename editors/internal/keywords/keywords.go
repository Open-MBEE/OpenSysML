// Package keywords groups the lexer's reserved words and contextual words into
// the categories editor highlighting colours them by. Every editor surface reads
// its keyword lists from here, so they cannot drift from the lexer or from each
// other.
package keywords

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/lexer"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// Category is the kind of syntax a keyword group introduces.
type Category string

// The categories, in the order editors apply them.
const (
	Constant     Category = "constant"
	Declaration  Category = "declaration"
	Control      Category = "control"
	Modifier     Category = "modifier"
	Relationship Category = "relationship"
	Operator     Category = "operator"
	// Other holds every reserved word no group claims, so a new keyword is
	// highlighted without an editor change. It is absent while every keyword is
	// grouped.
	Other Category = "other"
)

// Group is the reserved words of one category.
type Group struct {
	Category Category
	Words    []string
}

// Keyword groups. Every keyword the lexer knows is highlighted; grouping only
// refines the colour it gets, and a group naming a keyword the lexer dropped is
// a generation error.
var groups = []Group{
	{
		Category: Constant,
		Words:    []string{"false", "null", "true"},
	},
	{
		Category: Declaration,
		Words: []string{
			"action", "allocation", "analysis", "assoc", "attribute", "behavior",
			"binding", "calc", "case", "class", "classifier", "comment", "concern",
			"connection", "connector", "constraint", "datatype", "def", "dependency",
			"doc", "enum", "event", "expr", "feature", "flow", "function", "interaction",
			"interface", "item", "language", "metaclass", "metadata", "multiplicity",
			"namespace", "occurrence", "package", "part", "port", "predicate",
			"rendering", "rep", "requirement", "state", "step", "struct", "type",
			"verification", "view", "viewpoint",
		},
	},
	{
		Category: Control,
		Words: []string{
			// The words our state notation adds (`initial`, `done`) are not here:
			// the lexer does not reserve them (`done` is a contextual word).
			"accept", "after", "assert", "assign", "at", "decide",
			"do", "else", "entry", "exhibit", "exit", "first",
			"for", "fork", "if", "include", "join",
			"loop", "merge", "parallel", "perform", "render", "require",
			"return", "satisfy", "send", "succession", "terminate", "then",
			"to", "transition", "until", "via", "when", "while",
		},
	},
	{
		Category: Modifier,
		Words: []string{
			"abstract", "all", "composite", "const", "constant", "default", "derived",
			"end", "in", "individual", "inout", "library", "member", "new",
			"nonunique", "ordered", "out", "portion", "private", "protected", "public",
			"ref", "snapshot", "standard", "timeslice", "variant", "variation",
		},
	},
	{
		Category: Relationship,
		Words: []string{
			"alias", "conjugate", "conjugates", "conjugation", "crosses", "differences",
			"disjoining", "disjoint", "featured", "featuring", "import", "inverse",
			"inverting", "intersects", "redefines", "redefinition", "references",
			"specialization", "specializes", "subclassifier", "subset", "subsets",
			"subtype", "typed", "typing", "unions",
		},
	},
	{
		Category: Operator,
		Words: []string{
			"and", "as", "chains", "defined", "hastype", "implies", "istype", "meta",
			"not", "or", "xor",
		},
	},
}

// Groups returns the keyword groups, each word in the order listed, followed by
// an Other group when some reserved word is in none of them. It fails when a
// group names a word the lexer does not reserve or two groups share a word.
func Groups() ([]Group, error) {
	return validate(groups, source.Keywords())
}

func validate(in []Group, reserved []string) ([]Group, error) {
	known := map[string]bool{}
	for _, kw := range reserved {
		known[kw] = true
	}
	grouped := map[string]bool{}
	out := make([]Group, 0, len(in)+1)
	for _, g := range in {
		for _, kw := range g.Words {
			if !known[kw] {
				return nil, fmt.Errorf("keyword group %q names %q, which the lexer no longer knows", g.Category, kw)
			}
			if grouped[kw] {
				return nil, fmt.Errorf("keyword %q appears in more than one keyword group", kw)
			}
			grouped[kw] = true
		}
		out = append(out, Group{Category: g.Category, Words: append([]string(nil), g.Words...)})
	}
	var rest []string
	for _, kw := range reserved {
		if !grouped[kw] {
			rest = append(rest, kw)
		}
	}
	if len(rest) > 0 {
		out = append(out, Group{Category: Other, Words: rest})
	}
	return out, nil
}

// introducers are the keywords a declared name follows directly, e.g. `part def
// Vehicle`. `library` is not one: it only ever precedes `package`, which is.
var introducers = []string{"alias", "def", "namespace", "package"}

// Introducers returns the reserved words that are directly followed by the name
// a declaration introduces. It fails when one is no longer reserved.
func Introducers() ([]string, error) {
	return checkReserved(introducers, source.Keywords())
}

func checkReserved(words, reserved []string) ([]string, error) {
	known := map[string]bool{}
	for _, kw := range reserved {
		known[kw] = true
	}
	for _, w := range words {
		if !known[w] {
			return nil, fmt.Errorf("declaration introducer %q is not a reserved keyword", w)
		}
	}
	return append([]string(nil), words...), nil
}

// Contextual returns the words the parser reads as syntax positionally in a
// language without the lexer reserving them, in name order. It fails when one is
// also reserved: the two lists exist precisely to be disjoint, and that is what
// stops a contextual word leaking into source.Keywords().
func Contextual(kind source.Kind) ([]string, error) {
	words := lexer.ContextualWords(kind)
	if err := checkUnreserved(words, source.Keywords()); err != nil {
		return nil, err
	}
	return words, nil
}

func checkUnreserved(contextual, reserved []string) error {
	known := map[string]bool{}
	for _, kw := range reserved {
		known[kw] = true
	}
	for _, w := range contextual {
		if known[w] {
			return fmt.Errorf("contextual word %q is a reserved keyword; it must be in one list or the other", w)
		}
	}
	return nil
}
