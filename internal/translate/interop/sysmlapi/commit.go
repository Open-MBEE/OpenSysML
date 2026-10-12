package sysmlapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/translate/interop/reposync"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

// CommitRequest encodes a batch of element changes as the API's CommitRequest:
// creates and updates send the element whole under its identity, deletes send
// a null payload.
func CommitRequest(changes []reposync.ElementChange, message string) ([]byte, error) {
	versions := make([]map[string]any, 0, len(changes))
	for _, change := range changes {
		version := map[string]any{
			"@type":    "DataVersion",
			"identity": map[string]string{"@id": change.ID},
			"payload":  nil,
		}
		switch change.Kind {
		case reposync.KindDelete:
		case reposync.KindCreate, reposync.KindUpdate:
			content, err := payload(change)
			if err != nil {
				return nil, fmt.Errorf("%s %s: %w", change.Kind, change.ID, err)
			}
			version["payload"] = content
		default:
			return nil, fmt.Errorf("%s %s: the API has no commit for a %s", change.Kind, change.ID, change.Kind)
		}
		versions = append(versions, version)
	}
	request := map[string]any{"@type": "Commit", "change": versions}
	if message != "" {
		request["description"] = message
	}
	return json.Marshal(request)
}

// ErrUntyped is an element with no single sysml: metaclass, which the API's
// payload cannot state.
var ErrUntyped = errors.New("the element has no single SysML metaclass")

// payload renders an element's content as the API's JSON: @id and @type, then
// each sysml: property as a value or, when multi-valued, an array.
func payload(change reposync.ElementChange) (map[string]json.RawMessage, error) {
	out := map[string]json.RawMessage{}
	values := map[string][]json.RawMessage{}
	metaclass := ""
	for _, triple := range change.Content {
		predicate := triple.Predicate.Value
		if predicate == rdf.RDFType {
			name, ok := strings.CutPrefix(triple.Object.Value, rdf.SysML)
			if !ok || !triple.Object.IsIRI() || metaclass != "" {
				return nil, ErrUntyped
			}
			metaclass = name
			continue
		}
		name, ok := strings.CutPrefix(predicate, rdf.SysML)
		if !ok {
			return nil, &UnrepresentableError{Term: triple.Predicate, Reason: "the API stores sysml: properties only"}
		}
		value, err := jsonValue(triple.Object)
		if err != nil {
			return nil, err
		}
		values[name] = append(values[name], value)
	}
	if metaclass == "" {
		return nil, ErrUntyped
	}
	for name, list := range values {
		if len(list) == 1 {
			out[name] = list[0]
			continue
		}
		array, err := json.Marshal(list)
		if err != nil {
			return nil, err
		}
		out[name] = array
	}
	id, err := json.Marshal(change.ID)
	if err != nil {
		return nil, err
	}
	out["@id"] = id
	if out["@type"], err = json.Marshal(metaclass); err != nil {
		return nil, err
	}
	return out, nil
}

// jsonValue spells one carried term as the API reads it back from JSON.
func jsonValue(term rdf.Term) (json.RawMessage, error) {
	carried, err := carryTerm(term)
	if err != nil {
		return nil, err
	}
	if carried.IsIRI() {
		if carried.Value == rdfNil {
			return json.RawMessage("null"), nil
		}
		return json.Marshal(map[string]string{"@id": rdf.LocalName(carried.Value)})
	}
	switch carried.Datatype {
	case rdf.XSD + "boolean", rdf.XSD + "integer", rdf.XSD + "decimal":
		return json.RawMessage(carried.Value), nil
	}
	return json.Marshal(carried.Value)
}
