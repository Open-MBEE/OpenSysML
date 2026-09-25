package source

import "strings"

// Kind is the language a file is written in. The two languages share a lexer
// but not a grammar: KerML notation in a SysML file is diagnosed.
type Kind int

const (
	// KindUnknown is a name with neither model extension.
	KindUnknown Kind = iota
	// KindSysML is a `.sysml` file, whose grammar is SysML.xtext.
	KindSysML
	// KindKerML is a `.kerml` file, whose grammar is KerML.xtext.
	KindKerML
)

// String returns the language's name.
func (k Kind) String() string {
	switch k {
	case KindSysML:
		return "SysML"
	case KindKerML:
		return "KerML"
	default:
		return "unknown"
	}
}

// KindOf reports the language of a file name by its extension.
func KindOf(name string) Kind {
	switch {
	case strings.HasSuffix(name, ".sysml"):
		return KindSysML
	case strings.HasSuffix(name, ".kerml"):
		return KindKerML
	default:
		return KindUnknown
	}
}

// Kind reports the language of this file.
func (sf *SourceFile) Kind() Kind { return sf.kind }
