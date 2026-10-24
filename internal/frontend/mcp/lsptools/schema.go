package lsptools

import (
	"reflect"

	"github.com/google/jsonschema-go/jsonschema"
)

// schemaWithRecursion infers the schema of root for a type elem that nests
// lists of itself, which plain inference rejects as a cycle: every list of
// elem becomes a reference to the definition named name+"s".
func schemaWithRecursion(root, elem reflect.Type, name string) *jsonschema.Schema {
	opts := &jsonschema.ForOptions{TypeSchemas: map[reflect.Type]*jsonschema.Schema{
		reflect.SliceOf(elem): {Ref: "#/$defs/" + name + "s"},
	}}
	elemSchema, err := jsonschema.ForType(elem, opts)
	if err != nil {
		panic(err)
	}
	rootSchema, err := jsonschema.ForType(root, opts)
	if err != nil {
		panic(err)
	}
	rootSchema.Defs = map[string]*jsonschema.Schema{
		name:       elemSchema,
		name + "s": {Type: "array", Items: &jsonschema.Schema{Ref: "#/$defs/" + name}},
	}
	return rootSchema
}
