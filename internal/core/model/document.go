// Package model owns the workspace: the set of documents, the global symbol
// index, and the reindex pipeline that keeps them consistent.
package model

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
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
	digest           string
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
		digest:           digestOf(content),
	}
}

// Digest fingerprints the document's text: equal for equal content, so a
// position read from one snapshot can be checked against a later one.
func (d *Document) Digest() string {
	return d.digest
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
