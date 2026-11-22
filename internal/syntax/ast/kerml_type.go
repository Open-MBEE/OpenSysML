package ast

import (
	"maps"
	"slices"
)

// kermlTypeMetaclasses maps the kind keyword of a KerML type declaration to the
// metaclass it builds (KerML.xtext Type, Classifier and the rules specializing it).
var kermlTypeMetaclasses = map[string]string{
	"type":         "Type",
	"classifier":   "Classifier",
	"datatype":     "DataType",
	"class":        "Class",
	"struct":       "Structure",
	"assoc":        "Association",
	"assoc struct": "AssociationStructure",
	"behavior":     "Behavior",
	"function":     "Function",
	"predicate":    "Predicate",
	"interaction":  "Interaction",
	"metaclass":    "Metaclass",
}

// KerMLTypeMetaclass names the metaclass a KerML type declaration written with
// keyword builds, or "" for a keyword that declares no KerML type.
func KerMLTypeMetaclass(keyword string) string {
	return kermlTypeMetaclasses[keyword]
}

// KerMLTypeKeywords lists the kind keywords KerMLTypeMetaclass answers, sorted.
func KerMLTypeKeywords() []string {
	return slices.Sorted(maps.Keys(kermlTypeMetaclasses))
}
