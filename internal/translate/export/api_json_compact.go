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

var compactOwnershipSurvivors = []string{
	"owningMembership",
	"membershipOwningNamespace",
	"ownedMemberElement",
}

// CompactAPIJSONOptions chooses what the compact form leaves out.
type CompactAPIJSONOptions struct {
	// OmitDerived leaves out every property the metamodel derives, except the
	// ones named in KeepDerived. A derived property restates what the owned
	// properties already say, so a reader that follows ownership, typing and
	// specialization does not need it.
	OmitDerived bool
	// KeepDerived names the derived properties still written when OmitDerived is set.
	KeepDerived []string
	Omitted     func(APIJSONOmission)
	Diagnostic  func(string)
}

// WriteAPIJSONCompact writes the elements WriteAPIJSON writes in a form built
// to be small: no indentation, each element id written once, and every
// reference spelled by the position of its id. The document is one object:
//
//	{"format":"api-json-compact/1",
//	 "ids":[<id>, …],               // the id table
//	 "elementCount":N,              // ids[0..N) are the elements, in order
//	 "elementIdsAreIds":true,       // present only when elementId is left out (below)
//	 "referenceKeys":[<key>, …],    // keys whose values are only references or reference arrays
//	 "elements":[{"@type":…,…}, …]}
//
// Element i has the id ids[i], so it carries no "@id". Under a reference key a
// reference is the integer index into ids (an array of them for a collection);
// anywhere else it is {"@id": <index>}. ids beyond elementCount are elements
// the conversion references but does not write. Unresolved object-property
// names are null or omitted, as in the element form. When every
// element's "elementId" is its "@id", as it is unless a document declared
// another, "elementId" is left out and "elementIdsAreIds" says so: a reader
// restores it from the table.
func WriteAPIJSONCompact(graph *rdf.Graph, opts CompactAPIJSONOptions) ([]byte, error) {
	elements, err := apiJSONElementsWithOptions(graph, APIJSONOptions{
		Omitted: opts.Omitted, Diagnostic: opts.Diagnostic,
	})
	if err != nil {
		return nil, err
	}
	keepSize := len(opts.KeepDerived)
	if opts.OmitDerived {
		keepSize += len(compactOwnershipSurvivors)
	}
	keep := make(map[string]bool, keepSize)
	if opts.OmitDerived {
		for _, name := range compactOwnershipSurvivors {
			keep[name] = true
		}
	}
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

	// One pass applies omissions, hands out handles for every nested reference,
	// and classifies each kept member's top-level value.
	kept := make([]apiJSONObject, len(elements))
	references := map[string]bool{}
	disqualified := map[string]bool{}
	for i, element := range elements {
		typ, _ := element[0].value.(string)
		metaclass := typ
		if strings.HasPrefix(typ, "sysx:") {
			metaclass = ""
		}
		members := make(apiJSONObject, 0, len(element))
		members = append(members, element[0])
		for _, member := range element[2:] {
			if member.key == "qualifiedName" && member.value == nil {
				continue
			}
			if elementIDsAreIDs && member.key == "elementId" {
				continue
			}
			if member.key == apiJSONMappingValuesKey && opts.OmitDerived && metaclass != "" {
				if values, ok := member.value.(apiJSONObject); ok {
					filtered := make(apiJSONObject, 0, len(values))
					for _, value := range values {
						if property, ok := ontology.PropertyOf(metaclass, value.key); ok &&
							property.Derived && !keep[value.key] {
							continue
						}
						filtered = append(filtered, value)
					}
					if len(filtered) == 0 {
						continue
					}
					member.value = filtered
				}
			}
			if opts.OmitDerived && metaclass != "" && !keep[member.key] {
				if property, ok := ontology.PropertyOf(metaclass, member.key); ok && property.Derived {
					continue
				}
			}
			members = append(members, member)
			switch value := member.value.(type) {
			case nil:
			case apiJSONReference:
				references[member.key] = true
			case []any:
				if len(value) == 0 {
					break
				}
				allReferences := true
				for _, item := range value {
					if _, ok := item.(apiJSONReference); !ok {
						allReferences = false
						break
					}
				}
				if allReferences {
					references[member.key] = true
				} else {
					disqualified[member.key] = true
				}
			default:
				disqualified[member.key] = true
			}
			walkCompactValue(member.value, func(value any) {
				if reference, ok := value.(apiJSONReference); ok {
					handle(reference.ID)
				}
			})
		}
		kept[i] = members
	}
	referenceKeys := make([]string, 0, len(references))
	for key := range references {
		if !disqualified[key] {
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
	switch values := value.(type) {
	case []any:
		for _, v := range values {
			walkCompactValue(v, visit)
		}
	case map[string]any:
		for _, v := range values {
			walkCompactValue(v, visit)
		}
	case apiJSONObject:
		for _, member := range values {
			walkCompactValue(member.value, visit)
		}
	default:
		visit(value)
	}
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
	case map[string]any:
		keys := make([]string, 0, len(v))
		for key := range v {
			keys = append(keys, key)
		}
		slices.Sort(keys)
		w.buf.WriteByte('{')
		for i, key := range keys {
			if i > 0 {
				w.buf.WriteByte(',')
			}
			if err := w.value(key); err != nil {
				return err
			}
			w.buf.WriteByte(':')
			if err := writeCompactValue(w, v[key], false, handles); err != nil {
				return err
			}
		}
		w.buf.WriteByte('}')
		return nil
	case apiJSONObject:
		w.buf.WriteByte('{')
		for i, member := range v {
			if i > 0 {
				w.buf.WriteByte(',')
			}
			if err := w.value(member.key); err != nil {
				return err
			}
			w.buf.WriteByte(':')
			if err := writeCompactValue(w, member.value, false, handles); err != nil {
				return err
			}
		}
		w.buf.WriteByte('}')
		return nil
	}
	return w.value(value)
}
