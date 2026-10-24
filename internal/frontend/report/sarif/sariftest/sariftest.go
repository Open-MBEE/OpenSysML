// Package sariftest validates SARIF documents against the vendored SARIF 2.1.0
// schema, so a writer's tests can prove its output is what a code-scanning
// service reads. It is for tests only and is linked into no command.
package sariftest

import (
	"bytes"
	_ "embed" // for the go:embed of the vendored schema
	"fmt"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// schema is the OASIS SARIF 2.1.0 (errata 01) JSON schema, vendored so the
// check needs no network.
//
//go:embed sarif-schema-2.1.0.json
var schema []byte

// Validate reports why document is not a valid SARIF 2.1.0 log, or nil.
func Validate(document []byte) error {
	schemaDoc, err := jsonschema.UnmarshalJSON(bytes.NewReader(schema))
	if err != nil {
		return fmt.Errorf("vendored schema: %w", err)
	}
	c := jsonschema.NewCompiler()
	if err := c.AddResource("sarif-schema-2.1.0.json", schemaDoc); err != nil {
		return err
	}
	compiled, err := c.Compile("sarif-schema-2.1.0.json")
	if err != nil {
		return fmt.Errorf("vendored schema: %w", err)
	}
	instance, err := jsonschema.UnmarshalJSON(bytes.NewReader(document))
	if err != nil {
		return fmt.Errorf("not JSON: %w", err)
	}
	if err := compiled.Validate(instance); err != nil {
		return fmt.Errorf("not valid SARIF 2.1.0: %w", err)
	}
	return nil
}
