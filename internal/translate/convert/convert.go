// Package convert is the entry point of `sysml -convert`, `%save` and the
// service's Export: it names the formats a model is read from and written to
// and drives each conversion, parsing notation and migrating SysML v1 XMI
// before handing the tree or graph to internal/translate/export.
package convert

import (
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/format"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/lexer"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/translate/export"
	"github.com/Open-MBEE/OpenSysML/internal/translate/migrate"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
	"github.com/Open-MBEE/OpenSysML/internal/translate/simresults"
)

// Format is one of the representations a model can be read from or written to.
type Format int

const (
	// FormatSysML is SysML v2 / KerML textual notation.
	FormatSysML Format = iota
	// FormatTurtle is RDF in Turtle syntax.
	FormatTurtle
	// FormatXMI is SysML v1 as UML XMI 2.5.1, the OMG SysML profile applied, or
	// a .mdzip archive holding it. It is an input format only.
	FormatXMI
	// FormatAPIJSON is the OMG SysML v2 API element form: JSON objects with
	// "@type", "@id" and the metamodel properties as keys, over the same graph
	// the Turtle mapping builds.
	FormatAPIJSON
)

func (f Format) String() string {
	switch f {
	case FormatTurtle:
		return "ttl"
	case FormatXMI:
		return "xmi"
	case FormatAPIJSON:
		return "api-json"
	}
	return "sysml"
}

// Writable reports whether models can be written in the format.
func (f Format) Writable() bool {
	return f != FormatXMI
}

// formatNames are the names accepted on the command line for each format.
var formatNames = map[string]Format{
	"sysml":    FormatSysML,
	"kerml":    FormatSysML,
	"text":     FormatSysML,
	"ttl":      FormatTurtle,
	"turtle":   FormatTurtle,
	"rdf":      FormatTurtle,
	"xmi":      FormatXMI,
	"uml":      FormatXMI,
	"mdzip":    FormatXMI,
	"api-json": FormatAPIJSON,
	"json":     FormatAPIJSON,
}

// FormatList is the wording every surface lists the format names in.
const FormatList = "sysml, kerml, ttl, turtle, rdf, api-json, or xmi/uml/mdzip (input only)"

// FormatNames returns every name ParseFormat accepts, sorted.
func FormatNames() []string {
	names := make([]string, 0, len(formatNames))
	for name := range formatNames {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// ParseFormat resolves a format name, as given to `-convert`/`-from`.
func ParseFormat(name string) (Format, error) {
	if f, ok := formatNames[strings.ToLower(strings.TrimSpace(name))]; ok {
		return f, nil
	}
	return 0, fmt.Errorf("unknown format %q: expected %s", name, FormatList)
}

// NotWritableError reports a request to write a format that is only read.
type NotWritableError struct{ Format Format }

func (e *NotWritableError) Error() string {
	return fmt.Sprintf("cannot write %s: SysML v1 XMI is read and migrated, never written; convert to sysml or ttl", e.Format)
}

// UnknownFormatError reports that a path does not say which format to write.
// The remedy differs by surface — the command line has -convert/-from, the REPL only
// has the file name — so the caller supplies it with Advise.
type UnknownFormatError struct {
	Path string
	// NoExtension distinguishes a path with no extension from one whose
	// extension names a format we do not write.
	NoExtension bool
	// Advice is the surface's remedy, appended as "…, so <advice>".
	Advice string
}

func (e *UnknownFormatError) Error() string {
	reason := "expected .sysml, .kerml, .ttl or .json"
	if e.NoExtension {
		reason = "it has no extension"
	}
	msg := fmt.Sprintf("cannot tell the format of %q: %s", e.Path, reason)
	if e.Advice != "" {
		msg += ", so " + e.Advice
	}
	return msg
}

// ExtensionAdvice is the remedy every surface shares: the file name says which
// format to write. A surface with a format flag names it alongside this.
const ExtensionAdvice = "name the file with a .sysml, .kerml, .ttl or .json extension"

// Advise returns err with the surface's remedy attached when it is an
// *UnknownFormatError, and unchanged otherwise.
func Advise(err error, advice string) error {
	var unknown *UnknownFormatError
	if errors.As(err, &unknown) {
		return &UnknownFormatError{Path: unknown.Path, NoExtension: unknown.NoExtension, Advice: advice}
	}
	return err
}

// FormatOfPath infers the format from a file extension, so that the common case
// needs no -from. A path that names no format yields an
// *UnknownFormatError carrying no advice; pass it through Advise to add the
// remedy the calling surface offers.
func FormatOfPath(path string) (Format, error) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".sysml", ".kerml":
		return FormatSysML, nil
	case ".ttl", ".turtle":
		return FormatTurtle, nil
	case ".json":
		return FormatAPIJSON, nil
	case ".xmi", ".uml", ".mdzip":
		return FormatXMI, nil
	case "":
		return 0, &UnknownFormatError{Path: path, NoExtension: true}
	default:
		return 0, &UnknownFormatError{Path: path}
	}
}

