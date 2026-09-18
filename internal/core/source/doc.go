// Package source provides source file management and text indexing for the SysML v2 parser,
// and the notation's text forms that every layer above it reads and writes.
//
// This package defines SourceFile, which represents a single input file (either .sysml or .kerml),
// along with Span and Position types for tracking locations in source text. LineIndex provides
// efficient line/column lookups for error reporting and IDE features.
//
// It also holds the KerML and SysML keyword sets and the helpers that read and write
// notation text without a parse: names (NameText, QualifiedNameText), string values
// (StringValue, StringText) and comment bodies (CommentBody).
//
// All source locations are represented as byte offsets; LineIndex converts between offsets
// and user-facing line:column positions on demand.
package source
