package project

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// errBadConstraint is what a versionConstraint outside the grammar wraps.
var errBadConstraint = errors.New("malformed version constraint")

// versionRange is a parsed versionConstraint in the npm-style semver range grammar: `||`-separated
// sets of space/comma-separated comparators (`=`/bare, `<`, `<=`, `>`, `>=`, `^`, `~`, `x`/`*` wildcards, `a - b`).
type versionRange struct {
	text string
	sets [][]comparator // any set whose every comparator holds satisfies the range
}

type comparator struct {
	op      string // "=", "<", "<=", ">", ">="
	version string // canonical major.minor.patch[-pre]
}

// parseVersionRange reads a constraint; an empty one admits every version. Each
// hyphen range, caret, tilde and wildcard is lowered to its comparator pair.
func parseVersionRange(text string) (versionRange, error) {
	r := versionRange{text: strings.TrimSpace(text)}
	if r.text == "" {
		return r, nil
	}
	for _, set := range strings.Split(r.text, "||") {
		cs, err := parseComparatorSet(set)
		if err != nil {
			return versionRange{}, fmt.Errorf("%q: %w: %w", text, errBadConstraint, err)
		}
		r.sets = append(r.sets, cs)
	}
	return r, nil
}

// parseComparatorSet reads one `||` alternative: comparators separated by
// spaces or commas, `a - b` read as a hyphen range before anything else.
func parseComparatorSet(set string) ([]comparator, error) {
	fields := strings.FieldsFunc(set, func(r rune) bool { return r == ',' || r == ' ' || r == '\t' })
	var out []comparator
	for i := 0; i < len(fields); i++ {
		if i+2 < len(fields) && fields[i+1] == "-" {
			low, err := parseWildcardVersion(fields[i])
			if err != nil {
				return nil, err
			}
			high, err := parseWildcardVersion(fields[i+2])
			if err != nil {
				return nil, err
			}
			out = append(out, comparator{">=", low.version})
			if high.upper != "" {
				out = append(out, comparator{"<", high.upper})
			} else {
				out = append(out, comparator{"<=", high.version})
			}
			i += 2
			continue
		}
		cs, err := parseComparator(fields[i])
		if err != nil {
			return nil, err
		}
		out = append(out, cs...)
	}
	if len(fields) == 0 {
		return nil, errors.New("an empty alternative")
	}
	return out, nil
}

// parseComparator lowers one field to comparators: a bare or wildcard version,
// `=`/`==`, `<`, `<=`, `>`, `>=`, `^` or `~` before a version.
func parseComparator(field string) ([]comparator, error) {
	op := ""
	for _, candidate := range []string{"==", ">=", "<=", "=", "^", "~", ">", "<"} {
		if strings.HasPrefix(field, candidate) {
			op = candidate
			break
		}
	}
	w, err := parseWildcardVersion(strings.TrimPrefix(field, op))
	if err != nil {
		return nil, err
	}
	if w.wild == 0 && (op == "" || op == "=" || op == "==" || op == "^" || op == "~") {
		return nil, nil // *, ^* and ~* alone admit any version
	}
	major, minor, patch := w.parts[0], w.parts[1], w.parts[2]
	switch op {
	case "", "=", "==":
		if w.upper != "" { // 1.x, 1.2.x
			return []comparator{{">=", w.version}, {"<", w.upper}}, nil
		}
		return []comparator{{"=", w.version}}, nil
	case "^":
		var upper string
		switch {
		case major > 0 || w.wild == 1:
			upper = fmt.Sprintf("%d.0.0-0", major+1)
		case minor > 0 || w.wild == 2:
			upper = fmt.Sprintf("0.%d.0-0", minor+1)
		default:
			upper = fmt.Sprintf("0.0.%d-0", patch+1)
		}
		return []comparator{{">=", w.version}, {"<", upper}}, nil
	case "~":
		if w.wild == 1 {
			return []comparator{{">=", w.version}, {"<", fmt.Sprintf("%d.0.0-0", major+1)}}, nil
		}
		return []comparator{{">=", w.version}, {"<", fmt.Sprintf("%d.%d.0-0", major, minor+1)}}, nil
	case ">":
		if w.wild == 0 {
			return nil, fmt.Errorf("%q compares against a wildcard", field)
		}
		if w.upper != "" { // >1.x means >=2.0.0
			return []comparator{{">=", w.upper}}, nil
		}
		return []comparator{{">", w.version}}, nil
	case "<=":
		if w.upper != "" {
			return []comparator{{"<", w.upper}}, nil
		}
		return []comparator{{"<=", w.version}}, nil
	default: // ">=", "<"
		return []comparator{{op, w.version}}, nil
	}
}

