package export

import (
	"errors"
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/core/libs"
	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
)

// Sources is the `sources` model form: every document of the model as text, in
// path order, and the version of the standard library the host loaded beside them.
type Sources struct {
	Library   string     `json:"library"`
	Documents []Document `json:"documents"`
}

// Document is one source document of the model, its registered name and its text.
type Document struct {
	Path string `json:"path"`
	Text string `json:"text"`
}

// ErrNoSources reports a model that registered no document to export.
var ErrNoSources = errors.New("export: the model has no source documents")

// ErrFormUnsupported reports a model form this build does not export for an engine.
var ErrFormUnsupported = errors.New("export: model form not supported")

// FormUnsupportedError says which form was asked for and why it is not exported.
type FormUnsupportedError struct {
	Form   string
	Reason string
}

func (e *FormUnsupportedError) Error() string {
	return fmt.Sprintf("model form %q: %s", e.Form, e.Reason)
}

// Unwrap makes the error match ErrFormUnsupported.
func (e *FormUnsupportedError) Unwrap() error { return ErrFormUnsupported }

// RefuseRDFForm is the typed refusal of the `rdf` model form for an engine: the
// design defines it and the stage that adds policies and samplers delivers it.
func RefuseRDFForm() error {
	return &FormUnsupportedError{Form: "rdf", Reason: "not exported to engines in this build; it is delivered with the policy and sampler stage — ask for sources or graphs:1"}
}

// SourcesOf exports the `sources` form of a model: the registered documents in
// name order with their whole text, and libs.Version for the standard library.
func SourcesOf(model *runtime.Model) (*Sources, error) {
	if model == nil {
		return nil, ErrNoSources
	}
	files := model.Sources()
	if len(files) == 0 {
		return nil, ErrNoSources
	}
	docs := make([]Document, 0, len(files))
	for _, sf := range files {
		docs = append(docs, Document{Path: sf.Name(), Text: string(sf.Bytes())})
	}
	return &Sources{Library: libs.Version(), Documents: docs}, nil
}
