// Package export saves a SysML v2 model to a file and converts between the
// two representations OpenSysML can write: SysML textual notation and RDF
// Turtle. It also reads a third, SysML v1 in XMI, which is migrated to v2
// notation on the way in.
//
// # SysML output
//
// Saving a model that came from source writes that source, re-indented by
// internal/core/format. Printing the AST instead would drop comments, notes and
// anything the parser recorded as an ErrorNode, so a save has to keep the token
// stream (see the format package doc).
//
// # RDF output
//
// The graph uses the SysML vocabulary and element IRIs of the Flexo MMS SysML
// v2 service (https://www.omg.org/spec/SysML# and urn:sysmlv2:element:), so a
// converted model loads into that service's triplestore. Elements are addressed
// by qualified name (`urn:sysmlv2:element:Demo::Vehicle`), which makes the IRIs
// stable across conversions of the same model rather than newly generated each
// time.
//
// Each element carries its metaclass as rdf:type and its declaration as SysML
// metamodel properties: declaredName, declaredShortName, owningNamespace,
// visibility, direction, the feature flags, the typing and specialization
// clauses, multiplicity bounds and its value; expression-valued positions are
// expression trees. Properties the metamodel does not define live in a separate
// urn:opensysml:sysml: namespace so a consumer can tell them from the standard
// vocabulary: memberIndex (declaration order, which the notation is sensitive to
// and RDF is not), hasBody, the end forms of heads that bind ends, and each
// element's lines as written, comments included, as sourceText and sourceTail.
//
// # XMI input
//
// SysML v1 as UML XMI (OMG XMI 2.5.1, or the Eclipse UML2 .uml serialization
// Papyrus writes) is migrated to v2 notation by internal/core/migrate, then
// takes the notation path. XMI is never written.
//
// # RDF back to notation
//
// The structural triples are authoritative. An element is written from its
// source text while that text still states what the graph states — so comments
// and layout survive a round trip of an unedited model — and canonically where
// the two disagree or the graph carries no text, as one from another tool does
// not. docs/reference/rdf-mapping.md documents the mapping and its limitations.
package export

import (
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/format"
	"github.com/Open-MBEE/OpenSysML/internal/core/lexer"
	"github.com/Open-MBEE/OpenSysML/internal/core/migrate"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/rdf"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

// Format is one of the representations a model can be read from or written to.
type Format int

const (
	// FormatSysML is SysML v2 / KerML textual notation.
	FormatSysML Format = iota
	// FormatTurtle is RDF in Turtle syntax.
	FormatTurtle
	// FormatXMI is SysML v1 as UML XMI 2.5.1, the OMG SysML profile applied. It
	// is an input format only.
	FormatXMI
)

func (f Format) String() string {
	switch f {
	case FormatTurtle:
		return "ttl"
	case FormatXMI:
		return "xmi"
	}
	return "sysml"
}

// Writable reports whether models can be written in the format.
func (f Format) Writable() bool {
	return f != FormatXMI
}

// formatNames are the names accepted on the command line for each format.
var formatNames = map[string]Format{
	"sysml":  FormatSysML,
	"kerml":  FormatSysML,
	"text":   FormatSysML,
	"ttl":    FormatTurtle,
	"turtle": FormatTurtle,
	"rdf":    FormatTurtle,
	"xmi":    FormatXMI,
	"uml":    FormatXMI,
}

// FormatList is the wording every surface lists the format names in.
const FormatList = "sysml, kerml, ttl, turtle, rdf, or xmi/uml (input only)"

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
	reason := "expected .sysml, .kerml or .ttl"
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
const ExtensionAdvice = "name the file with a .sysml, .kerml or .ttl extension"

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
	case ".xmi", ".uml":
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

// Convert reads data in the from format and writes it in the to format. name is
// used in diagnostics and needs no relation to a file on disk.
//
// Converting between two formats requires the input to be syntactically valid:
// a model with syntax errors is rejected, because the tree the parser recovers
// from broken input does not hold the declarations it could not read, and a
// graph built from it would be quietly missing them.
func Convert(name string, data []byte, from, to Format) ([]byte, error) {
	out, _, err := convert(name, data, from, to, false)
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
	return convert(name, data, from, to, true)
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
	return convert(file.Name(), []byte(text), FormatSysML, FormatSysML, true)
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

// Migrate reads a SysML v1 model in XMI and writes it in the to format, with
// the report of what each v1 element became.
func Migrate(name string, data []byte, to Format) ([]byte, *migrate.Report, error) {
	if !to.Writable() {
		return nil, nil, &NotWritableError{Format: to}
	}
	result, err := migrate.Migrate(name, data)
	if err != nil {
		return nil, nil, fmt.Errorf("%s: %w", name, err)
	}
	out, _, err := convert(name+".sysml", result.Notation, FormatSysML, to, false)
	if err != nil {
		return nil, nil, fmt.Errorf("the migrated notation could not be written: %w", err)
	}
	return out, result.Report, nil
}

func convert(name string, data []byte, from, to Format, tolerateSyntaxErrors bool) ([]byte, *SyntaxError, error) {
	switch {
	case !to.Writable():
		return nil, nil, &NotWritableError{Format: to}

	case from == FormatXMI:
		out, _, err := Migrate(name, data, to)
		return out, nil, err

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
		graph, err := SysMLToRDF(name, data)
		if err != nil {
			return nil, nil, err
		}
		return rdf.WriteTurtle(graph), nil, nil

	case from == FormatTurtle && to == FormatSysML:
		graph, err := rdf.ParseTurtle(data)
		if err != nil {
			return nil, nil, &SyntaxError{Name: name, Messages: []string{err.Error()}}
		}
		out, err := ToSysML(graph)
		return out, nil, err

	default:
		// Turtle to Turtle: read and rewrite, which normalizes the document
		// and reports anything the reader cannot represent.
		graph, err := rdf.ParseTurtle(data)
		if err != nil {
			return nil, nil, &SyntaxError{Name: name, Messages: []string{err.Error()}}
		}
		return rdf.WriteTurtle(graph), nil, nil
	}
}

// SysMLToRDF parses SysML notation and converts it to a graph.
func SysMLToRDF(name string, data []byte) (*rdf.Graph, error) {
	file := source.New(name, data)
	p := parser.New(file)
	root := p.ParseFile()
	if err := syntaxError(name, file, p); err != nil {
		return nil, err
	}
	return ToRDF(file, root)
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
