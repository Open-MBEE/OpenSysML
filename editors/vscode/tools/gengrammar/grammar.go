package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/lexer"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// Grammar describes one generated TextMate grammar file.
type Grammar struct {
	File      string
	Name      string
	ScopeName string
	FileTypes []string
	Kind      source.Kind
}

// Grammars returns the grammars shipped by the extension. SysML v2 and KerML
// share a keyword set, so they share every pattern but their scope name and the
// contextual words their own grammar reads as syntax.
func Grammars() []Grammar {
	return []Grammar{
		{File: "sysml.tmLanguage.json", Name: "SysML v2", ScopeName: "source.sysml", FileTypes: []string{"sysml"}, Kind: source.KindSysML},
		{File: "kerml.tmLanguage.json", Name: "KerML", ScopeName: "source.kerml", FileTypes: []string{"kerml"}, Kind: source.KindKerML},
	}
}

// Keyword groups, each mapped to a TextMate scope. Every keyword the lexer
// knows is highlighted; grouping only refines the colour it gets, and a group
// naming a keyword the lexer dropped is a generation error.
var keywordGroups = []struct {
	rule     string
	scope    string
	keywords []string
}{
	{
		rule:  "keywords-declaration",
		scope: "keyword.declaration",
		keywords: []string{
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
		rule:  "keywords-control",
		scope: "keyword.control",
		keywords: []string{
			// The words our state notation adds (`initial`, `done`) are not
			// here: the lexer does not reserve them, so they are in "keywords-contextual".
			"accept", "after", "assert", "assign", "at", "decide",
			"do", "else", "entry", "exhibit", "exit", "first",
			"for", "fork", "if", "include", "join",
			"loop", "merge", "parallel", "perform", "render", "require",
			"return", "satisfy", "send", "succession", "terminate", "then",
			"to", "transition", "until", "via", "when", "while",
		},
	},
	{
		rule:  "keywords-modifier",
		scope: "storage.modifier",
		keywords: []string{
			"abstract", "all", "composite", "const", "constant", "default", "derived",
			"end", "in", "individual", "inout", "library", "member", "new",
			"nonunique", "ordered", "out", "portion", "private", "protected", "public",
			"ref", "snapshot", "standard", "timeslice", "variant", "variation",
		},
	},
	{
		rule:  "keywords-relationship",
		scope: "keyword.other.relationship",
		keywords: []string{
			"alias", "conjugate", "conjugates", "conjugation", "crosses", "differences",
			"disjoining", "disjoint", "featured", "featuring", "import", "inverse",
			"inverting", "intersects", "redefines", "redefinition", "references",
			"specialization", "specializes", "subclassifier", "subset", "subsets",
			"subtype", "typed", "typing", "unions",
		},
	},
	{
		rule:  "keywords-operator",
		scope: "keyword.operator.word",
		keywords: []string{
			"and", "as", "chains", "defined", "hastype", "implies", "istype", "meta",
			"not", "or", "xor",
		},
	},
	{
		rule:     "constants",
		scope:    "constant.language",
		keywords: []string{"false", "null", "true"},
	},
}

// ruleOrder is the order the grammar applies its rules in: comments and quoted
// text first so their contents are never read as code, keywords before the
// catch-all identifier and operator rules.
var ruleOrder = []string{
	"comments",
	"strings",
	"quoted-names",
	"declaration-names",
	"type-references",
	"constants",
	"keywords-declaration",
	"keywords-control",
	"keywords-modifier",
	"keywords-relationship",
	"keywords-operator",
	"keywords-other",
	"keywords-contextual",
	"qualified-names",
	"numbers",
	"operators",
}

// Render returns the JSON text of a grammar file, ending in a newline.
func Render(g Grammar) ([]byte, error) {
	repo, err := repository(g.Kind)
	if err != nil {
		return nil, err
	}
	return render(g, repo)
}

func render(g Grammar, repo map[string]pattern) ([]byte, error) {
	// A rule the repository leaves out is skipped — "keywords-other" is empty
	// once every keyword is grouped. The count check still catches a rule the
	// order forgets or misspells.
	includes := make([]pattern, 0, len(ruleOrder))
	for _, name := range ruleOrder {
		if _, ok := repo[name]; ok {
			includes = append(includes, pattern{Include: "#" + name})
		}
	}
	if len(includes) != len(repo) {
		return nil, fmt.Errorf("rule order covers %d of %d repository rules", len(includes), len(repo))
	}

	scoped := make(map[string]pattern, len(repo))
	for name, p := range repo {
		scoped[name] = qualify(p, g.ScopeName)
	}

	doc := grammarFile{
		Schema:     "https://raw.githubusercontent.com/martinring/tmlanguage/master/tmlanguage.json",
		Comment:    "Generated by editors/vscode/tools/gengrammar from internal/core/source.Keywords() and lexer.ContextualWords(); do not edit by hand.",
		Name:       g.Name,
		ScopeName:  g.ScopeName,
		FileTypes:  g.FileTypes,
		Patterns:   includes,
		Repository: scoped,
	}

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(doc); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

type grammarFile struct {
	Schema     string             `json:"$schema"`
	Comment    string             `json:"comment"`
	Name       string             `json:"name"`
	ScopeName  string             `json:"scopeName"`
	FileTypes  []string           `json:"fileTypes"`
	Patterns   []pattern          `json:"patterns"`
	Repository map[string]pattern `json:"repository"`
}

type pattern struct {
	Include       string             `json:"include,omitempty"`
	Name          string             `json:"name,omitempty"`
	Match         string             `json:"match,omitempty"`
	Begin         string             `json:"begin,omitempty"`
	End           string             `json:"end,omitempty"`
	Captures      map[string]pattern `json:"captures,omitempty"`
	BeginCaptures map[string]pattern `json:"beginCaptures,omitempty"`
	Patterns      []pattern          `json:"patterns,omitempty"`
}

// qualify appends the grammar's scope name to every scope in a pattern tree, so
// `keyword.control` becomes `keyword.control.source.sysml`.
func qualify(p pattern, scopeName string) pattern {
	if p.Name != "" && !strings.HasSuffix(p.Name, scopeName) {
		p.Name += "." + scopeName
	}
	p.Captures = qualifyMap(p.Captures, scopeName)
	p.BeginCaptures = qualifyMap(p.BeginCaptures, scopeName)
	if len(p.Patterns) > 0 {
		out := make([]pattern, 0, len(p.Patterns))
		for _, sub := range p.Patterns {
			out = append(out, qualify(sub, scopeName))
		}
		p.Patterns = out
	}
	return p
}

func qualifyMap(m map[string]pattern, scopeName string) map[string]pattern {
	if len(m) == 0 {
		return nil
	}
	out := make(map[string]pattern, len(m))
	for k, v := range m {
		out[k] = qualify(v, scopeName)
	}
	return out
}

const identifier = `[A-Za-z_][A-Za-z0-9_]*`

func repository(kind source.Kind) (map[string]pattern, error) {
	repo := map[string]pattern{
		"comments": {
			Patterns: []pattern{
				{Name: "comment.line.double-slash", Match: `//.*$`},
				{Name: "comment.block", Begin: `/\*`, End: `\*/`},
			},
		},
		"strings": {
			Name:  "string.quoted.double",
			Begin: `"`,
			End:   `"`,
			Patterns: []pattern{
				{Name: "constant.character.escape", Match: `\\.`},
			},
		},
		// A quoted name, e.g. `part 'left wheel';`.
		"quoted-names": {
			Name:  "entity.name.quoted",
			Begin: `'`,
			End:   `'`,
			Patterns: []pattern{
				{Name: "constant.character.escape", Match: `\\.`},
			},
		},
		"numbers": {
			Name:  "constant.numeric",
			Match: `\b\d+(\.\d+)?([eE][+-]?\d+)?\b`,
		},
		// The name a declaration introduces, e.g. `part def Vehicle`.
		"declaration-names": {
			Match: `\b(def|package|namespace|alias|library)\s+(` + identifier + `|'[^']*')`,
			Captures: map[string]pattern{
				"1": {Name: "keyword.declaration"},
				"2": {Name: "entity.name.type"},
			},
		},
		// A namespace segment of a qualified name, e.g. `ScalarValues::Real`.
		"qualified-names": {
			Name:  "entity.name.namespace",
			Match: `\b` + identifier + `(?=\s*::)`,
		},
		// The type a feature is typed by or specializes.
		"type-references": {
			Match: `(:>>|::>|:>|:)\s*((?:` + identifier + `::)*(?:` + identifier + `|'[^']*'))`,
			Captures: map[string]pattern{
				"1": {Name: "keyword.operator"},
				"2": {Name: "entity.name.type"},
			},
		},
		"operators": {
			Name:  "keyword.operator",
			Match: `:>>|::>|:>|=>|->|\.\.|\*\*|[=+\-*/<>!&|^~?@]`,
		},
	}

	known := map[string]bool{}
	for _, kw := range source.Keywords() {
		known[kw] = true
	}
	grouped := map[string]bool{}
	for _, group := range keywordGroups {
		for _, kw := range group.keywords {
			if !known[kw] {
				return nil, fmt.Errorf("grammar group %q names %q, which the lexer no longer knows", group.rule, kw)
			}
			if grouped[kw] {
				return nil, fmt.Errorf("keyword %q appears in more than one grammar group", kw)
			}
			grouped[kw] = true
		}
		repo[group.rule] = pattern{Name: group.scope, Match: alternation(group.keywords)}
	}

	// Anything the lexer knows and no group claims still highlights as a
	// keyword, so a new keyword needs no grammar change.
	var rest []string
	for _, kw := range source.Keywords() {
		if !grouped[kw] {
			rest = append(rest, kw)
		}
	}
	if len(rest) > 0 {
		repo["keywords-other"] = pattern{Name: "keyword.other", Match: alternation(rest)}
	}

	// The words the parser reads as syntax positionally without the lexer
	// reserving them. Last of the keyword rules, so a word an earlier rule
	// claims keeps its own scope, and a generation error if one is reserved:
	// that is what stops the list leaking into Keywords().
	contextual := lexer.ContextualWords(kind)
	if err := checkUnreserved(contextual, known); err != nil {
		return nil, err
	}
	if len(contextual) > 0 {
		repo["keywords-contextual"] = pattern{Name: "keyword.other.contextual", Match: alternation(contextual)}
	}

	return repo, nil
}

// checkUnreserved fails generation when a contextual word is also a reserved
// keyword: the two lists exist precisely to be disjoint.
func checkUnreserved(contextual []string, known map[string]bool) error {
	for _, w := range contextual {
		if known[w] {
			return fmt.Errorf("contextual word %q is a reserved keyword; it must be in one list or the other", w)
		}
	}
	return nil
}

// alternation builds a word-bounded regex matching any of the keywords, longest
// first so that `subsets` is not matched as `subset`.
func alternation(keywords []string) string {
	sorted := make([]string, len(keywords))
	copy(sorted, keywords)
	sort.Slice(sorted, func(i, j int) bool {
		if len(sorted[i]) != len(sorted[j]) {
			return len(sorted[i]) > len(sorted[j])
		}
		return sorted[i] < sorted[j]
	})
	return `\b(?:` + strings.Join(sorted, "|") + `)\b`
}
