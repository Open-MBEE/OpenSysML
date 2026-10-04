// Package model owns the workspace: the set of documents, the global symbol
// index, and the reindex pipeline that keeps them consistent.
package model

import (
	"crypto/sha256"
	"encoding/hex"
	"slices"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

// Document is the parsed state of one source file. It is immutable once built:
// it is parsed from bytes the workspace owns, newDocument sets every field and
// nothing writes one afterwards, so a document the workspace hands out is a
// snapshot that neither a later update nor whoever supplied the bytes can touch.
type Document struct {
	Name             string
	Content          []byte
	Version          int
	AST              *ast.RootNamespace
	ParseDiagnostics []parser.Diagnostic
	ParseWarnings    []parser.Diagnostic
	Scope            *symbols.Scope
	kind             source.Kind
	sf               *source.SourceFile
	digest           string
	// recorded is set for a document installed from its interface record (see
	// Workspace.OpenRecorded): the diagnostics stored with the record, which the
	// document reports in place of an analysis. The record itself is not kept;
	// its facts live on the symbols of Scope. Content and sf are the text the
	// record was written from, so the diagnostics' spans locate in it.
	recorded *recordedDiagnostics
}

// recordedDiagnostics are what a recorded document reports: the diagnostics
// its analysis found when its record was written, verbatim, valid while
// nothing that analysis read has changed (see reads).
type recordedDiagnostics struct {
	diagnostics []diag.Diagnostic
	// provenance is what the recording analysis read of the index and who
	// answered: a change to one of those hydrates the document as it drops a
	// live frame, unless the same documents, unchanged, still answer.
	provenance *libs.Provenance
	// members are the document's top-level members as its record states them.
	members []libs.TopMember
}

// Recorded reports whether the document is held as its interface record: its
// scope tree carries facts in place of declarations, so AST is nil while
// Content, Digest and Lines are the text's.
func (d *Document) Recorded() bool {
	return d != nil && d.recorded != nil
}

// Kind reports the document's source kind, inferred from its name when none is stored.
func (d *Document) Kind() source.Kind {
	if d.kind != source.KindUnknown {
		return d.kind
	}
	return source.KindOf(d.Name)
}

// TopMembers reports the document's top-level members as written, from its
// tree when loaded and from its record when recorded.
func (d *Document) TopMembers() []libs.TopMember {
	if d.Recorded() {
		return slices.Clone(d.recorded.members)
	}
	return libs.TopMembers(d.AST)
}

// newDocument parses content, which the workspace owns and never writes, and
// builds the document's local scope tree.
func newDocument(name string, content []byte, version int, kind source.Kind) *Document {
	sf := newDocumentSource(name, content, kind)
	p := parser.New(sf)
	root := p.ParseFile()
	scope := symbols.Build(root)
	symbols.SetDocName(scope, name)
	return &Document{
		Name:             name,
		Content:          content,
		Version:          version,
		AST:              root,
		ParseDiagnostics: p.Diagnostics,
		ParseWarnings:    p.Warnings,
		Scope:            scope,
		kind:             sf.Kind(),
		sf:               sf,
		digest:           digestOf(content),
	}
}

func newDocumentSource(name string, content []byte, kind source.Kind) *source.SourceFile {
	if kind == source.KindUnknown {
		return source.New(name, content)
	}
	return source.NewWithKind(name, content, kind)
}

// Digest fingerprints the document's text: equal for equal content, so a
// position read from one snapshot can be checked against a later one.
func (d *Document) Digest() string {
	return d.digest
}

// Lines returns the document's line index, built once and cached.
func (d *Document) Lines() *source.LineIndex {
	return d.sf.Lines()
}

// digestOf is the first 16 bytes of the SHA-256 of content, in hex.
func digestOf(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:16])
}

// IsModelSource reports whether path is a SysML/KerML source file.
func IsModelSource(path string) bool {
	return strings.HasSuffix(path, ".sysml") || strings.HasSuffix(path, ".kerml")
}
