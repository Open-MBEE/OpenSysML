package main

import (
	"fmt"
	"path/filepath"
	"sort"

	"google.golang.org/protobuf/reflect/protoreflect"
)

// Placeholders a normalized response carries in place of a value that is not
// the same twice. Scenarios compare against these spellings.
const (
	modelHashPlaceholder = "${model_hash}"
	versionPlaceholder   = "${version}"
	pathPlaceholder      = "${path}"
)

// normalizedIDs are the int64 fields carrying a runtime instance id, which is
// assigned per call: each distinct id becomes "@1", "@2", … in order of first
// appearance, so a scenario can state that two feature values name one object
// without knowing which id it was given.
var normalizedIDs = map[string]bool{
	"sysml.Instance.id":         true,
	"sysml.Value.instance_id":   true,
	"sysml.Verdict.instance_id": true,
	"sysml.Function.self_id":    true,
}

// integer and unsigned are normalized integral values. They are distinct from
// float64 so an integral field is compared exactly rather than within the
// tolerance a Real carries.
type (
	integer  int64
	unsigned uint64
)

// normalizer turns a response into a JSON-shaped tree with the values that
// cannot be compared literally replaced. See conformance/README.md.
type normalizer struct {
	modelHash string
	labels    map[int64]string
}

func newNormalizer(modelHash string) *normalizer {
	return &normalizer{modelHash: modelHash, labels: map[int64]string{}}
}

// normalize renders msg as maps, slices, strings, numbers and bools. Only set
// fields appear; a scalar left at its default is absent.
func (n *normalizer) normalize(msg protoreflect.Message) map[string]any {
	out := map[string]any{}
	var set []protoreflect.FieldDescriptor
	msg.Range(func(field protoreflect.FieldDescriptor, _ protoreflect.Value) bool {
		set = append(set, field)
		return true
	})
	// Range yields fields in no defined order; number order makes id labels
	// reproducible.
	sort.Slice(set, func(i, j int) bool { return set[i].Number() < set[j].Number() })

	for _, field := range set {
		value := msg.Get(field)
		switch {
		case field.IsMap():
			entries := map[string]any{}
			keys := make([]string, 0, value.Map().Len())
			byKey := map[string]protoreflect.Value{}
			value.Map().Range(func(key protoreflect.MapKey, item protoreflect.Value) bool {
				keys = append(keys, key.String())
				byKey[key.String()] = item
				return true
			})
			sort.Strings(keys)
			for _, key := range keys {
				entries[key] = n.value(field.MapValue(), byKey[key])
			}
			out[field.TextName()] = entries
		case field.IsList():
			list := value.List()
			items := make([]any, 0, list.Len())
			for i := 0; i < list.Len(); i++ {
				items = append(items, n.value(field, list.Get(i)))
			}
			out[field.TextName()] = items
		default:
			out[field.TextName()] = n.value(field, value)
		}
	}
	return out
}

// value normalizes one field value, list element or map value.
func (n *normalizer) value(field protoreflect.FieldDescriptor, value protoreflect.Value) any {
	full := string(field.FullName())
	switch field.Kind() {
	case protoreflect.MessageKind, protoreflect.GroupKind:
		return n.normalize(value.Message())
	case protoreflect.EnumKind:
		return string(field.Enum().Values().ByNumber(value.Enum()).Name())
	case protoreflect.BoolKind:
		return value.Bool()
	case protoreflect.StringKind:
		return n.stringValue(full, value.String())
	case protoreflect.DoubleKind, protoreflect.FloatKind:
		return value.Float()
	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		if normalizedIDs[full] {
			return n.label(value.Int())
		}
		return integer(value.Int())
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
		return integer(value.Int())
	case protoreflect.Uint32Kind, protoreflect.Uint64Kind, protoreflect.Fixed32Kind, protoreflect.Fixed64Kind:
		return unsigned(value.Uint())
	default:
		return value.String()
	}
}

// stringValue replaces the strings a call cannot repeat: the model hash it was
// given, the service's build version, and any absolute path.
func (n *normalizer) stringValue(field, text string) string {
	switch {
	case field == "sysml.ServerInfoResponse.version":
		return versionPlaceholder
	case n.modelHash != "" && text == n.modelHash:
		return modelHashPlaceholder
	case filepath.IsAbs(text):
		return pathPlaceholder
	default:
		return text
	}
}

// label is the symbolic name of a runtime instance id.
func (n *normalizer) label(id int64) string {
	if existing, ok := n.labels[id]; ok {
		return existing
	}
	label := fmt.Sprintf("@%d", len(n.labels)+1)
	n.labels[id] = label
	return label
}
