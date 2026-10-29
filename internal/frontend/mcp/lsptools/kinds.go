package lsptools

import (
	"strconv"

	"go.lsp.dev/protocol"
)

// The protocol library's 1.0 kinds are bare integers; the tool results keep
// naming them as the LSP specification does.

var symbolKindNames = map[protocol.SymbolKind]string{
	protocol.SymbolKindFile:          "File",
	protocol.SymbolKindModule:        "Module",
	protocol.SymbolKindNamespace:     "Namespace",
	protocol.SymbolKindPackage:       "Package",
	protocol.SymbolKindClass:         "Class",
	protocol.SymbolKindMethod:        "Method",
	protocol.SymbolKindProperty:      "Property",
	protocol.SymbolKindField:         "Field",
	protocol.SymbolKindConstructor:   "Constructor",
	protocol.SymbolKindEnum:          "Enum",
	protocol.SymbolKindInterface:     "Interface",
	protocol.SymbolKindFunction:      "Function",
	protocol.SymbolKindVariable:      "Variable",
	protocol.SymbolKindConstant:      "Constant",
	protocol.SymbolKindString:        "String",
	protocol.SymbolKindNumber:        "Number",
	protocol.SymbolKindBoolean:       "Boolean",
	protocol.SymbolKindArray:         "Array",
	protocol.SymbolKindObject:        "Object",
	protocol.SymbolKindKey:           "Key",
	protocol.SymbolKindNull:          "Null",
	protocol.SymbolKindEnumMember:    "EnumMember",
	protocol.SymbolKindStruct:        "Struct",
	protocol.SymbolKindEvent:         "Event",
	protocol.SymbolKindOperator:      "Operator",
	protocol.SymbolKindTypeParameter: "TypeParameter",
}

var completionKindNames = map[protocol.CompletionItemKind]string{
	protocol.CompletionItemKindText:          "Text",
	protocol.CompletionItemKindMethod:        "Method",
	protocol.CompletionItemKindFunction:      "Function",
	protocol.CompletionItemKindConstructor:   "Constructor",
	protocol.CompletionItemKindField:         "Field",
	protocol.CompletionItemKindVariable:      "Variable",
	protocol.CompletionItemKindClass:         "Class",
	protocol.CompletionItemKindInterface:     "Interface",
	protocol.CompletionItemKindModule:        "Module",
	protocol.CompletionItemKindProperty:      "Property",
	protocol.CompletionItemKindUnit:          "Unit",
	protocol.CompletionItemKindValue:         "Value",
	protocol.CompletionItemKindEnum:          "Enum",
	protocol.CompletionItemKindKeyword:       "Keyword",
	protocol.CompletionItemKindSnippet:       "Snippet",
	protocol.CompletionItemKindColor:         "Color",
	protocol.CompletionItemKindFile:          "File",
	protocol.CompletionItemKindReference:     "Reference",
	protocol.CompletionItemKindFolder:        "Folder",
	protocol.CompletionItemKindEnumMember:    "EnumMember",
	protocol.CompletionItemKindConstant:      "Constant",
	protocol.CompletionItemKindStruct:        "Struct",
	protocol.CompletionItemKindEvent:         "Event",
	protocol.CompletionItemKindOperator:      "Operator",
	protocol.CompletionItemKindTypeParameter: "TypeParameter",
}

// symbolKindName names a symbol kind, by number when the specification has none.
func symbolKindName(k protocol.SymbolKind) string {
	if name, ok := symbolKindNames[k]; ok {
		return name
	}
	return strconv.Itoa(int(k))
}

// completionKindName names a completion kind, by number when the specification has none.
func completionKindName(k protocol.CompletionItemKind) string {
	if name, ok := completionKindNames[k]; ok {
		return name
	}
	return strconv.Itoa(int(k))
}

// optString is the value of an optional string, empty when absent.
func optString(o protocol.Optional[string]) string {
	v, _ := o.Get()
	return v
}

// stringArm is the text of a string-or-markup union, the markup's value when
// it is one.
func stringArm(v any) string {
	switch d := v.(type) {
	case protocol.String:
		return string(d)
	case *protocol.MarkupContent:
		if d != nil {
			return d.Value
		}
	}
	return ""
}
