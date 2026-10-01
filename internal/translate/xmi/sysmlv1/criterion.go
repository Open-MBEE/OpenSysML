package sysmlv1

import (
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/translate/xmi"
)

// criterion decodes one structured expression as MagicDraw serializes a
// matrix's dependency criterion or a relation map's relation criterion: an
// XML document, escaped into the tag's text, whose root call applies one
// expression to the THIS argument.
func (m *Model) criterion(raw string) Criterion {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return Criterion{Kind: CriterionOther, Malformed: "empty"}
	}
	doc, err := xmi.Parse(strings.NewReader(raw))
	if err != nil {
		return Criterion{Kind: CriterionOther, Malformed: "not well-formed XML: " + err.Error()}
	}
	call := doc.Root
	c := Criterion{Name: taggedValue(call, "name")}
	expr := call.First("expression")
	if expr == nil {
		c.Kind, c.Malformed = CriterionOther, "no expression element"
		return c
	}
	c.Expression = expr.Attr("type")
	c.Direction = expr.Attr("direction")
	c.IncludeSubtypes = expr.Attr("includeSubtypes") == "true"
	switch c.Expression {
	case "dslRelationExpressionSpecification":
		c.Kind = CriterionRelation
		if id := expr.Attr("stereotype"); id != "" {
			c.Stereotype = m.StereotypeRef(id)
		} else {
			c.Malformed = "relation expression names no stereotype"
		}
	case "relationExpressionSpecification":
		c.Kind = CriterionRelation
		if c.Metaclass = expr.Attr("metaclass"); c.Metaclass == "" {
			c.Malformed = "relation expression names no metaclass"
		}
	case "metaChainExpressionSpecification":
		c.Kind = CriterionMetachain
		var steps []string
		for _, step := range expr.Tagged("chain") {
			steps = append(steps, chainStep(step))
		}
		c.Detail = strings.Join(steps, ".")
	case "propertyExpressionSpecification":
		c.Kind = CriterionProperty
		if p := expr.First("property"); p != nil {
			c.Detail = chainStep(p)
		}
	case "makeInlineExpressionSpecification":
		c.Kind = CriterionScript
		if body := expr.First("body"); body != nil {
			c.Detail = strings.TrimSpace(body.Text)
		}
		if lang := expr.Attr("language"); lang != "" {
			c.Detail = lang + ": " + c.Detail
		}
	case "":
		c.Kind, c.Malformed = CriterionOther, "expression has no type"
	default:
		c.Kind = CriterionOther
	}
	return c
}

// taggedValue reads one entry of a structured expression's taggedValues.
func taggedValue(e *xmi.Element, key string) string {
	values := e.First("taggedValues")
	if values == nil {
		return ""
	}
	for _, entry := range values.Tagged("entry") {
		if entry.Attr("key") == key {
			if v := entry.First("value"); v != nil {
				return strings.TrimSpace(v.Text)
			}
		}
	}
	return ""
}

// chainStep spells one metachain step: Metaclass.property for a UML property,
// «Stereotype».tag for a stereotype property.
func chainStep(step *xmi.Element) string {
	if step.Attr("type") == "stereotypeProperty" {
		return "«" + step.Attr("stereotype") + "»." + step.Attr("tag")
	}
	return step.Attr("metaclass") + "." + step.Attr("property")
}
