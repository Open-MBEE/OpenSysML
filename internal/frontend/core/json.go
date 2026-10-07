// Copyright 2025 Open‐MBEE Foundation. All rights reserved.
// Use of this source code is governed by the LICENSE file.

package core

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"

	fsyntax "github.com/Open-MBEE/OpenSysML/internal/frontend/syntax"
)

// I64 is an int64 encoded as the protojson decimal string.
type I64 int64

// MarshalJSON writes the decimal string protojson writes for an int64.
func (v I64) MarshalJSON() ([]byte, error) {
	return []byte(strconv.Quote(strconv.FormatInt(int64(v), 10))), nil
}

// F64 is a protojson double, including protojson's non-finite spellings.
type F64 float64

// MarshalJSON writes a number, or protojson's string for a non-finite value.
func (v F64) MarshalJSON() ([]byte, error) {
	switch f := float64(v); {
	case math.IsNaN(f):
		return []byte(`"NaN"`), nil
	case math.IsInf(f, 1):
		return []byte(`"Infinity"`), nil
	case math.IsInf(f, -1):
		return []byte(`"-Infinity"`), nil
	default:
		return json.Marshal(f)
	}
}

// JSourceDocument is one document of a ParseSources request.
type JSourceDocument struct {
	FilePath *string `json:"filePath,omitempty"`
	Content  *string `json:"content,omitempty"`
	Language string  `json:"language,omitempty"`
	Name     string  `json:"name,omitempty"`
}

// UnmarshalJSON rejects a document naming both source arms.
func (d *JSourceDocument) UnmarshalJSON(data []byte) error {
	type alias JSourceDocument
	var a alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	*d = JSourceDocument(a)
	return oneofArms("source document", map[string]bool{
		"filePath": d.FilePath != nil,
		"content":  d.Content != nil,
	})
}

// JParseSourcesRequest is the protojson-shaped ParseSources request.
type JParseSourcesRequest struct {
	Documents         []*JSourceDocument `json:"documents,omitempty"`
	StrictConformance bool               `json:"strictConformance,omitempty"`
}

// JParseSourcesResponse is the protojson-shaped ParseSources response.
type JParseSourcesResponse struct {
	ModelHash   string         `json:"modelHash,omitempty"`
	Roots       []*JSymbolInfo `json:"roots,omitempty"`
	Diagnostics []*JDiagnostic `json:"diagnostics,omitempty"`
	Error       string         `json:"error,omitempty"`
}

// JParseFileRequest is the protojson-shaped ParseFile request.
type JParseFileRequest struct {
	FilePath          *string `json:"filePath,omitempty"`
	Content           *string `json:"content,omitempty"`
	ContentHash       string  `json:"contentHash,omitempty"`
	Language          string  `json:"language,omitempty"`
	StrictConformance bool    `json:"strictConformance,omitempty"`
}

// UnmarshalJSON rejects a ParseFile request naming both source arms.
func (r *JParseFileRequest) UnmarshalJSON(data []byte) error {
	type alias JParseFileRequest
	var a alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	*r = JParseFileRequest(a)
	return oneofArms("source", map[string]bool{
		"filePath": r.FilePath != nil,
		"content":  r.Content != nil,
	})
}

// JParseFileResponse is the protojson-shaped ParseFile response.
type JParseFileResponse struct {
	ModelHash   string         `json:"modelHash,omitempty"`
	Root        *JSymbolInfo   `json:"root,omitempty"`
	Diagnostics []*JDiagnostic `json:"diagnostics,omitempty"`
	Error       string         `json:"error,omitempty"`
}

// JGetSymbolRequest is the protojson-shaped GetSymbol request.
type JGetSymbolRequest struct {
	ModelHash string `json:"modelHash,omitempty"`
	SymbolId  string `json:"symbolId,omitempty"`
}

// JSymbolResponse is the protojson-shaped GetSymbol response.
type JSymbolResponse struct {
	Symbol *JSymbolInfo `json:"symbol,omitempty"`
	Error  string       `json:"error,omitempty"`
}

