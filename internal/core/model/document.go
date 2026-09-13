// Package model owns the workspace: the set of documents, the global symbol
// index, and the reindex pipeline that keeps them consistent.
package model

import (
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
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
	sf               *source.SourceFile
}

// newDocument parses content, which the workspace owns and never writes, and
// builds the document's local scope tree.
func newDocument(name string, content []byte, version int) *Document {
	sf := source.New(name, content)
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
		sf:               sf,
	}
}

// IsModelSource reports whether path is a SysML/KerML source file.
func IsModelSource(path string) bool {
	return strings.HasSuffix(path, ".sysml") || strings.HasSuffix(path, ".kerml")
}
