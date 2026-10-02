package ingest

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// Class is the kind of value a feature's declared type admits.
type Class int

const (
	// ClassUnknown follows what the cell is: a boolean, a number, else a string.
	ClassUnknown Class = iota
	ClassString
	ClassBoolean
	ClassInteger
	ClassReal
	// ClassEnum is an enumeration whose literals a cell names.
	ClassEnum
)

var (
	integerText = regexp.MustCompile(`^[+-]?[0-9]+$`)
	realText    = regexp.MustCompile(`^[+-]?([0-9]+(\.[0-9]*)?|\.[0-9]+)([eE][+-]?[0-9]+)?$`)
	unitText    = regexp.MustCompile(`^[^\[\];]+$`)
)

// Literal writes a cell as the notation of the value it assigns a feature of
// class; enum qualifies the literal an enumeration cell names.
func Literal(c Cell, class Class, enum string) (string, error) {
	switch c.Type {
	case "string":
		class = ClassString
	case "boolean":
		class = ClassBoolean
	case "integer":
		class = ClassInteger
	case "real", "number":
		class = ClassReal
	}
	if class == ClassUnknown {
		class = inferred(c)
	}
	if c.Unit != "" && class != ClassReal && class != ClassInteger {
		return "", fmt.Errorf("%s carries unit %s, but the value is not a number", c.Text, c.Unit)
	}
	var text string
	switch class {
	case ClassString:
		text = source.StringText(c.Text)
	case ClassBoolean:
		if c.Kind != KindText && c.Kind != KindBoolean {
			return "", fmt.Errorf("%s is not a boolean", c.Text)
		}
		switch strings.ToLower(c.Text) {
		case "true":
			text = "true"
		case "false":
			text = "false"
		default:
			return "", fmt.Errorf("%s is not a boolean", c.Text)
		}
	case ClassInteger:
		if (c.Kind != KindText && c.Kind != KindNumber) || !integerText.MatchString(c.Text) {
			return "", fmt.Errorf("%s is not an integer", c.Text)
		}
		text = strings.TrimPrefix(c.Text, "+")
	case ClassReal:
		number, err := realLiteral(c)
		if err != nil {
			return "", err
		}
		text = number
	case ClassEnum:
		if c.Kind != KindText && c.Kind != KindString {
			return "", fmt.Errorf("%s is not a literal of %s", c.Text, enum)
		}
		name := strings.TrimPrefix(c.Text, enum+"::")
		text = source.QualifiedNameText(enum) + "::" + source.NameText(name)
	}
	if c.Unit != "" {
		if !unitText.MatchString(c.Unit) {
			return "", fmt.Errorf("%s is not a unit", c.Unit)
		}
		text += " [" + c.Unit + "]"
	}
	return text, nil
}

func inferred(c Cell) Class {
	switch c.Kind {
	case KindString:
		return ClassString
	case KindBoolean:
		return ClassBoolean
	case KindNumber:
		if integerText.MatchString(c.Text) {
			return ClassInteger
		}
		return ClassReal
	}
	if strings.EqualFold(c.Text, "true") || strings.EqualFold(c.Text, "false") {
		return ClassBoolean
	}
	if integerText.MatchString(c.Text) {
		return ClassInteger
	}
	if f, err := strconv.ParseFloat(c.Text, 64); err == nil && !math.IsInf(f, 0) && !math.IsNaN(f) {
		return ClassReal
	}
	return ClassString
}

func realLiteral(c Cell) (string, error) {
	if c.Kind != KindText && c.Kind != KindNumber {
		return "", fmt.Errorf("%s is not a number", c.Text)
	}
	f, err := strconv.ParseFloat(c.Text, 64)
	if err != nil || math.IsInf(f, 0) || math.IsNaN(f) {
		return "", fmt.Errorf("%s is not a finite number", c.Text)
	}
	text := strings.TrimPrefix(c.Text, "+")
	if !realText.MatchString(text) {
		text = strconv.FormatFloat(f, 'g', -1, 64)
	}
	if strings.HasSuffix(text, ".") {
		text += "0"
	}
	return text, nil
}
