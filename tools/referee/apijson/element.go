package apijson

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// unkeyed marks a reference to an element no stable key addresses (an ownerless
// unnamed element the pilot synthesizes, or an id its export never lists).
const unkeyed = "?"

// ref is a reference to another element, compared by that element's key.
type ref struct{ Key string }

// element is one exported element with its properties normalized: references
// are refs, collections are []any, scalars are JSON scalars.
type element struct {
	Key     string
	Type    string
	File    string
	Implied bool
	Props   map[string]any
}

// idKeys maps the ids one side's export uses to stable keys.
type idKeys map[string]string

// isIDValued reports whether a property holds element ids as strings (the
// metamodel's elementId and the membership's memberElementId variants).
func isIDValued(property string) bool {
	return property == "elementId" || strings.HasSuffix(property, "ElementId")
}

// normalizeValue maps one raw JSON value to its comparable form: `{"@id": x}`
// objects become refs keyed through keys, id-valued strings become keys, lists
// keep their order, other objects become their canonical JSON text.
func normalizeValue(property string, raw any, keys idKeys) any {
	switch v := raw.(type) {
	case nil, bool, float64:
		return v
	case string:
		if isIDValued(property) {
			if key, ok := keys[v]; ok {
				return key
			}
			return unkeyed
		}
		return v
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = normalizeValue(property, item, keys)
		}
		return out
	case map[string]any:
		if id, ok := v["@id"].(string); ok && len(v) <= 2 {
			if key, ok := keys[id]; ok {
				return ref{key}
			}
			return ref{unkeyed}
		}
		return canonicalJSON(v)
	}
	return fmt.Sprint(raw)
}

// canonicalJSON renders a non-reference object with sorted keys.
func canonicalJSON(v map[string]any) string {
	encoded, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprint(v)
	}
	return string(encoded)
}

// isAbsent reports whether a normalized value states nothing: JSON null, a
// missing property, or an empty collection.
func isAbsent(v any) bool {
	if v == nil {
		return true
	}
	list, ok := v.([]any)
	return ok && len(list) == 0
}

// render summarizes a normalized value for an example, within limit runes.
func render(v any, limit int) string {
	var text string
	switch value := v.(type) {
	case nil:
		text = "absent"
	case ref:
		text = "→" + value.Key
	case []any:
		parts := make([]string, len(value))
		for i, item := range value {
			parts[i] = render(item, limit)
		}
		text = "[" + strings.Join(parts, ", ") + "]"
	case string:
		text = fmt.Sprintf("%q", value)
	default:
		text = fmt.Sprint(value)
	}
	if runes := []rune(text); len(runes) > limit {
		text = string(runes[:limit-1]) + "…"
	}
	return text
}

// refID is the id a raw `{"@id": x}` object names, or "".
func refID(raw any) string {
	object, ok := raw.(map[string]any)
	if !ok {
		return ""
	}
	id, _ := object["@id"].(string)
	return id
}

// refIDs lists the ids of a raw reference collection, in order.
func refIDs(raw any) []string {
	list, ok := raw.([]any)
	if !ok {
		return nil
	}
	ids := make([]string, 0, len(list))
	for _, item := range list {
		if id := refID(item); id != "" {
			ids = append(ids, id)
		}
	}
	return ids
}

// sortedKeys lists a map's keys in order.
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// canonicalKey spells a key with every name segment unquoted: the pilot writes
// a name that is not a plain identifier the way the textual notation quotes it
// ('Cycle Probe'::B1), this repository's writer does not, and a key must match
// across the two. The qualifiedName property itself is compared as written.
func canonicalKey(key string) string {
	if !strings.Contains(key, "'") {
		return key
	}
	var b strings.Builder
	runes := []rune(key)
	for i := 0; i < len(runes); i++ {
		if runes[i] != '\'' {
			b.WriteRune(runes[i])
			continue
		}
		for i++; i < len(runes) && runes[i] != '\''; i++ {
			if runes[i] == '\\' && i+1 < len(runes) {
				i++
			}
			b.WriteRune(runes[i])
		}
	}
	return b.String()
}
