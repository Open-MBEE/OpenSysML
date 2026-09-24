package migrate

import (
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/translate/xmi/sysmlv1"
)

// typeModifier is what MagicDraw's «typeModifier» on a property or parameter
// says of its v2 declaration: a collection shape, or that the usage is a reference.
type typeModifier struct {
	// text is the modifier as written: "[]", "[3]", "*", "&", "[][]"...
	text string
	// mult is the multiplicity the modifier gives, "[0..*]" or "[n]"; "" for a reference.
	mult string
	// ref is set when the usage is written as a reference.
	ref bool
	// refused is why the modifier has no v2 form; "" when it has one.
	refused string
}

// isTypeModifier matches the tool's exact «typeModifier» application.
func isTypeModifier(s *sysmlv1.Stereotype) bool {
	return s.Name == "typeModifier" && s.Namespace == sysmlv1.MagicDrawProfileNS
}

// readsTypeModifier reports whether the declaration of e reads application s
// as a «typeModifier»: it is written exactly, or kept as a comment with its own note.
func readsTypeModifier(e *sysmlv1.Element, s *sysmlv1.Stereotype) bool {
	return isTypeModifier(s) && (e.Type == "Property" || e.Type == "Parameter" || e.Type == "Port")
}

// writesTypeModifier reports whether application s on e is a «typeModifier»
// the declaration of e writes exactly, so that no comment repeats it.
func (m *migration) writesTypeModifier(e *sysmlv1.Element, s *sysmlv1.Stereotype) bool {
	if !readsTypeModifier(e, s) {
		return false
	}
	tm := m.typeModifier(e)
	return tm != nil && tm.refused == ""
}

// typeModifier reads the «typeModifier» applied to p, nil when none is.
func (m *migration) typeModifier(p *sysmlv1.Element) *typeModifier {
	var app *sysmlv1.Stereotype
	for _, s := range p.Stereotypes {
		if isTypeModifier(s) {
			app = s
			break
		}
	}
	if app == nil {
		return nil
	}
	tm := &typeModifier{text: strings.TrimSpace(app.Tag("typeModifier"))}
	switch shape, ok := strings.CutPrefix(tm.text, "["); {
	case tm.text == "":
		tm.refused = "it says nothing of the type"
	case tm.text == "*", tm.text == "&":
		tm.reference(m, p)
	case !ok:
		tm.refused = "the type modifier " + tm.text + " is not one the migrator reads"
	case strings.Count(tm.text, "[") > 1, strings.ContainsAny(shape, "*,"):
		tm.refused = "the type modifier " + tm.text + " has no v2 form: a multiplicity has one dimension"
	default:
		tm.collection(m, p, strings.TrimSuffix(shape, "]"))
	}
	return tm
}

// collection maps [] and [n] to a multiplicity, on a declaration of one value.
func (tm *typeModifier) collection(m *migration, p *sysmlv1.Element, n string) {
	if !strings.HasSuffix(tm.text, "]") || (n != "" && !isNatural(n)) {
		tm.refused = "the type modifier " + tm.text + " is not one the migrator reads"
		return
	}
	mult, note := m.declaredMultiplicity(p)
	if note != "" {
		tm.refused = "the type modifier " + tm.text + " has no v2 form: the declared " + note
		return
	}
	if mult != "" {
		tm.refused = "the type modifier " + tm.text + " has no v2 form: the declared multiplicity " + mult + " is already a collection, and a collection of collections has no multiplicity"
		return
	}
	if n == "" {
		tm.mult = "[0..*]"
	} else {
		tm.mult = "[" + n + "]"
	}
}

// reference maps * and & to a reference usage, on a property typed by a block.
func (tm *typeModifier) reference(m *migration, p *sysmlv1.Element) {
	if p.Type != "Property" || p.Parent == nil {
		tm.refused = "the type modifier " + tm.text + " has no v2 form: a parameter is not held by reference"
		return
	}
	cat, _ := m.classify(p.Parent)
	switch kw, _, _ := m.featureKeyword(p, cat); kw {
	case "part", "item":
		tm.ref = true
	default:
		tm.refused = "the type modifier " + tm.text + " has no v2 form: only a part or item is held by reference, not " + kwArticle(kw)
	}
}

// kwArticle names a usage keyword with its article.
func kwArticle(kw string) string {
	if strings.HasPrefix(kw, "a") || strings.HasPrefix(kw, "i") {
		return "an " + kw
	}
	return "a " + kw
}

// shape is the multiplicity the type modifier writes in place of the declared
// one, "" when it writes none: a v1 array is ordered and admits repeated values.
func (tm *typeModifier) shape() string {
	if tm == nil || tm.mult == "" {
		return ""
	}
	return tm.mult + " ordered nonunique"
}

// note is what the report says of the modifier: nothing when it is written
// exactly, else why it is kept as a comment.
func (tm *typeModifier) note() string {
	if tm == nil || tm.refused == "" {
		return ""
	}
	if tm.text == "" {
		return "an empty «typeModifier» is kept as a comment: " + tm.refused
	}
	return "«typeModifier» " + tm.text + " is kept as a comment: " + tm.refused
}
