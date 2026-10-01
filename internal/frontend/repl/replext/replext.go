// Package replext is where REPL features that need a translator or a transport
// register themselves, so a bare REPL links neither; replext/all links them all.
package replext

import (
	"github.com/Open-MBEE/OpenSysML/internal/doc/docrender"
	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// NotationWriter prints the model as SysML notation and saves it in the format
// a file name selects, for %print and %save.
type NotationWriter interface {
	// Print is the prompt lines printing src, the session buffer named origin,
	// through the writer a `.sysml` save writes with.
	Print(origin string, src []byte) []string
	// PrintElement is the prompt lines printing the source span covers in file;
	// shown names the element in the line refusing a span with no notation.
	PrintElement(file *source.SourceFile, span source.Span, shown string) []string
	// Save writes src, the session buffer named origin, to path in the format
	// its extension names, and is the lines reporting what it did. The error is
	// only the write's own.
	Save(origin string, src []byte, path string) ([]string, error)
}

// GraphWriter writes an object and what is reachable from it as the API's
// Instantiate response, for %features json.
type GraphWriter interface {
	// DefaultInstances is how many objects a listing not asked for whole holds.
	DefaultInstances() int
	// Write is the JSON of inst's graph within depth and instances; warning, when
	// the instance bound cut the graph short, is the diagnostic it carries.
	// failures are the feature values the graph could not answer.
	Write(ctx *runtime.Context, inst *runtime.Instance, index *symbols.Index, depth, instances int, warning func() string) (out []byte, failures []error, err error)
}

// PositionalDocument pairs a session document with the scope holding its symbols.
type PositionalDocument struct {
	File *source.SourceFile
	Root *symbols.Scope
}

// PositionalNamer names the symbols identifies refuses as the RDF export names
// them, by position, for OSLC query; byName is the inverse.
type PositionalNamer func(index *symbols.Index, docs []PositionalDocument, identifies func(*symbols.Symbol) bool) (names map[*symbols.Symbol]string, byName map[string]*symbols.Symbol)

var (
	notation   NotationWriter
	graph      GraphWriter
	positional PositionalNamer
	drawer     docrender.DiagramDrawer
)

// RegisterNotation installs the writer %print and %save use.
func RegisterNotation(w NotationWriter) {
	if notation != nil {
		panic("replext: notation writer registered twice")
	}
	notation = w
}

// RegisterGraph installs the writer %features json uses.
func RegisterGraph(w GraphWriter) {
	if graph != nil {
		panic("replext: graph writer registered twice")
	}
	graph = w
}

// RegisterPositional installs the namer OSLC query identifies unnamed elements with.
func RegisterPositional(n PositionalNamer) {
	if positional != nil {
		panic("replext: positional namer registered twice")
	}
	positional = n
}

// RegisterDrawer installs the drawer %render-document draws DOT diagrams with.
func RegisterDrawer(d docrender.DiagramDrawer) {
	if drawer != nil {
		panic("replext: diagram drawer registered twice")
	}
	drawer = d
}

// Notation is the registered notation writer, nil when none is linked.
func Notation() NotationWriter { return notation }

// Graph is the registered graph writer, nil when none is linked.
func Graph() GraphWriter { return graph }

// Positional is the registered positional namer, nil when none is linked.
func Positional() PositionalNamer { return positional }

// Drawer is the registered diagram drawer, nil when none is linked.
func Drawer() docrender.DiagramDrawer { return drawer }
