// Package replext is where REPL features that need a translator or a transport
// register themselves, so a bare REPL links neither; replext/all links them all.
package replext

import (
	"context"

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

// ProjectInfo names one project of a repository.
type ProjectInfo struct {
	ID   string
	Name string
}

// ProjectState is the branch a session's model was loaded from or published
// to: what a later publish commits on top of. LastSeenCommit is the head the
// session last read or wrote.
type ProjectState struct {
	Base           string // the server the state was read from
	ProjectID      string
	ProjectName    string
	Branch         string
	BranchName     string
	LastSeenCommit string
}

// LoadRequest picks the project and branch %load reads: a project by id or
// else by name, a branch by name or id or else the project's default.
type LoadRequest struct {
	ProjectID string
	Name      string
	Branch    string
}

// LoadResult is a loaded branch: its notation, ready to submit, and where it
// came from.
type LoadResult struct {
	State    ProjectState
	Notation []byte
	Warnings []string
}

// PublishRequest publishes the elements rooted in one element of the session
// model: Root is its qualified name, Project the project name (the element's
// own name when empty), Branch the branch name (the default when empty), State
// the branch the session loaded the project from, if it did.
type PublishRequest struct {
	Origin  string
	Source  []byte
	Root    string
	Project string
	Branch  string
	Derived bool
	State   *ProjectState
}

// PublishResult is what a publish did: the project and branch it wrote, the
// commit it made (empty when the branch already agreed), and how many elements
// it created, updated and deleted. Created reports a project made anew.
type PublishResult struct {
	State                     ProjectState
	Commit                    string
	Created, Updated, Deleted int
	NewProject                bool
	Notes                     []string
}

// Repository is what %repo, %projects, %load and %publish drive: a SysML v2 API
// server addressed by base URL. Commands that reach the network take the base
// URL every time, as %repo may change it between them.
type Repository interface {
	// DefaultURL is the base URL a session starts with, from the environment.
	DefaultURL() string
	// CheckURL refuses a base URL the transport policy does not allow.
	CheckURL(base string) error
	// Projects lists every project, paged through completely.
	Projects(ctx context.Context, base string) ([]ProjectInfo, error)
	// Load reads a branch head as notation.
	Load(ctx context.Context, base string, req LoadRequest) (*LoadResult, error)
	// Publish writes the elements rooted in one element as a project or a commit.
	Publish(ctx context.Context, base string, req PublishRequest) (*PublishResult, error)
}

var repository Repository

// RegisterRepository installs the repository the %repo family of commands drives.
func RegisterRepository(r Repository) {
	if repository != nil {
		panic("replext: repository registered twice")
	}
	repository = r
}

// Repo returns the registered repository, nil when no binary linked one.
func Repo() Repository { return repository }
