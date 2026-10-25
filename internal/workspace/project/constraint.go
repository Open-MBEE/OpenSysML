package project

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// ErrBadConstraint is the error a version constraint that does not follow
// the grammar wraps.
var ErrBadConstraint = errors.New("malformed version constraint")

// Constraint is a parsed versionConstraint. The grammar, pinned by the tests:
//
//	constraint := "" | clause ( ("," | " ") clause )*
//	clause     := [ "=" | "==" ] version          exactly that version
//	            | "^" version                     caret: same major (same minor below 1.0, same patch below 0.1)
//	            | "~" version                     tilde: same major.minor
//	            | ( ">=" | ">" | "<=" | "<" ) version
//	version    := [ "v" ] major [ "." minor [ "." patch ] ] [ "-" pre ]
//
// An empty constraint matches every version; a missing minor or patch is 0.
// Versions compare as semantic versions, pre-releases below the release.
type Constraint struct {
	text    string
	clauses []clause
}

type clause struct {
	op      string
	version string
	upper   string // exclusive upper bound of ^ and ~
}

// ParseConstraint reads a constraint; an empty one matches every version.
func ParseConstraint(text string) (Constraint, error) {
	c := Constraint{text: strings.TrimSpace(text)}
	for _, field := range strings.FieldsFunc(c.text, func(r rune) bool { return r == ',' || r == ' ' || r == '\t' }) {
		cl, err := parseClause(field)
		if err != nil {
			return Constraint{}, fmt.Errorf("%q: %w: %w", text, ErrBadConstraint, err)
		}
		c.clauses = append(c.clauses, cl)
	}
	return c, nil
}

func parseClause(field string) (clause, error) {
	op := ""
	for _, candidate := range []string{"==", ">=", "<=", "=", "^", "~", ">", "<"} {
		if strings.HasPrefix(field, candidate) {
			op = candidate
			break
		}
	}
	rest := strings.TrimSpace(strings.TrimPrefix(field, op))
	if op == "" || op == "==" {
		op = "="
	}
	version, err := canonicalVersion(rest)
	if err != nil {
		return clause{}, err
	}
	cl := clause{op: op, version: version}
	core, _ := splitVersion(version)
	major, _ := strconv.Atoi(core[0])
	minor, _ := strconv.Atoi(core[1])
	patch, _ := strconv.Atoi(core[2])
	switch op {
	case "^":
		switch {
		case major > 0:
			cl.upper = fmt.Sprintf("%d.0.0-0", major+1)
		case minor > 0:
			cl.upper = fmt.Sprintf("0.%d.0-0", minor+1)
		default:
			cl.upper = fmt.Sprintf("0.0.%d-0", patch+1)
		}
	case "~":
		cl.upper = fmt.Sprintf("%d.%d.0-0", major, minor+1)
	}
	return cl, nil
}

// canonicalVersion checks a version is numeric major[.minor[.patch]] with an
// optional pre-release and returns it with three numeric parts.
func canonicalVersion(v string) (string, error) {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	if v == "" {
		return "", errors.New("a version is missing")
	}
	core, pre := splitVersion(v)
	if len(core) == 0 || len(core) > 3 {
		return "", fmt.Errorf("%q is not a semantic version", v)
	}
	for _, part := range core {
		if part == "" || strings.Trim(part, "0123456789") != "" {
			return "", fmt.Errorf("%q is not a semantic version", v)
		}
	}
	for len(core) < 3 {
		core = append(core, "0")
	}
	out := strings.Join(core, ".")
	if len(pre) > 0 {
		out += "-" + strings.Join(pre, ".")
	}
	return out, nil
}

// String is the constraint as written.
func (c Constraint) String() string { return c.text }

// Any reports whether the constraint admits every version.
func (c Constraint) Any() bool { return len(c.clauses) == 0 }

// Exact returns the single version an exact constraint names, else "".
func (c Constraint) Exact() string {
	if len(c.clauses) == 1 && c.clauses[0].op == "=" {
		return c.clauses[0].version
	}
	return ""
}

// Matches reports whether a version satisfies every clause.
func (c Constraint) Matches(version string) bool {
	v, err := canonicalVersion(version)
	if err != nil {
		return false
	}
	for _, cl := range c.clauses {
		cmp := compareVersions(v, cl.version)
		switch cl.op {
		case "=":
			if cmp != 0 {
				return false
			}
		case ">=":
			if cmp < 0 {
				return false
			}
		case ">":
			if cmp <= 0 {
				return false
			}
		case "<=":
			if cmp > 0 {
				return false
			}
		case "<":
			if cmp >= 0 {
				return false
			}
		case "^", "~":
			if cmp < 0 || compareVersions(v, cl.upper) >= 0 {
				return false
			}
		}
	}
	return true
}

// Latest returns the highest version matching the constraint, "" when none
// does. Ties between spellings of one version (v1.0 and 1.0.0) go to the
// lexically smaller spelling, so the answer is deterministic.
func (c Constraint) Latest(versions []string) string {
	sorted := append([]string(nil), versions...)
	sort.Strings(sorted)
	best := ""
	for _, v := range sorted {
		if !c.Matches(v) {
			continue
		}
		if best == "" || newerVersion(v, best) {
			best = v
		}
	}
	return best
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

// exactVersion returns the version an exact constraint names, "" otherwise.
func exactVersion(constraint string) string {
	c, err := ParseConstraint(constraint)
	if err != nil {
		return ""
	}
	return c.Exact()
}