// SyntaxError reports that the input could not be read as its format. It lists
// every syntax error rather than only the first, so one conversion attempt
// shows everything that needs fixing.
type SyntaxError struct {
	Name     string
	Messages []string
	// Diags are the diagnostics behind Messages, for a caller that reports them
	// with their spans. Empty when the input is not notation, since a Turtle
	// reader reports a message and no span.
	Diags []parser.Diagnostic
	// File is what Diags' spans point into; nil when Diags is empty.
	File *source.SourceFile
}

func (e *SyntaxError) Error() string {
	return fmt.Sprintf("%s: %d syntax error(s):\n  %s", e.Name, len(e.Messages), strings.Join(e.Messages, "\n  "))
}

// Options carries the conversion settings a caller may change from their
// defaults.
type Options struct {
	// ID is the form derived element ids are written in; the zero value is
	// qualified-name-derived ids.
	ID export.IDForm
}

// Convert reads data in the from format and writes it in the to format. name is
// used in diagnostics and needs no relation to a file on disk.
//
// Converting between two formats requires the input to be syntactically valid:
// a model with syntax errors is rejected, because the tree the parser recovers
// from broken input does not hold the declarations it could not read, and a
// graph built from it would be quietly missing them.
func Convert(name string, data []byte, from, to Format) ([]byte, error) {
	return ConvertWith(name, data, from, to, Options{})
}

// ConvertWith is Convert under non-default options.
func ConvertWith(name string, data []byte, from, to Format, opts Options) ([]byte, error) {
	out, _, err := convert(name, data, from, to, false, opts)
	return out, err
}

// ConvertTolerant is Convert with one difference: notation converted back to
// notation is written even when the parser could not read all of it, and its
// syntax errors are returned as a warning instead of an error. That direction
// re-indents the input rather than building anything from the parse tree, so the
// output is exactly as valid as the input and refusing it would only strand
// work that exists nowhere else — which is why the REPL's `%save` uses it for a
// session buffer. Every other direction builds a graph from the tree, where
// declarations the parser could not read would be silently missing, so a broken
// model is still rejected.
func ConvertTolerant(name string, data []byte, from, to Format) ([]byte, *SyntaxError, error) {
	return convert(name, data, from, to, true, Options{})
}

// ErrNoNotation reports an element no notation can be written for: one the
// document holds no source of, as a symbol read from an index cache is.
var ErrNoNotation = errors.New("no notation to write")

// SysMLElement writes the notation of one element of a document: the source at
// span, through the same writer a whole-document notation save goes through, so
// what one surface writes cannot drift from what another writes. Syntax errors
// are tolerated and returned as a warning, as ConvertTolerant does, since the
// span comes from a buffer that is written back as typed.
//
// Trailing comments are dropped: a declaration's span ends where the next token
// begins, so it runs over the notes written for whatever follows it.
func SysMLElement(file *source.SourceFile, span source.Span) ([]byte, *SyntaxError, error) {
	if file == nil || span.Len <= 0 || span.Offset < 0 || span.End() > file.Len() {
		return nil, nil, ErrNoNotation
	}
	text := trimTrailingTrivia(file.Text(span))
	if text == "" {
		return nil, nil, ErrNoNotation
	}
	return convert(file.Name(), []byte(text), FormatSysML, FormatSysML, true, Options{})
}

// trimTrailingTrivia cuts source at its last token that is neither whitespace
// nor a comment or note, and drops the whitespace before its first one.
func trimTrailingTrivia(text string) string {
	lx := lexer.New(source.New("element", []byte(text)))
	end := 0
	for tok := lx.Next(); tok.Kind != lexer.EOF; tok = lx.Next() {
		if tok.IsTrivia() || tok.Kind == lexer.RegularComment {
			continue
		}
		end = tok.Span.End()
	}
	return strings.TrimSpace(text[:end])
}

// Migration is a v1 model written in another format, with the report of what
// each v1 element became and the results its simulation tool stored.
type Migration struct {
	Output  []byte
	Report  *migrate.Report
	Results *simresults.Results
}

// Migrate reads a SysML v1 model in XMI and writes it in the to format. opts
// carries the migration's augments: an MTIP export whose diagram records lay
// out the views the migration writes.
func Migrate(name string, data []byte, to Format, opts migrate.Options) (*Migration, error) {
	if !to.Writable() {
		return nil, &NotWritableError{Format: to}
	}
	result, err := migrate.MigrateOptions(name, data, opts)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	out, _, err := convert(name+".sysml", result.Notation, FormatSysML, to, false, Options{})
	if err != nil {
		return nil, fmt.Errorf("the migrated notation could not be written: %w", err)
	}
	return &Migration{Output: out, Report: result.Report, Results: result.Results}, nil
}

