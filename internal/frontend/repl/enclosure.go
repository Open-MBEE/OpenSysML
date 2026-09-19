package repl

import (
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// probeName and probeDecl are appended to a submission to see whether it closes
// the text it opened. They are never accepted into the session.
const probeName = "__repl_encloses__"

const probeDecl = "\npart def " + probeName + ";\n"

// closesItsOwnText reports whether a declaration written after src is still read
// as one: text leaving a brace, comment or quoted name open absorbs what follows.
// doc is the session document the text parses in, which carries its kind.
func closesItsOwnText(doc, src string) bool {
	root := parser.New(source.New(doc, []byte(src+probeDecl))).ParseFile()
	for _, name := range declaredNames(root) {
		if name == probeName {
			return true
		}
	}
	return false
}

// maskedText blanks src byte for byte, so the buffer keeps every offset and line
// while parsing as if the text were absent.
func maskedText(src string) string {
	blanked := []byte(src)
	for i, b := range blanked {
		if b != '\n' && b != '\r' {
			blanked[i] = ' '
		}
	}
	return string(blanked)
}
