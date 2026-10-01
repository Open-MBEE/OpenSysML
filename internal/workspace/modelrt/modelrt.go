// Package modelrt builds execution runtimes over a workspace's documents, so the
// workspace model itself never links the runtime.
package modelrt

import (
	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/model"
)

// Origin is what a runtime is built from: a model.Workspace, or a model.Reading under its lock.
type Origin interface {
	Detach() (*model.Detached, error)
}

// Runtime is a runtime model over the workspace's documents as they stood when
// it was built, on an index of its own that later edits do not touch.
type Runtime struct {
	*model.Detached
	model *runtime.Model
}

// New builds a runtime over origin's current documents on a detached index.
func New(origin Origin) (*Runtime, error) {
	d, err := origin.Detach()
	if err != nil {
		return nil, err
	}
	m := runtime.NewModel(d.Semantics(), d.Resolver())
	m.SetExpressionParser(parser.ParseOneExpression)
	for _, name := range d.Documents() {
		m.RegisterSource(d.Source(name))
		m.RegisterScope(d.Index().DocumentRoot(name))
	}
	return &Runtime{Detached: d, model: m}, nil
}

// Model is the runtime model executions are built over with runtime.NewContext.
func (r *Runtime) Model() *runtime.Model { return r.model }
