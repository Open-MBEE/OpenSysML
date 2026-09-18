package main

import (
	"encoding/json"
	"fmt"
	"math"
	"slices"
	"sort"
	"strconv"
	"strings"
)

// tolerance is the relative difference two Real values may have and still be
// the same value: enough for a different summation order, far below any
// difference a model would state. It applies to Real alone; an integral field
// is compared exactly, since a tolerance there would let large counts and ids
// differ and still pass.
const tolerance = 1e-9

// check compares a normalized response against one expectation, returning one
// message per mismatch, in a stable order.
func check(expect *Expect, actual map[string]any) []string {
	var failures []string
	if expect.Response != nil {
		failures = append(failures, match("", expect.Response, actual)...)
	}
	for _, path := range sorted(expect.NonEmpty) {
		value, ok := lookup(actual, path)
		if !ok {
			failures = append(failures, fmt.Sprintf("%s: not set, want a value", path))
			continue
		}
		if empty(value) {
			failures = append(failures, fmt.Sprintf("%s: empty, want a value", path))
		}
	}
	for _, path := range sorted(expect.Absent) {
		if value, ok := lookup(actual, path); ok && !empty(value) {
			failures = append(failures, fmt.Sprintf("%s: set to %s, want it unset", path, render(value)))
		}
	}
	for _, path := range sortedKeys(expect.Contains) {
		want := expect.Contains[path]
		value, ok := lookup(actual, path)
		if !ok {
			failures = append(failures, fmt.Sprintf("%s: not set, want it to contain %q", path, want))
			continue
		}
		text, isText := value.(string)
		if !isText {
			failures = append(failures, fmt.Sprintf("%s: %s is not text, want it to contain %q", path, render(value), want))
			continue
		}
		if !strings.Contains(text, want) {
			failures = append(failures, fmt.Sprintf("%s: %q does not contain %q", path, text, want))
		}
	}
	for _, path := range sortedKeys(expect.ContainsAll) {
		failures = append(failures, containsAll(actual, path, expect.ContainsAll[path])...)
	}
	for _, path := range sortedKeys(expect.Counts) {
		if got, ok := count(actual, path); !ok {
			failures = append(failures, fmt.Sprintf("%s: not a list or map, want %d entries", path, expect.Counts[path]))
		} else if got != expect.Counts[path] {
			failures = append(failures, fmt.Sprintf("%s: %d entries, want %d", path, got, expect.Counts[path]))
		}
	}
	for _, path := range sortedKeys(expect.MinCounts) {
		if got, ok := count(actual, path); !ok {
			failures = append(failures, fmt.Sprintf("%s: not a list or map, want at least %d entries", path, expect.MinCounts[path]))
		} else if got < expect.MinCounts[path] {
			failures = append(failures, fmt.Sprintf("%s: %d entries, want at least %d", path, got, expect.MinCounts[path]))
		}
	}
	return failures
}

// containsAll checks that every wanted string is at path: a substring of the
// text there, or a member of the values there. A path may use "*" to take one
// field of every entry of a list, as in "elements.*.id".
func containsAll(actual map[string]any, path string, wants []string) []string {
	if text, ok := lookup(actual, path); ok {
		if str, isText := text.(string); isText {
			var failures []string
			for _, want := range wants {
				if !strings.Contains(str, want) {
					failures = append(failures, fmt.Sprintf("%s: does not contain %q", path, want))
				}
			}
			return failures
		}
	}
	found, ok := values(actual, path)
	if !ok {
		return []string{fmt.Sprintf("%s: neither text nor a list, want it to contain %s", path, render(wants))}
	}
	var failures []string
	for _, want := range wants {
		if !slices.ContainsFunc(found, func(item any) bool { return item == any(want) }) {
			failures = append(failures, fmt.Sprintf("%s: %s does not contain %q", path, render(found), want))
		}
	}
	return failures
}

// values are the values at a path, expanding a "*" segment over every entry of
// the list or map there, and a list at the end of the path into its members.
func values(tree any, path string) ([]any, bool) {
	segments := strings.Split(path, ".")
	found, ok := walk(tree, segments)
	if !ok {
		return nil, false
	}
	if len(found) == 1 {
		if list, isList := found[0].([]any); isList {
			return list, true
		}
	}
	return found, true
}

// walk collects the values the remaining path segments reach.
func walk(current any, segments []string) ([]any, bool) {
	if len(segments) == 0 {
		return []any{current}, true
	}
	if segments[0] == "*" {
		var entries []any
		switch node := current.(type) {
		case []any:
			entries = node
		case map[string]any:
			for _, key := range sortedKeys(node) {
				entries = append(entries, node[key])
			}
		default:
			return nil, false
		}
		var found []any
		for _, entry := range entries {
			reached, ok := walk(entry, segments[1:])
			if !ok {
				return nil, false
			}
			found = append(found, reached...)
		}
		return found, true
	}
	value, ok := lookup(current, segments[0])
	if !ok {
		return nil, false
	}
	return walk(value, segments[1:])
}