// JDiagnosticsRequest is the protojson-shaped GetDiagnostics request.
type JDiagnosticsRequest struct {
	ModelHash string `json:"modelHash,omitempty"`
}

// JDiagnosticsResponse is the protojson-shaped GetDiagnostics response.
type JDiagnosticsResponse struct {
	Diagnostics []*JDiagnostic `json:"diagnostics,omitempty"`
	Error       string         `json:"error,omitempty"`
}

// JDiagnostic is the Diagnostic message shared with the syntax frontend.
type JDiagnostic = fsyntax.Diagnostic

// JSymbolInfo is the protojson-shaped symbol facts message.
type JSymbolInfo struct {
	Id                        string             `json:"id,omitempty"`
	Name                      string             `json:"name,omitempty"`
	Kind                      string             `json:"kind,omitempty"`
	Metadata                  map[string]string  `json:"metadata,omitempty"`
	ChildIds                  []string           `json:"childIds,omitempty"`
	Attributes                []*JAttributeInfo  `json:"attributes,omitempty"`
	TypeInfo                  *JTypeInfo         `json:"typeInfo,omitempty"`
	Multiplicity              *JMultiplicityInfo `json:"multiplicity,omitempty"`
	Specializations           []*JSpecialization `json:"specializations,omitempty"`
	WithheldLibraryAttributes int32              `json:"withheldLibraryAttributes,omitempty"`
}

// JTypeInfo is the protojson-shaped static type message.
type JTypeInfo struct {
	Declared        string `json:"declared,omitempty"`
	ResolvedId      string `json:"resolvedId,omitempty"`
	ResolvedKind    string `json:"resolvedKind,omitempty"`
	Primitive       string `json:"primitive,omitempty"`
	PrimitiveSource string `json:"primitiveSource,omitempty"`
	Quantity        bool   `json:"quantity,omitempty"`
	Unit            string `json:"unit,omitempty"`
}

// JMultiplicityInfo contains protojson-shaped lower and upper bounds.
type JMultiplicityInfo struct {
	Lower string `json:"lower,omitempty"`
	Upper string `json:"upper,omitempty"`
}

// JSpecialization is one protojson-shaped generalization relationship.
type JSpecialization struct {
	Kind       string `json:"kind,omitempty"`
	Declared   string `json:"declared,omitempty"`
	TargetId   string `json:"targetId,omitempty"`
	TargetKind string `json:"targetKind,omitempty"`
}

// JAttributeInfo is the protojson-shaped attribute facts message.
type JAttributeInfo struct {
	Name  string  `json:"name,omitempty"`
	Type  string  `json:"type,omitempty"`
	Value *JValue `json:"value,omitempty"`
	Unit  string  `json:"unit,omitempty"`
}

// JValue holds the constant attribute value arms the core can derive.
type JValue struct {
	IntValue    *I64    `json:"intValue,omitempty"`
	RealValue   *F64    `json:"realValue,omitempty"`
	BoolValue   *bool   `json:"boolValue,omitempty"`
	StringValue *string `json:"stringValue,omitempty"`
	Null        *string `json:"null,omitempty"`
	Infinity    *bool   `json:"infinity,omitempty"`
	// RationalValue is an exact Rational binary64 does not hold exactly.
	RationalValue *JRational `json:"rationalValue,omitempty"`
}

// JRational is an exact Rational in lowest terms, its denominator positive.
type JRational struct {
	Numerator   string `json:"numerator,omitempty"`
	Denominator string `json:"denominator,omitempty"`
}

func oneofArms(message string, arms map[string]bool) error {
	var set []string
	for name, present := range arms {
		if present {
			set = append(set, name)
		}
	}
	if len(set) > 1 {
		sort.Strings(set)
		return fmt.Errorf("%s sets both %s and %s", message, set[0], set[1])
	}
	return nil
}