func convert(name string, data []byte, from, to Format, tolerateSyntaxErrors bool, opts Options) ([]byte, *SyntaxError, error) {
	switch {
	case !to.Writable():
		return nil, nil, &NotWritableError{Format: to}

	case from == FormatXMI:
		m, err := Migrate(name, data, to, migrate.Options{})
		if err != nil {
			return nil, nil, err
		}
		return m.Output, nil, nil

	case from == FormatSysML && to == FormatSysML:
		// A save of textual notation: keep every lexeme, fix the indentation.
		syntax := checkSyntax(name, data)
		if syntax != nil && !tolerateSyntaxErrors {
			return nil, nil, syntax
		}
		out, err := format.Source(name, data, format.DefaultOptions)
		if err != nil {
			return nil, nil, err
		}
		return out, syntax, nil

	case from == FormatSysML && to == FormatTurtle:
		graph, err := sysmlToRDFWith(name, data, opts.ID)
		if err != nil {
			return nil, nil, err
		}
		return rdf.WriteTurtle(graph), nil, nil

	case from == FormatSysML && to == FormatAPIJSON:
		graph, err := sysmlToRDFWith(name, data, opts.ID)
		if err != nil {
			return nil, nil, err
		}
		out, err := export.WriteAPIJSON(graph)
		return out, nil, err

	case from == FormatAPIJSON:
		// The input is the API element form; api-json to api-json normalizes it
		// as Turtle to Turtle does.
		graph, err := readAPIJSON(name, data)
		if err != nil {
			return nil, nil, err
		}
		out, err := FromGraph(graph, to)
		return out, nil, err

	default:
		// The input is a graph: Turtle parsed, or one read from a repository.
		graph, err := rdf.ParseTurtle(data)
		if err != nil {
			return nil, nil, &SyntaxError{Name: name, Messages: []string{err.Error()}}
		}
		out, err := FromGraph(graph, to)
		return out, nil, err
	}
}

// FromGraph writes a graph in the to format, so a graph that did not come
// from a Turtle file — a repository branch read as RDF — converts alike.
func FromGraph(graph *rdf.Graph, to Format) ([]byte, error) {
	switch {
	case to == FormatSysML:
		return export.ToSysML(graph)
	case to == FormatTurtle:
		return rdf.WriteTurtle(graph), nil
	case to == FormatAPIJSON:
		return export.WriteAPIJSON(graph)
	default:
		return nil, &NotWritableError{Format: to}
	}
}

// readAPIJSON parses the API element form, reporting a malformed document as a
// syntax error of the input the way a Turtle parse failure is.
func readAPIJSON(name string, data []byte) (*rdf.Graph, error) {
	graph, err := export.ReadAPIJSON(data)
	if err != nil {
		return nil, &SyntaxError{Name: name, Messages: []string{err.Error()}}
	}
	return graph, nil
}

// SysMLToRDF parses SysML notation and converts it to a graph.
func SysMLToRDF(name string, data []byte) (*rdf.Graph, error) {
	return sysmlToRDFWith(name, data, export.IDQualifiedName)
}

// sysmlToRDFWith is SysMLToRDF under a non-default id form.
func sysmlToRDFWith(name string, data []byte, form export.IDForm) (*rdf.Graph, error) {
	file := source.New(name, data)
	p := parser.New(file)
	root := p.ParseFile()
	if err := syntaxError(name, file, p); err != nil {
		return nil, err
	}
	return export.ToRDFWith(file, root, form)
}

// checkSyntax reports the notation's syntax errors, if any.
func checkSyntax(name string, data []byte) *SyntaxError {
	file := source.New(name, data)
	p := parser.New(file)
	p.ParseFile()
	return syntaxError(name, file, p)
}

// syntaxError turns a parse's diagnostics into a SyntaxError, or nil when the
// input parsed clean.
func syntaxError(name string, file *source.SourceFile, p *parser.Parser) *SyntaxError {
	if len(p.Diagnostics) == 0 {
		return nil
	}
	lines := file.Lines()
	messages := make([]string, 0, len(p.Diagnostics))
	for _, diag := range p.Diagnostics {
		pos := lines.PosAt(diag.Span.Offset)
		messages = append(messages, fmt.Sprintf("%d:%d: %s", pos.Line, pos.Col, diag.Message))
	}
	return &SyntaxError{Name: name, Messages: messages, Diags: p.Diagnostics, File: file}
}
