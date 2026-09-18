package xpect

import (
	"regexp"
	"strings"
)

// A declared diagnostic and ours can state the same rule about the same element
// in different words. Such a row is agreement in substance, so it is scored as
// agreement — but only on rule and element identity, never on severity and
// offset alone: a diagnostic of another rule landing on the declared token is a
// disagreement wearing agreement's clothes.

// wordingRule pairs one declared rule with one of ours. Both patterns capture
// the element the diagnostic is about, so a row joins the class only when both
// the rule and the element match.
type wordingRule struct {
	class    string
	declared *regexp.Regexp
	ours     *regexp.Regexp
	// suffix admits our shorter spelling of the element: where the pilot names
	// the whole qualified name, we name the segment that failed to resolve.
	suffix bool
}

// declaredUnresolved is the pilot's linking failure, whose element is the
// qualified name it could not resolve and whose kind word is the metaclass it
// expected there.
var declaredUnresolved = regexp.MustCompile(`^Couldn't resolve reference to \w+ '([^']+)'\.?$`)

var wordingRules = []wordingRule{{
	class:    "unresolved-reference",
	declared: declaredUnresolved,
	ours:     regexp.MustCompile(`^unresolved reference: ([^\s(]+)`),
}, {
	class:    "unresolved-reference",
	declared: declaredUnresolved,
	ours:     regexp.MustCompile(`^unresolved member: ([^\s(]+)`),
	suffix:   true,
}}

// wordingOnly reports the rule both messages state, if they state one rule
// about one element in different words.
func wordingOnly(declared, ours string) (string, bool) {
	for _, rule := range wordingRules {
		d := rule.declared.FindStringSubmatch(normalizeMessage(declared))
		o := rule.ours.FindStringSubmatch(normalizeMessage(ours))
		if d == nil || o == nil {
			continue
		}
		if sameElement(d[1], o[1], rule.suffix) {
			return rule.class, true
		}
	}
	return "", false
}

func normalizeMessage(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// sameElement compares a declared element name with ours in the pilot's
// notation. With suffix, our name may spell the declared name's last segment.
func sameElement(declared, ours string, suffix bool) bool {
	d := dotted(strings.Trim(declared, "'"))
	o := dotted(strings.Trim(ours, "'"))
	if d == o {
		return true
	}
	if !suffix {
		return false
	}
	segs := strings.Split(d, ".")
	return o == segs[len(segs)-1]
}
