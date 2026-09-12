package main

import (
	"fmt"
	"math/big"
	"regexp"
	"strconv"
	"strings"
)

const (
	kindInt      = "int"
	kindReal     = "real"
	kindBool     = "bool"
	kindString   = "string"
	kindInfinity = "infinity"
	kindSequence = "sequence"
	kindQuantity = "quantity"
	// kindUndetermined is our `<undetermined>`: a model-level result the model
	// leaves open, answered as such rather than evaluated or errored.
	kindUndetermined = "undetermined"
)

var (
	pilotUUID = regexp.MustCompile(` \([0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}\)$`)
	intText   = regexp.MustCompile(`^-?[0-9]+$`)
	realText  = regexp.MustCompile(`^-?[0-9]+\.[0-9]+$`)
	quantity  = regexp.MustCompile(`^(-?[0-9]+(?:\.[0-9]+)?) \[(.+)\]$`)
)

type normalized struct {
	Kind        string       `json:"kind"`
	Value       string       `json:"value,omitempty"`
	Elements    []normalized `json:"elements,omitempty"`
	Unevaluated bool         `json:"unevaluated,omitempty"`
}

type sideResult struct {
	Raw         string
	Value       normalized
	Error       bool
	PilotSilent bool
}

func normalizePilot(raw string) sideResult {
	lines := nonEmptyLines(raw)
	if len(lines) == 0 {
		return sideResult{Raw: raw, PilotSilent: true}
	}
	for _, line := range lines {
		if strings.HasPrefix(line, "ERROR:") || strings.HasPrefix(line, "EXCEPTION:") {
			return sideResult{Raw: raw, Error: true}
		}
	}

	var values []normalized
	for _, line := range lines {
		if strings.HasPrefix(line, "WARNING:") {
			continue
		}
		line = pilotUUID.ReplaceAllString(line, "")
		kind, value, ok := strings.Cut(line, " ")
		if !ok {
			return sideResult{Raw: raw, Value: normalized{Unevaluated: true}}
		}
		switch kind {
		case "LiteralInteger":
			if !intText.MatchString(value) {
				return sideResult{Raw: raw, Value: normalized{Unevaluated: true}}
			}
			values = append(values, normalized{Kind: kindInt, Value: value})
		case "LiteralRational", "LiteralReal":
			if _, ok := realRat(value); !ok {
				return sideResult{Raw: raw, Value: normalized{Unevaluated: true}}
			}
			values = append(values, normalized{Kind: kindReal, Value: value})
		case "LiteralBoolean":
			if value != "true" && value != "false" {
				return sideResult{Raw: raw, Value: normalized{Unevaluated: true}}
			}
			values = append(values, normalized{Kind: kindBool, Value: value})
		case "LiteralString":
			values = append(values, normalized{Kind: kindString, Value: value})
		case "LiteralInfinity":
			values = append(values, normalized{Kind: kindInfinity, Value: value})
		default:
			return sideResult{Raw: raw, Value: normalized{Unevaluated: true}}
		}
	}
	return sideResult{Raw: raw, Value: canonicalValue(sequenceOrSingle(values))}
}

func normalizeOurs(raw string, runErr bool) sideResult {
	if runErr {
		return sideResult{Raw: raw, Error: true}
	}
	for _, line := range strings.Split(raw, "\n") {
		if strings.HasPrefix(line, "sysml:") || strings.HasPrefix(line, "error:") {
			return sideResult{Raw: raw, Error: true}
		}
	}
	for _, line := range strings.Split(raw, "\n") {
		if strings.HasPrefix(line, "  = ") {
			value := strings.TrimSpace(strings.TrimPrefix(line, "  = "))
			parsed, ok := parseOurValue(value)
			if !ok {
				return sideResult{Raw: raw, Error: true}
			}
			return sideResult{Raw: raw, Value: canonicalValue(parsed)}
		}
	}
	return sideResult{Raw: raw, Value: normalized{}}
}

func parseOurValue(text string) (normalized, bool) {
	text = strings.TrimSpace(text)
	if text == "[]" {
		return normalized{Kind: kindSequence, Elements: []normalized{}}, true
	}
	if strings.HasPrefix(text, "[") && strings.HasSuffix(text, "]") {
		parts, ok := splitSequence(text[1 : len(text)-1])
		if !ok {
			return normalized{}, false
		}
		elements := make([]normalized, 0, len(parts))
		for _, part := range parts {
			value, ok := parseOurValue(part)
			if !ok {
				return normalized{}, false
			}
			elements = append(elements, value)
		}
		return normalized{Kind: kindSequence, Elements: elements}, true
	}
	if strings.HasPrefix(text, `"`) && strings.HasSuffix(text, `"`) {
		value, err := strconv.Unquote(text)
		return normalized{Kind: kindString, Value: value}, err == nil
	}
	if text == "true" || text == "false" {
		return normalized{Kind: kindBool, Value: text}, true
	}
	if text == "<undetermined>" {
		return normalized{Kind: kindUndetermined}, true
	}
	if quantity.MatchString(text) {
		return normalized{Kind: kindQuantity, Value: text}, true
	}
	if intText.MatchString(text) {
		return normalized{Kind: kindInt, Value: text}, true
	}
	if realText.MatchString(text) {
		return normalized{Kind: kindReal, Value: text}, true
	}
	return normalized{}, false
}