// wildcardVersion is a version as a comparator spells it: canonical, with the
// count of parts given before a wildcard or omission (wild: 0 none given, 1
// major only, 2 major.minor; 3 means complete) and the exclusive upper bound
// of what the wildcard spans.
type wildcardVersion struct {
	version string
	parts   [3]int
	wild    int
	upper   string
}

func parseWildcardVersion(text string) (wildcardVersion, error) {
	v := strings.TrimPrefix(strings.TrimSpace(text), "v")
	if v == "" {
		return wildcardVersion{}, errors.New("a version is missing")
	}
	base, build, hasBuild := strings.Cut(v, "+")
	if hasBuild && !validIdentifiers(build) {
		return wildcardVersion{}, fmt.Errorf("%q has malformed build metadata", v)
	}
	core, pre := splitVersion(base)
	if len(core) > 3 || (strings.Contains(base, "-") && len(pre) == 0) {
		return wildcardVersion{}, fmt.Errorf("%q is not a semantic version", v)
	}
	w := wildcardVersion{wild: 3}
	for i := 0; i < 3; i++ {
		if i >= len(core) || core[i] == "x" || core[i] == "X" || core[i] == "*" {
			if w.wild == 3 {
				w.wild = i
			}
			continue
		}
		if w.wild != 3 {
			return wildcardVersion{}, fmt.Errorf("%q names a part after a wildcard", v)
		}
		n, err := strconv.Atoi(core[i])
		if err != nil || core[i] == "" {
			return wildcardVersion{}, fmt.Errorf("%q is not a semantic version", v)
		}
		w.parts[i] = n
	}
	w.version = fmt.Sprintf("%d.%d.%d", w.parts[0], w.parts[1], w.parts[2])
	if w.wild == 3 && len(pre) > 0 {
		w.version += "-" + strings.Join(pre, ".")
	}
	switch w.wild {
	case 1:
		w.upper = fmt.Sprintf("%d.0.0-0", w.parts[0]+1)
	case 2:
		w.upper = fmt.Sprintf("%d.%d.0-0", w.parts[0], w.parts[1]+1)
	}
	if len(pre) > 0 && !validIdentifiers(strings.Join(pre, ".")) {
		return wildcardVersion{}, fmt.Errorf("%q has a malformed pre-release", v)
	}
	return w, nil
}

// validIdentifiers reports whether ids is dot-separated, non-empty [0-9A-Za-z-] identifiers.
func validIdentifiers(ids string) bool {
	for _, id := range strings.Split(ids, ".") {
		if id == "" || strings.Trim(id, "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ-") != "" {
			return false
		}
	}
	return true
}

// any reports whether the range admits every version.
func (r versionRange) any() bool { return len(r.sets) == 0 }

// matches reports whether version satisfies the range: some alternative whose every comparator
// holds, versions ordered as semver 2.0.0 orders them; a pre-release only where the alternative names its release.
func (r versionRange) matches(version string) bool {
	if r.any() {
		return true
	}
	v := strings.TrimPrefix(strings.TrimSpace(version), "v")
	core, pre := splitVersion(v)
	for _, set := range r.sets {
		if len(pre) > 0 && !admitsPreRelease(set, core) {
			continue
		}
		ok := true
		for _, c := range set {
			cmp := compareVersions(v, c.version)
			switch c.op {
			case "=":
				ok = cmp == 0
			case "<":
				ok = cmp < 0
			case "<=":
				ok = cmp <= 0
			case ">":
				ok = cmp > 0
			case ">=":
				ok = cmp >= 0
			}
			if !ok {
				break
			}
		}
		if ok {
			return true
		}
	}
	return false
}

// admitsPreRelease reports whether some comparator of the set is itself a pre-release of the
// release core names, which is when npm-style ranges let a pre-release in.
func admitsPreRelease(set []comparator, core []string) bool {
	for _, c := range set {
		if cc, cpre := splitVersion(c.version); len(cpre) > 0 && strings.Join(cc, ".") == strings.Join(core, ".") {
			return true
		}
	}
	return false
}

// compareVersions orders two versions: -1, 0 or 1 as a is below, at or above b.
func compareVersions(a, b string) int {
	switch {
	case newerVersion(a, b):
		return 1
	case newerVersion(b, a):
		return -1
	}
	return 0
}
