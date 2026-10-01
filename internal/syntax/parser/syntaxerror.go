// Copyright 2025 Open‐MBEE Foundation. All rights reserved.
// Use of this source code is governed by the LICENSE file.

package parser

import (
	"fmt"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// SyntaxError reports that the input could not be read as its format. It lists
// every syntax error rather than only the first, so one conversion attempt
// shows everything that needs fixing.
type SyntaxError struct {
	Name     string
	Messages []string
	// Diags are the diagnostics behind Messages, for a caller that reports them
	// with their spans. Empty when the input is not notation, since a Turtle
	// reader reports a message and no span.
	Diags []Diagnostic
	// File is what Diags' spans point into; nil when Diags is empty.
	File *source.SourceFile
}

func (e *SyntaxError) Error() string {
	return fmt.Sprintf("%s: %d syntax error(s):\n  %s", e.Name, len(e.Messages), strings.Join(e.Messages, "\n  "))
}

// SyntaxErrorOf reports a document's parser diagnostics as a SyntaxError, or
// nil when there are none: a graph built from a tree the parser could not read
// whole would silently miss what it skipped.
func SyntaxErrorOf(name string, file *source.SourceFile, diags []Diagnostic) *SyntaxError {
	if len(diags) == 0 {
		return nil
	}
	lines := file.Lines()
	messages := make([]string, 0, len(diags))
	for _, diag := range diags {
		pos := lines.PosAt(diag.Span.Offset)
		messages = append(messages, fmt.Sprintf("%d:%d: %s", pos.Line, pos.Col, diag.Message))
	}
	return &SyntaxError{Name: name, Messages: messages, Diags: diags, File: file}
}
