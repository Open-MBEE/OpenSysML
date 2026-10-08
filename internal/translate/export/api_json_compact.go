package export

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf/ontology"
)

// CompactAPIJSONFormat names the document WriteAPIJSONCompact writes.
const CompactAPIJSONFormat = "api-json-compact/1"

// CompactAPIJSONOptions chooses what the compact form leaves out.
type CompactAPIJSONOptions struct {
	// OmitDerived leaves out every property the metamodel derives, except the
	// ones named in KeepDerived. A derived property restates what the owned
	// properties already say, so a reader that follows ownership, typing and
	// specialization does not need it.
	OmitDerived bool
	// KeepDerived names the derived properties still written when OmitDerived is set.
	KeepDerived []string
}

// WriteAPIJSONCompact writes the elements WriteAPIJSON writes in a form built
// to be small: no indentation, each element id written once, and every
// reference spelled by the position of its id. The document is one object:
//
//	{"format":"api-json-compact/1",
//	 "ids":[<id>, …],               // the id table
//	 "elementCount":N,              // ids[0..N) are the elements, in order
//	 "elementIdsAreIds":true,       // present only when elementId is left out (below)
//	 "referenceKeys":[<key>, …],    // keys whose values are bare handles
//	 "elements":[{"@type":…,…}, …]}
//
// Element i has the id ids[i], so it carries no "@id". Under a reference key a
// reference is the integer index into ids (an array of them for a collection);
// anywhere else it is {"@id": <index>}. ids beyond elementCount are elements
// the conversion references but does not write. Everything else, scalars and
// {"@ref": name} included, is spelled as in the element form. When every
// element's "elementId" is its "@id", as it is unless a document declared
// another, "elementId" is left out and "elementIdsAreIds" says so: a reader
// restores it from the table.
func WriteAPIJSONCompact(graph *rdf.Graph, opts CompactAPIJSONOptions) ([]byte, error) {
	elements, err := apiJSONElements(graph)
	if err != nil {
		return nil, err
	}
	keep := make(map[string]bool, len(opts.KeepDerived))
	for _, name := range opts.KeepDerived {
		keep[name] = true
	}

	ids := make([]string, 0, len(elements))
	handles := make(map[string]int, len(elements))
	for _, element := range elements {
		id, _ := element[1].value.(string)
		if _, seen := handles[id]; !seen {
			handles[id] = len(ids)
		}
		ids = append(ids, id)
	}
	handle := func(id string) int {
		if h, ok := handles[id]; ok {
			return h
		}
		handles[id] = len(ids)
		ids = append(ids, id)
		return len(ids) - 1
	}

	// "elementId" repeats the id for every element unless a document declared
	// another; it is left out only when it does so for all.
	elementIDsAreIDs := len(elements) > 0
	for _, element := range elements {
		found := false
		for _, member := range element[2:] {
			if member.key == "elementId" {
				found = member.value == element[1].value
				break
			}
		}
		if !found {
			elementIDsAreIDs = false
			break
		}
	}

	// One pass drops what is left out, hands out the handles, and finds which
	// keys carry references and which carry numbers: a key that carries both
	// cannot spell a reference as a bare integer.
	kept := make([]apiJSONObject, len(elements))
	references := map[string]bool{}
	numbers := map[string]bool{}
	for i, element := range elements {
		typ, _ := element[0].value.(string)
		metaclass := typ
		if strings.HasPrefix(typ, "sysx:") {
			metaclass = ""
		}
		members := make(apiJSONObject, 0, len(element))
		members = append(members, element[0])
		for _, member := range element[2:] {
			if elementIDsAreIDs && member.key == "elementId" {
				continue
			}
			if opts.OmitDerived && metaclass != "" && !keep[member.key] {
				if property, ok := ontology.PropertyOf(metaclass, member.key); ok && property.Derived {
					continue
				}
			}
			members = append(members, member)
			walkCompactValue(member.value, func(value any) {
				switch v := value.(type) {
				case apiJSONReference:
					handle(v.ID)
					references[member.key] = true
				case json.Number:
					numbers[member.key] = true
				}
			})
		}
		kept[i] = members
	}
	referenceKeys := make([]string, 0, len(references))
	for key := range references {
		if !numbers[key] {
			referenceKeys = append(referenceKeys, key)
		}
	}
	slices.Sort(referenceKeys)

	var out bytes.Buffer
	w := apiJSONWriter{buf: &out, enc: json.NewEncoder(&out)}
	out.WriteString(`{"format":`)
	if err := w.value(CompactAPIJSONFormat); err != nil {
		return nil, err
	}
	out.WriteString(`,"ids":[`)
	for i, id := range ids {
		if i > 0 {
			out.WriteByte(',')
		}
		if err := w.value(id); err != nil {
			return nil, err
		}
	}
	fmt.Fprintf(&out, `],"elementCount":%d,`, len(elements))
	if elementIDsAreIDs {
		out.WriteString(`"elementIdsAreIds":true,`)
	}
	out.WriteString(`"referenceKeys":`)
	if err := w.value(referenceKeys); err != nil {
		return nil, err
	}
	out.WriteString(`,"elements":[`)
	bare := make(map[string]bool, len(referenceKeys))
	for _, key := range referenceKeys {
		bare[key] = true
	}
	for i, element := range kept {
		if i > 0 {
			out.WriteByte(',')
		}
		out.WriteByte('{')
		for j, member := range element {
			if j > 0 {
				out.WriteByte(',')
			}
			if err := w.value(member.key); err != nil {
				return nil, err
			}
			out.WriteByte(':')
			if err := writeCompactValue(w, member.value, bare[member.key], handles); err != nil {
				return nil, fmt.Errorf("%s: %w", member.key, err)
			}
		}
		out.WriteByte('}')
	}
	out.WriteString("]}\n")
	return out.Bytes(), nil
}

// walkCompactValue visits a member's value, or each member of its collection.
func walkCompactValue(value any, visit func(any)) {
	if values, ok := value.([]any); ok {
		for _, v := range values {
			visit(v)
		}
		return
	}
	visit(value)
}

// writeCompactValue spells a value in the compact form: a reference by its
// handle, bare under a reference key, and anything else as the element form does.
func writeCompactValue(w apiJSONWriter, value any, bare bool, handles map[string]int) error {
	switch v := value.(type) {
	case apiJSONReference:
		if bare {
			w.buf.WriteString(strconv.Itoa(handles[v.ID]))
		} else {
			fmt.Fprintf(w.buf, `{"@id":%d}`, handles[v.ID])
		}
		return nil
	case []any:
		w.buf.WriteByte('[')
		for i, item := range v {
			if i > 0 {
				w.buf.WriteByte(',')
			}
			if err := writeCompactValue(w, item, bare, handles); err != nil {
				return err
			}
		}
		w.buf.WriteByte(']')
		return nil
	}
	return w.value(value)
}
