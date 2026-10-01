// Copyright 2025 Open‐MBEE Foundation. All rights reserved.
// Use of this source code is governed by the LICENSE file.

// Package semtok is the semantic-token vocabulary a classified document is
// written in: the LSP-standardized classes and modifiers, the lexical half of
// the classification (keywords, comments and literals, which need no symbol
// table), and the relative UTF-16 encoding the Language Server Protocol
// transmits them in.
package semtok

import (
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// Class is the kind of a semantic token. Its String is the LSP token type name.
type Class int

// The classes this package emits, in legend order.
const (
	ClassNamespace Class = iota
	ClassClass
	ClassEnum
	ClassInterface
	ClassStruct
	ClassParameter
	ClassVariable
	ClassProperty
	ClassEnumMember
	ClassFunction
	ClassMethod
	ClassKeyword
	ClassComment
	ClassString
	ClassNumber
)

var classNames = []string{
	ClassNamespace:  "namespace",
	ClassClass:      "class",
	ClassEnum:       "enum",
	ClassInterface:  "interface",
	ClassStruct:     "struct",
	ClassParameter:  "parameter",
	ClassVariable:   "variable",
	ClassProperty:   "property",
	ClassEnumMember: "enumMember",
	ClassFunction:   "function",
	ClassMethod:     "method",
	ClassKeyword:    "keyword",
	ClassComment:    "comment",
	ClassString:     "string",
	ClassNumber:     "number",
}

// String returns the LSP token type name of the class.
func (c Class) String() string {
	if int(c) < 0 || int(c) >= len(classNames) {
		return "unknown"
	}
	return classNames[c]
}

// Classes returns every class in legend order, so a class's index in the result
// is the token type index an encoded token carries.
func Classes() []Class {
	out := make([]Class, len(classNames))
	for i := range classNames {
		out[i] = Class(i)
	}
	return out
}

// Modifier is a bitset of token modifiers. Bit i is the modifier at index i of
// Modifiers.
type Modifier uint32

// The modifiers this package emits, in legend order.
const (
	ModDeclaration Modifier = 1 << iota
	ModDefinition
	ModReadonly
	ModAbstract
)

var modifierNames = []struct {
	mod  Modifier
	name string
}{
	{ModDeclaration, "declaration"},
	{ModDefinition, "definition"},
	{ModReadonly, "readonly"},
	{ModAbstract, "abstract"},
}

// Modifiers returns every modifier in legend order.
func Modifiers() []Modifier {
	out := make([]Modifier, len(modifierNames))
	for i, m := range modifierNames {
		out[i] = m.mod
	}
	return out
}

// String returns the LSP modifier name of a single modifier bit.
func (m Modifier) String() string {
	for _, entry := range modifierNames {
		if entry.mod == m {
			return entry.name
		}
	}
	return "unknown"
}

// Token is one classified span of source text.
type Token struct {
	Span      source.Span
	Class     Class
	Modifiers Modifier
}
