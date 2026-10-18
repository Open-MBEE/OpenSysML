package llm

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// Structure is the set view of a model's API JSON export that structural
// fidelity compares: typed named elements and the relationships between them.
type Structure struct {
	// Elements are "<@type> <qualifiedName>" for every named element.
	Elements []string `json:"elements"`
	// Relationships are "<@type> <source qualifiedName> -> <target qualifiedName>"
	// for every relationship whose ends are both named elements, except
	// memberships: ownership is already carried by the qualified names.
	Relationships []string `json:"relationships"`
}

// Facts is the union of elements and relationships.
func (s Structure) Facts() map[string]bool {
	out := make(map[string]bool, len(s.Elements)+len(s.Relationships))
	for _, e := range s.Elements {
		out[e] = true
	}
	for _, r := range s.Relationships {
		out[r] = true
	}
	return out
}

type apiElement struct {
	Type          string   `json:"@type"`
	ID            string   `json:"@id"`
	QualifiedName string   `json:"qualifiedName"`
	Source        []apiRef `json:"source"`
	Target        []apiRef `json:"target"`
}

type apiRef struct {
	ID string `json:"@id"`
}

// ExtractStructure reads the API JSON export into its structural fact sets.
// Anonymous elements (qualified names with an `@n` segment) carry no name a
// task could ask for, so they and the relationships touching them are dropped.
func ExtractStructure(apiJSON []byte) (Structure, error) {
	var elems []apiElement
	if err := json.Unmarshal(apiJSON, &elems); err != nil {
		return Structure{}, fmt.Errorf("api-json: %w", err)
	}
	names := map[string]string{}
	for _, e := range elems {
		if e.QualifiedName != "" && !anonymous(e.QualifiedName) {
			names[e.ID] = e.QualifiedName
		}
	}
	var s Structure
	for _, e := range elems {
		if qn, ok := names[e.ID]; ok {
			s.Elements = append(s.Elements, e.Type+" "+qn)
		}
		if len(e.Source) == 0 || len(e.Target) == 0 || strings.HasSuffix(e.Type, "Membership") {
			continue
		}
		for _, src := range e.Source {
			for _, tgt := range e.Target {
				from, okFrom := names[src.ID]
				to, okTo := names[tgt.ID]
				if okFrom && okTo {
					s.Relationships = append(s.Relationships, e.Type+" "+from+" -> "+to)
				}
			}
		}
	}
	s.Elements = dedupe(s.Elements)
	s.Relationships = dedupe(s.Relationships)
	return s, nil
}

func anonymous(qn string) bool {
	for _, seg := range strings.Split(qn, "::") {
		if strings.HasPrefix(seg, "@") {
			return true
		}
	}
	return false
}

func dedupe(xs []string) []string {
	sort.Strings(xs)
	out := xs[:0]
	for i, x := range xs {
		if i == 0 || x != xs[i-1] {
			out = append(out, x)
		}
	}
	return out
}

// Jaccard is |A ∩ B| / |A ∪ B| over the fact sets; two empty sets score 1.
func Jaccard(a, b Structure) float64 {
	fa, fb := a.Facts(), b.Facts()
	union := len(fa)
	inter := 0
	for f := range fb {
		if fa[f] {
			inter++
		} else {
			union++
		}
	}
	if union == 0 {
		return 1
	}
	return float64(inter) / float64(union)
}