// match compares an expected tree against the response's, reporting the paths
// that differ. Only what the expectation names is compared, so a field added to
// the schema later does not fail a scenario.
func match(path string, want, got any) []string {
	switch expected := want.(type) {
	case map[string]any:
		actual, ok := got.(map[string]any)
		if !ok {
			return []string{fmt.Sprintf("%s: %s, want an object", at(path), render(got))}
		}
		var failures []string
		for _, key := range sortedKeys(expected) {
			child := join(path, key)
			value, present := actual[key]
			if !present {
				// An unset field and a field left at its default are the same
				// thing on the wire, so a default expectation still matches.
				if isDefault(expected[key]) {
					continue
				}
				failures = append(failures, fmt.Sprintf("%s: not set, want %s", child, render(expected[key])))
				continue
			}
			failures = append(failures, match(child, expected[key], value)...)
		}
		return failures
	case []any:
		actual, ok := got.([]any)
		if !ok {
			return []string{fmt.Sprintf("%s: %s, want a list", at(path), render(got))}
		}
		if len(actual) != len(expected) {
			return []string{fmt.Sprintf("%s: %d entries, want %d (%s)", at(path), len(actual), len(expected), render(got))}
		}
		var failures []string
		for i := range expected {
			failures = append(failures, match(join(path, strconv.Itoa(i)), expected[i], actual[i])...)
		}
		return failures
	case json.Number:
		return matchNumber(path, expected.String(), got)
	case float64:
		return matchNumber(path, strconv.FormatFloat(expected, 'g', -1, 64), got)
	default:
		if fmt.Sprint(want) != fmt.Sprint(got) {
			return []string{fmt.Sprintf("%s: %s, want %s", at(path), render(got), render(want))}
		}
		return nil
	}
}

// matchNumber compares an expected number, written as the literal a scenario
// holds, against the value in the response.
func matchNumber(path, want string, got any) []string {
	switch actual := got.(type) {
	case integer:
		return matchIntegral(path, want, strconv.FormatInt(int64(actual), 10))
	case unsigned:
		return matchIntegral(path, want, strconv.FormatUint(uint64(actual), 10))
	case float64:
		number, err := strconv.ParseFloat(want, 64)
		if err != nil || !near(actual, number) {
			return []string{fmt.Sprintf("%s: %v, want %s", at(path), actual, want)}
		}
		return nil
	default:
		return []string{fmt.Sprintf("%s: %s, want the number %s", at(path), render(got), want)}
	}
}

// matchIntegral compares an integral field by its digits, so no difference is
// lost to floating point on the way.
func matchIntegral(path, want, got string) []string {
	digits, ok := integerLiteral(want)
	if !ok || digits != got {
		return []string{fmt.Sprintf("%s: %s, want %s", at(path), got, want)}
	}
	return nil
}

// integerLiteral is an expected number as the digits of a whole number, also
// accepting the "1500.0" and "1.5e3" spellings of one.
func integerLiteral(text string) (string, bool) {
	if _, err := strconv.ParseInt(text, 10, 64); err == nil {
		return text, true
	}
	if _, err := strconv.ParseUint(text, 10, 64); err == nil {
		return text, true
	}
	number, err := strconv.ParseFloat(text, 64)
	if err != nil || number != math.Trunc(number) || math.Abs(number) >= 1<<63 {
		return "", false
	}
	return strconv.FormatInt(int64(number), 10), true
}

// near reports whether two Real values are the same within tolerance.
func near(got, want float64) bool {
	if got == want {
		return true
	}
	scale := math.Max(math.Abs(got), math.Abs(want))
	return math.Abs(got-want) <= tolerance*scale
}

// isDefault reports whether an expected value is what an unset field holds.
func isDefault(want any) bool {
	switch value := want.(type) {
	case bool:
		return !value
	case float64:
		return value == 0
	case integer:
		return value == 0
	case unsigned:
		return value == 0
	case json.Number:
		number, err := strconv.ParseFloat(value.String(), 64)
		return err == nil && number == 0
	case string:
		return value == ""
	case []any:
		return len(value) == 0
	case map[string]any:
		return len(value) == 0
	case nil:
		return true
	default:
		return false
	}
}

// lookup walks a dotted path into a normalized response: field names, map keys
// and list indices, as in "instances.0.feature_values.mass.value.real_value".
func lookup(tree any, path string) (any, bool) {
	current := tree
	for _, segment := range strings.Split(path, ".") {
		switch node := current.(type) {
		case map[string]any:
			value, ok := node[segment]
			if !ok {
				return nil, false
			}
			current = value
		case []any:
			index, err := strconv.Atoi(segment)
			if err != nil || index < 0 || index >= len(node) {
				return nil, false
			}
			current = node[index]
		default:
			return nil, false
		}
	}
	return current, true
}

// count is the number of entries of the list or map at path.
func count(tree any, path string) (int, bool) {
	value, ok := lookup(tree, path)
	if !ok {
		// An empty list or map is an unset field, so nothing there is zero
		// entries rather than a missing path.
		return 0, true
	}
	switch node := value.(type) {
	case []any:
		return len(node), true
	case map[string]any:
		return len(node), true
	default:
		return 0, false
	}
}

// empty reports whether a value is what an unset field holds.
func empty(value any) bool {
	return isDefault(value)
}

// at names the root of a response when a path is empty.
func at(path string) string {
	if path == "" {
		return "response"
	}
	return path
}

func join(path, segment string) string {
	if path == "" {
		return segment
	}
	return path + "." + segment
}

func sorted(values []string) []string {
	out := append([]string(nil), values...)
	sort.Strings(out)
	return out
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// render is a value as a scenario would write it.
func render(value any) string {
	switch typed := value.(type) {
	case string:
		return strconv.Quote(typed)
	case nil:
		return "nothing"
	case map[string]any:
		parts := make([]string, 0, len(typed))
		for _, key := range sortedKeys(typed) {
			parts = append(parts, key+": "+render(typed[key]))
		}
		return "{" + strings.Join(parts, ", ") + "}"
	case []any:
		parts := make([]string, 0, len(typed))
		for _, item := range typed {
			parts = append(parts, render(item))
		}
		return "[" + strings.Join(parts, ", ") + "]"
	default:
		return fmt.Sprint(value)
	}
}
