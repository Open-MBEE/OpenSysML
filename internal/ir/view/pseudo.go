package view

import (
	"sort"
	"strings"
)

// PseudoViewPrefix marks a rendering of exposed elements rather than a declared
// view.
const PseudoViewPrefix = "#"

// PseudoViewKind reports the supported rendering kind named by name.
func PseudoViewKind(name string) (Kind, bool) {
	kind, ok := pseudoViewKinds()[name]
	return kind, ok
}

// ParsePseudoView parses #<kind> or #<kind>:<target>.
func ParsePseudoView(spec string) (Kind, string, bool) {
	if !strings.HasPrefix(spec, PseudoViewPrefix) {
		return "", "", false
	}
	name, target, _ := strings.Cut(strings.TrimPrefix(spec, PseudoViewPrefix), ":")
	kind, ok := PseudoViewKind(name)
	return kind, target, ok
}

// PseudoViewSpecs names the supported pseudo-views in sorted order.
func PseudoViewSpecs() []string {
	kinds := pseudoViewKinds()
	out := make([]string, 0, len(kinds))
	for name := range kinds {
		out = append(out, PseudoViewPrefix+name)
	}
	sort.Strings(out)
	return out
}

func pseudoViewKinds() map[string]Kind {
	out := map[string]Kind{}
	add := func(kinds map[string]Kind) {
		for _, kind := range kinds {
			if kind.Supported() {
				out[string(kind)] = kind
			}
		}
	}
	add(standardRenderings)
	add(standardViewDefinitions)
	return out
}