func splitSequence(text string) ([]string, bool) {
	if strings.TrimSpace(text) == "" {
		return []string{}, true
	}
	var parts []string
	start, depth := 0, 0
	inString, escaped := false, false
	for i, r := range text {
		if inString {
			if escaped {
				escaped = false
			} else if r == '\\' {
				escaped = true
			} else if r == '"' {
				inString = false
			}
			continue
		}
		switch r {
		case '"':
			inString = true
		case '[':
			depth++
		case ']':
			depth--
			if depth < 0 {
				return nil, false
			}
		case ',':
			if depth == 0 {
				parts = append(parts, strings.TrimSpace(text[start:i]))
				start = i + 1
			}
		}
	}
	if inString || depth != 0 {
		return nil, false
	}
	parts = append(parts, strings.TrimSpace(text[start:]))
	return parts, true
}

func sequenceOrSingle(values []normalized) normalized {
	switch len(values) {
	case 0:
		return normalized{}
	case 1:
		return values[0]
	default:
		return normalized{Kind: kindSequence, Elements: values}
	}
}

func canonicalValue(value normalized) normalized {
	if value.Kind != kindSequence {
		return value
	}
	if len(value.Elements) == 0 {
		return normalized{}
	}
	if len(value.Elements) == 1 {
		return canonicalValue(value.Elements[0])
	}
	for i, element := range value.Elements {
		value.Elements[i] = canonicalValue(element)
	}
	return value
}

func nonEmptyLines(raw string) []string {
	var lines []string
	for _, line := range strings.Split(raw, "\n") {
		if strings.TrimSpace(line) != "" {
			lines = append(lines, strings.TrimSpace(line))
		}
	}
	return lines
}

func realRat(text string) (*big.Rat, bool) {
	rat, ok := new(big.Rat).SetString(text)
	return rat, ok
}

func exactRat(value normalized) (*big.Rat, bool) {
	switch value.Kind {
	case kindInt:
		rat := new(big.Rat)
		if _, ok := rat.SetString(value.Value); ok {
			return rat, true
		}
	case kindReal:
		return realRat(value.Value)
	}
	return nil, false
}

func roundedReal(value normalized) string {
	rat, ok := exactRat(value)
	if !ok {
		return ""
	}
	float := new(big.Float).SetPrec(256).SetRat(rat)
	return float.Text('f', 2)
}

func compareValues(pilot, ours normalized) string {
	if pilot.Kind == kindSequence || ours.Kind == kindSequence {
		if pilot.Kind != kindSequence || ours.Kind != kindSequence {
			return "disagree"
		}
		if len(pilot.Elements) != len(ours.Elements) {
			return "disagree"
		}
		sameOrder, kindOnly := true, false
		for i := range pilot.Elements {
			bucket := compareValues(pilot.Elements[i], ours.Elements[i])
			if bucket == "disagree" {
				sameOrder = false
				break
			}
			if bucket == bucketKindOnly {
				kindOnly = true
			}
		}
		if sameOrder {
			if kindOnly {
				return bucketKindOnly
			}
			return "agree"
		}
		if sameMultiset(pilot.Elements, ours.Elements) {
			return "order-only"
		}
		return "disagree"
	}
	if pilot.Kind == kindQuantity || ours.Kind == kindQuantity {
		return "disagree"
	}
	if pilot.Kind == kindInt && ours.Kind == kindInt {
		if pilot.Value == ours.Value {
			return "agree"
		}
		return "disagree"
	}
	if pilot.Kind == kindReal && ours.Kind == kindReal {
		if roundedReal(pilot) == roundedReal(ours) {
			return "agree"
		}
		return "disagree"
	}
	if (pilot.Kind == kindInt && ours.Kind == kindReal) ||
		(pilot.Kind == kindReal && ours.Kind == kindInt) {
		pilotRat, pilotOK := exactRat(pilot)
		oursRat, oursOK := exactRat(ours)
		if pilotOK && oursOK && pilotRat.Cmp(oursRat) == 0 {
			return bucketKindOnly
		}
		return "disagree"
	}
	if pilot.Kind == ours.Kind && pilot.Value == ours.Value {
		return "agree"
	}
	return "disagree"
}

func sameMultiset(left, right []normalized) bool {
	used := make([]bool, len(right))
	for _, value := range left {
		found := false
		for i, candidate := range right {
			if !used[i] && compareValues(value, candidate) == "agree" {
				used[i], found = true, true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func bucketResults(pilot, ours sideResult) string {
	switch {
	case pilot.PilotSilent:
		return "pilot-silent"
	case pilot.Value.Unevaluated:
		return "pilot-unevaluated"
	case pilot.Error && ours.Error:
		return "both-error"
	case pilot.Error:
		return "pilot-error"
	case ours.Error:
		return "ours-error"
	case ours.Value.Kind == kindUndetermined:
		return "ours-undetermined"
	case pilot.Value.Kind == "" && ours.Value.Kind == "":
		return "agree"
	case pilot.Value.Kind == "" || ours.Value.Kind == "":
		return "disagree"
	default:
		return compareValues(pilot.Value, ours.Value)
	}
}

func normalizedText(value normalized) string {
	if value.Kind == "" {
		return "empty"
	}
	if value.Unevaluated {
		return "pilot-unevaluated"
	}
	if value.Kind == kindUndetermined {
		return kindUndetermined
	}
	if value.Kind == kindSequence {
		parts := make([]string, len(value.Elements))
		for i, element := range value.Elements {
			parts[i] = normalizedText(element)
		}
		return fmt.Sprintf("sequence[%s]", strings.Join(parts, ", "))
	}
	return value.Kind + ":" + value.Value
}
