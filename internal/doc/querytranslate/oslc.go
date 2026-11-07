package querytranslate

import (
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/query"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/suggest"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

// Query parameters this package names in more than one place.
const (
	paramWhere      = "oslc.where"
	paramSelect     = "oslc.select"
	paramOrderBy    = "oslc.orderBy"
	paramPrefix     = "oslc.prefix"
	paramProperties = "oslc.properties"
	paramSearch     = "oslc.searchTerms"
)

// oslcParameters is the closed set of query parameters this implementation
// reads. Ignoring any other one would answer a different query than the one
// asked: a misspelt oslc.where would select the whole model rather than fail.
var oslcParameters = map[string]bool{
	paramWhere: true, paramSelect: true, paramOrderBy: true,
	paramPrefix: true, paramProperties: true, paramSearch: true,
}

// ParameterNames returns the query parameters this implementation reads, in
// stable order. It is what an unknown-parameter error lists.
func ParameterNames() []string {
	out := make([]string, 0, len(oslcParameters))
	for name := range oslcParameters {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// DefaultPrefixes are the prefixes available without an oslc.prefix binding:
// the ones the RDF conversion writes on every graph.
var DefaultPrefixes = rdf.DefaultPrefixes

// SelectAll is the wildcard oslc.select spells every property with.
const SelectAll = "*"

// ParseOSLC parses a where expression against the model's property table. It
// also accepts a query-parameter string, allowing callers to pass an HTTP-style
// OSLC query directly.
func ParseOSLC(text string, table *Table) (Query, error) {
	if isParameterText(text) {
		return ParseParameters(text, table)
	}
	p := oslcParser{s: text, prefixes: clonePrefixes(DefaultPrefixes), table: table}
	where, err := p.parseCompound()
	if err != nil {
		return Query{}, err
	}
	return Query{Where: where}, nil
}

func isParameterText(text string) bool {
	if strings.HasPrefix(strings.TrimSpace(text), "oslc.") {
		return true
	}
	for _, part := range strings.Split(text, "&") {
		if strings.HasPrefix(strings.TrimSpace(part), "oslc.") {
			return true
		}
	}
	return false
}

// ParseParameters parses oslc.where, oslc.select, oslc.orderBy, oslc.searchTerms
// and oslc.prefix parameters. Parameter text follows URL query decoding rules.
func ParseParameters(text string, table *Table) (Query, error) {
	values, err := url.ParseQuery(text)
	if err != nil {
		return Query{}, errorf(ErrMalformed, "malformed OSLC query parameters: %v", err)
	}
	if err := checkParameters(values); err != nil {
		return Query{}, err
	}
	prefixes := clonePrefixes(DefaultPrefixes)
	if prefixText := values.Get(paramPrefix); prefixText != "" {
		if err := parsePrefixes(prefixText, prefixes); err != nil {
			return Query{}, err
		}
	}
	if properties, present := values[paramProperties]; present && len(properties) > 0 {
		return Query{}, errorf(ErrMalformed, "oslc.properties is not implemented: name the properties to report with %s", paramSelect)
	}
	q := Query{}
	if search, present := values[paramSearch]; present {
		terms, err := parseSearchTerms(search[0])
		if err != nil {
			return Query{}, err
		}
		q.SearchTerms = terms
	}
	if order := values.Get(paramOrderBy); order != "" {
		terms, err := parseOrder(order, prefixes, table)
		if err != nil {
			return Query{}, err
		}
		q.OrderBy = terms
	}
	if where, present := values[paramWhere]; present {
		p := oslcParser{s: where[0], prefixes: prefixes, table: table}
		parsed, err := p.parseCompound()
		if err != nil {
			return Query{}, err
		}
		q.Where = parsed
	}
	if selectText := values.Get(paramSelect); selectText != "" {
		p := oslcParser{s: selectText, prefixes: prefixes, table: table}
		selections, spelling, err := p.parseSelection()
		if err != nil {
			return Query{}, err
		}
		q.Select, q.Spelling = selections, spelling
	}
	return q, nil
}

// omitting names, per parameter, what leaving it out asks for: an empty value
// is a misuse, since the parameter as written narrows nothing.
var omitting = map[string]string{
	paramWhere:   "select every element",
	paramSelect:  "report every property",
	paramOrderBy: "keep declaration order",
	paramPrefix:  "use the default prefixes",
	paramSearch:  "search no text",
}

// checkParameters refuses a parameter this implementation does not read, one
// given more than once (reading only the first would discard the rest), and
// one given no value.
func checkParameters(values url.Values) error {
	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if !oslcParameters[name] {
			return errorf(ErrMalformed, "unknown OSLC query parameter %q; this implementation reads %s",
				name, strings.Join(ParameterNames(), ", "))
		}
		if len(values[name]) > 1 {
			return errorf(ErrMalformed, "OSLC query parameter %q is given %d times", name, len(values[name]))
		}
		if omit, read := omitting[name]; read && strings.TrimSpace(values[name][0]) == "" {
			return errorf(ErrMalformed, "%s is empty: omit it to %s", name, omit)
		}
	}
	return nil
}

func parsePrefixes(text string, prefixes map[string]string) error {
	bindings, err := splitCommaStrict(text, paramPrefix)
	if err != nil {
		return err
	}
	for _, binding := range bindings {
		name, iri, ok := strings.Cut(strings.TrimSpace(binding), "=")
		if !ok {
			return errorf(ErrMalformed, "malformed oslc.prefix binding %q", binding)
		}
		iri = strings.TrimSpace(iri)
		if len(iri) < 2 || iri[0] != '<' || iri[len(iri)-1] != '>' {
			return errorf(ErrMalformed, "oslc.prefix binding %q must use an IRI", binding)
		}
		prefix := strings.TrimSuffix(strings.TrimSpace(name), ":")
		if err := checkPrefixName(prefix); err != nil {
			return err
		}
		prefixes[prefix] = iri[1 : len(iri)-1]
	}
	return nil
}

// checkPrefixName refuses a prefix no prefixed name could be written with, so
// every binding a diagnostic offers is one the parser reads back.
func checkPrefixName(prefix string) error {
	if prefix == "" {
		return errorf(ErrMalformed, "oslc.prefix binding has an empty prefix")
	}
	for _, r := range prefix {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || strings.ContainsRune("_-.", r) {
			continue
		}
		return errorf(ErrMalformed,
			"oslc.prefix %q contains %q, which no prefixed name can be written with", prefix, r)
	}
	return nil
}

// parseSearchTerms reads oslc.searchTerms: quoted strings separated by commas.
func parseSearchTerms(text string) ([]string, error) {
	p := oslcParser{s: text}
	var terms []string
	for {
		p.skipSpace()
		if !p.peek('"') {
			return nil, errorf(ErrMalformed, "%s is a comma-separated list of quoted strings", paramSearch)
		}
		term, err := p.value()
		if err != nil {
			return nil, err
		}
		if term == "" {
			return nil, errorf(ErrMalformed, "%s contains an empty term", paramSearch)
		}
		terms = append(terms, term)
		p.skipSpace()
		if p.pos >= len(p.s) {
			return terms, nil
		}
		if !p.consume(',') {
			return nil, errorf(ErrMalformed, "%s terms are separated by ','", paramSearch)
		}
	}
}

func parseOrder(text string, prefixes map[string]string, table *Table) ([]OrderTerm, error) {
	var out []OrderTerm
	terms, err := splitCommaStrict(text, paramOrderBy)
	if err != nil {
		return nil, err
	}
	for _, raw := range terms {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			return nil, errorf(ErrMalformed, "oslc.orderBy contains an empty term")
		}
		desc := false
		if raw[0] == '+' || raw[0] == '-' {
			desc, raw = raw[0] == '-', strings.TrimSpace(raw[1:])
		}
		if raw == SelectAll {
			return nil, errorf(ErrMalformed, "oslc.orderBy sorts by named properties; '*' names none")
		}
		p := oslcParser{s: raw, prefixes: prefixes, table: table}
		name, err := p.parsePath()
		if err != nil {
			return nil, err
		}
		p.skipSpace()
		if p.pos < len(p.s) {
			return nil, errorf(ErrMalformed, "oslc.orderBy term %q has trailing text", raw)
		}
		out = append(out, OrderTerm{Property: name, Desc: desc})
	}
	return out, nil
}

type oslcParser struct {
	s        string
	pos      int
	prefixes map[string]string
	table    *Table
}

// parseCompound reads `term (and term)*`, a scoped term's body included.
func (p *oslcParser) parseCompound() (Constraint, error) {
	var terms []Constraint
	for {
		p.skipSpace()
		if p.pos >= len(p.s) {
			if len(terms) == 0 {
				return nil, errorf(ErrMalformed, "OSLC compound term is empty")
			}
			break
		}
		term, err := p.parseTerm()
		if err != nil {
			return nil, err
		}
		terms = append(terms, term)
		p.skipSpace()
		if p.pos >= len(p.s) || p.peek('}') {
			break
		}
		if p.consumeWord("or") {
			return nil, errorf(ErrUnsupportedDisjunction,
				"OSLC Query has no \"or\": compound terms are joined by \" and \"; ask a separate query for each alternative")
		}
		if !p.consumeWord("and") {
			return nil, errorf(ErrMalformed, "OSLC compound terms may be joined only by \" and \"")
		}
		p.skipSpace()
		if p.pos >= len(p.s) || p.peek('}') {
			return nil, errorf(ErrMalformed, "OSLC compound term has a trailing \"and\"")
		}
	}
	if len(terms) == 1 {
		return terms[0], nil
	}
	return Conjunction{Terms: terms}, nil
}

// parseTerm reads one term: a comparison, an `in` list, or a scoped term
// `property{compound}`.
func (p *oslcParser) parseTerm() (Constraint, error) {
	identifier, err := p.identifier()
	if err != nil {
		return nil, err
	}
	p.skipSpace()
	if p.consume('{') {
		property, err := p.resolveProperty(identifier)
		if err != nil {
			return nil, err
		}
		if property == query.PropertyType {
			return nil, errorf(ErrMalformed, "rdf:type names a metaclass, not an element with properties; it cannot be scoped")
		}
		inner, err := p.parseCompound()
		if err != nil {
			return nil, err
		}
		p.skipSpace()
		if !p.consume('}') {
			return nil, errorf(ErrMalformed, "OSLC scoped term for %q is not closed by '}'", identifier)
		}
		return Scoped{Property: property, Inner: inner}, nil
	}
	property, err := p.resolveProperty(identifier)
	if err != nil {
		return nil, err
	}
	if p.consumeWord("in") {
		p.skipSpace()
		if !p.consume('[') {
			return nil, errorf(ErrMalformed, "OSLC in term for %q must contain a value list", identifier)
		}
		var vals []string
		for {
			p.skipSpace()
			value, err := p.value()
			if err != nil {
				return nil, err
			}
			vals = append(vals, value)
			p.skipSpace()
			if p.consume(']') {
				break
			}
			if !p.consume(',') {
				return nil, errorf(ErrMalformed, "OSLC in term for %q needs ',' or ']'", identifier)
			}
		}
		return p.term(property, In, vals)
	}
	op := p.operator()
	if op == "" {
		return nil, errorf(ErrMalformed, "OSLC term for %q has no comparison operator", identifier)
	}
	p.skipSpace()
	value, err := p.value()
	if err != nil {
		return nil, err
	}
	return p.term(property, op, []string{value})
}

// term is the constraint a comparison of property denotes: a type test or an
// instance selection for rdf:type, a comparison for anything else.
func (p *oslcParser) term(property string, op Operator, values []string) (Constraint, error) {
	if property != query.PropertyType {
		return Comparison{Property: property, Operator: op, Values: values}, nil
	}
	switch op {
	case Equal, In, NotEqual:
	default:
		return nil, errorf(ErrUnorderedProperty, "rdf:type names a metaclass, which %q cannot order", op)
	}
	var metaclasses, instances []string
	for _, value := range values {
		if name, ok := elementName(value); ok {
			instances = append(instances, name)
			continue
		}
		metaclasses = append(metaclasses, value)
	}
	if len(instances) > 0 && len(metaclasses) > 0 {
		return nil, errorf(ErrMalformed, "rdf:type names either metaclasses (sysml:) or elements (elmt:), not both in one term")
	}
	if len(instances) > 0 {
		if op == NotEqual {
			return nil, errorf(ErrUnsupportedInstanceTerm, "rdf:type!=elmt:… selects nothing definite: name the element whose objects to select with '='")
		}
		return Instances{Elements: instances}, nil
	}
	return TypeTest{Types: metaclasses, Exact: true, Inverse: op == NotEqual}, nil
}

// elementName decodes an element IRI of the RDF conversion to the qualified
// name of the element.
func elementName(value string) (string, bool) {
	if !strings.HasPrefix(value, rdf.Element) {
		return "", false
	}
	return rdf.DecodeElementID(strings.TrimPrefix(value, rdf.Element))
}

// parseSelection reads oslc.select: properties, nested selections
// `property{selection}`, and the `*` wildcard, with the spelling of each column.
func (p *oslcParser) parseSelection() ([]Selection, map[string]string, error) {
	spelling := map[string]string{}
	selections, err := p.selectionList("", spelling, func(spelled string) string { return spelled })
	if err != nil {
		return nil, nil, err
	}
	p.skipSpace()
	if p.pos < len(p.s) {
		return nil, nil, errorf(ErrMalformed, "%s has trailing text at %q", paramSelect, p.s[p.pos:])
	}
	return selections, spelling, nil
}

func (p *oslcParser) selectionList(prefix string, spelling map[string]string, wrap func(string) string) ([]Selection, error) {
	var out []Selection
	seen := map[string]string{}
	for {
		p.skipSpace()
		if p.pos >= len(p.s) || p.peek('}') {
			if len(out) == 0 {
				return nil, errorf(ErrMalformed, "%s contains an empty term", paramSelect)
			}
			return out, nil
		}
		identifier, err := p.identifier()
		if err != nil {
			return nil, err
		}
		spelled := identifier
		var selection Selection
		if identifier == SelectAll {
			selection = Selection{Property: SelectAll}
		} else {
			property, err := p.resolveProperty(identifier)
			if err != nil {
				return nil, err
			}
			selection = Selection{Property: property}
		}
		p.skipSpace()
		if p.consume('{') {
			if selection.Property == SelectAll {
				return nil, errorf(ErrMalformed, "the %s wildcard '*' cannot be scoped", paramSelect)
			}
			nested, err := p.selectionList(prefix+selection.Property+".", spelling, func(spelled string) string {
				return wrap(identifier + "{" + spelled + "}")
			})
			if err != nil {
				return nil, err
			}
			p.skipSpace()
			if !p.consume('}') {
				return nil, errorf(ErrMalformed, "%s nested selection for %q is not closed by '}'", paramSelect, identifier)
			}
			selection.Nested = nested
		}
		if first, repeated := seen[selection.Property]; repeated {
			if first == identifier {
				return nil, errorf(ErrMalformed, "%s names %q twice", paramSelect, identifier)
			}
			return nil, errorf(ErrMalformed, "%s names one property twice: %q and %q are the same property",
				paramSelect, first, identifier)
		}
		seen[selection.Property] = identifier
		if len(selection.Nested) == 0 {
			spelling[prefix+selection.Property] = wrap(spelled)
		}
		out = append(out, selection)
		p.skipSpace()
		if p.pos >= len(p.s) || p.peek('}') {
			return out, nil
		}
		if !p.consume(',') {
			return nil, errorf(ErrMalformed, "%s terms are separated by ','", paramSelect)
		}
		p.skipSpace()
		if p.pos >= len(p.s) || p.peek('}') {
			return nil, errorf(ErrMalformed, "%s contains an empty term", paramSelect)
		}
	}
}

// parsePath reads a property, or a nested one `property{property}`, as a
// dotted path, for oslc.orderBy.
func (p *oslcParser) parsePath() (string, error) {
	identifier, err := p.identifier()
	if err != nil {
		return "", err
	}
	if identifier == SelectAll {
		return "", errorf(ErrMalformed, "oslc.orderBy sorts by named properties; '*' names none")
	}
	property, err := p.resolveProperty(identifier)
	if err != nil {
		return "", err
	}
	if property == query.PropertyType {
		return "", errorf(ErrMalformed, "rdf:type names a metaclass; oslc.orderBy sorts by a property's values")
	}
	p.skipSpace()
	if !p.consume('{') {
		return property, nil
	}
	p.skipSpace()
	inner, err := p.parsePath()
	if err != nil {
		return "", err
	}
	p.skipSpace()
	if !p.consume('}') {
		return "", errorf(ErrMalformed, "oslc.orderBy nested term for %q is not closed by '}'", identifier)
	}
	return property + "." + inner, nil
}

func (p *oslcParser) identifier() (string, error) {
	if p.peek('*') {
		p.pos++
		return SelectAll, nil
	}
	start := p.pos
	// A leading "@" scans as a name so resolveProperty can name its OSLC spelling.
	if p.peek('@') {
		p.pos++
	}
	for p.pos < len(p.s) {
		// Decoded, not byte-wise: classifying a byte of a multi-byte letter would
		// end a name the prefix bindings accept in the middle of a rune.
		r, size := utf8.DecodeRuneInString(p.s[p.pos:])
		if r == utf8.RuneError && size <= 1 {
			break
		}
		if unicode.IsLetter(r) || unicode.IsDigit(r) || strings.ContainsRune("_-.:", r) {
			p.pos += size
			continue
		}
		break
	}
	word := p.s[start:p.pos]
	if start == p.pos || !(strings.Contains(word, ":") || strings.HasPrefix(word, "@")) {
		return "", errorf(ErrMalformed, "OSLC term requires a prefixed-name identifier")
	}
	return word, nil
}

func (p *oslcParser) operator() Operator {
	for _, candidate := range []struct {
		text string
		op   Operator
	}{{">=", GreaterEqual}, {"<=", LessEqual}, {"!=", NotEqual}, {"=", Equal}, {">", Greater}, {"<", Less}} {
		if strings.HasPrefix(p.s[p.pos:], candidate.text) {
			p.pos += len(candidate.text)
			return candidate.op
		}
	}
	return ""
}

func (p *oslcParser) value() (string, error) {
	p.skipSpace()
	if p.pos >= len(p.s) {
		return "", errorf(ErrMalformed, "OSLC term has no value")
	}
	if p.s[p.pos] == '<' {
		start := p.pos + 1
		end := strings.IndexByte(p.s[start:], '>')
		if end < 0 {
			return "", errorf(ErrMalformed, "unterminated URI value")
		}
		p.pos = start + end + 1
		return localValue(p.s[start : start+end]), nil
	}
	if p.s[p.pos] == '"' {
		p.pos++
		var b strings.Builder
		for p.pos < len(p.s) {
			c := p.s[p.pos]
			p.pos++
			if c == '"' {
				if p.pos < len(p.s) && p.s[p.pos] == '@' {
					return "", errorf(ErrUnsupportedLiteral, "language-tagged literals are not supported")
				}
				if p.pos+1 < len(p.s) && strings.HasPrefix(p.s[p.pos:], "^^") {
					p.pos += 2
					dt, err := p.identifier()
					if err != nil {
						return "", err
					}
					if !strings.HasPrefix(dt, "xsd:") {
						return "", errorf(ErrUnsupportedLiteral, "non-xsd datatype %q is not supported", dt)
					}
					switch strings.TrimPrefix(dt, "xsd:") {
					case "string", "boolean", "integer", "decimal", "double":
					default:
						return "", errorf(ErrUnsupportedLiteral, "xsd datatype %q is not supported", dt)
					}
				}
				return b.String(), nil
			}
			if c == '\\' {
				if p.pos >= len(p.s) || (p.s[p.pos] != '\\' && p.s[p.pos] != '"') {
					return "", errorf(ErrMalformed, "literal contains unsupported escape")
				}
				b.WriteByte(p.s[p.pos])
				p.pos++
				continue
			}
			b.WriteByte(c)
		}
		return "", errorf(ErrMalformed, "unterminated string literal")
	}
	if p.s[p.pos] == '*' {
		p.pos++
		return "*", nil
	}
	start := p.pos
	for p.pos < len(p.s) {
		r, size := utf8.DecodeRuneInString(p.s[p.pos:])
		if unicode.IsSpace(r) || strings.ContainsRune(",]}", r) {
			break
		}
		p.pos += size
	}
	raw := p.s[start:p.pos]
	if raw == "true" || raw == "false" {
		return raw, nil
	}
	if _, err := strconv.ParseFloat(raw, 64); err == nil {
		return raw, nil
	}
	if strings.Contains(raw, ":") {
		return resolveValue(raw, p.prefixes)
	}
	if raw == "" {
		return "", errorf(ErrMalformed, "OSLC comparison has no value")
	}
	// A bare name is the answer's own spelling of a value, so say what quoting it takes.
	return "", errorf(ErrMalformed,
		"invalid OSLC value %q: write a name as a quoted literal %s", raw, strconv.Quote(raw))
}

func (p *oslcParser) skipSpace() {
	for p.pos < len(p.s) {
		r, size := utf8.DecodeRuneInString(p.s[p.pos:])
		if !unicode.IsSpace(r) {
			return
		}
		p.pos += size
	}
}

func (p *oslcParser) consume(c byte) bool {
	if p.pos < len(p.s) && p.s[p.pos] == c {
		p.pos++
		return true
	}
	return false
}

func (p *oslcParser) peek(c byte) bool { return p.pos < len(p.s) && p.s[p.pos] == c }

func (p *oslcParser) consumeWord(word string) bool {
	if !strings.HasPrefix(p.s[p.pos:], word) {
		return false
	}
	end := p.pos + len(word)
	if end < len(p.s) {
		if r, _ := utf8.DecodeRuneInString(p.s[end:]); !unicode.IsSpace(r) && r != '{' {
			return false
		}
	}
	p.pos = end
	return true
}

// resolveProperty names the evaluator's property an OSLC prefixed name denotes:
// rdf:type, a metamodel property under the SysML namespace, or one of the
// evaluator's own under the OpenSysML namespace.
func (p *oslcParser) resolveProperty(name string) (string, error) {
	if name == SelectAll {
		return "", errorf(ErrMalformed, "the '*' wildcard names every property in %s; a term compares one", paramSelect)
	}
	// The Go API names these two "@type" and "@id"; OSLC query text cannot.
	switch name {
	case query.PropertyType:
		return "", errorf(ErrMalformed, "OSLC query text spells the metamodel type \"rdf:type\", not %q", name)
	case query.PropertyID:
		return "", errorf(ErrMalformed,
			"%q is not an OSLC query property: element identity is reported for every result", name)
	}
	iri, err := expandName(name, p.prefixes)
	if err != nil {
		return "", err
	}
	if iri == rdf.RDFType {
		return query.PropertyType, nil
	}
	table := p.table
	if table == nil {
		table = NewTable(nil, nil)
	}
	if local, ok := strings.CutPrefix(iri, rdf.SysML); ok && table.Metamodel(local) {
		return local, nil
	}
	if local, ok := strings.CutPrefix(iri, rdf.OpenSysML); ok && table.Derived(local) &&
		local != query.PropertyID && local != query.PropertyType {
		return local, nil
	}
	return "", unknownOSLCProperty(name, iri, p.prefixes, table)
}

// PrefixedPropertyNames returns the property names OSLC query text spells
// under prefixes, in stable order, along with the local names of every
// namespace those bindings leave unnamed, keyed by namespace. An oslc.prefix
// binding decides what the parser accepts, so a diagnostic reads the same map.
func PrefixedPropertyNames(prefixes map[string]string, table *Table) (names []string, unbound map[string][]string) {
	unbound = make(map[string][]string)
	add := func(namespace string, locals []string) {
		prefix, bound := namingPrefix(prefixes, namespace)
		for _, local := range locals {
			if bound {
				names = append(names, prefix+":"+local)
				continue
			}
			unbound[namespace] = append(unbound[namespace], local)
		}
	}
	add(rdf.RDFNS, []string{"type"})
	add(rdf.SysML, table.MetamodelNames())
	var derived []string
	for _, name := range table.DerivedNames() {
		if name != query.PropertyID && name != query.PropertyType {
			derived = append(derived, name)
		}
	}
	add(rdf.OpenSysML, derived)
	sort.Strings(names)
	for _, locals := range unbound {
		sort.Strings(locals)
	}
	return names, unbound
}

// namingPrefix returns the alphabetically first prefix bound to namespace, so
// one namespace is always offered under one spelling.
func namingPrefix(prefixes map[string]string, namespace string) (string, bool) {
	bound := make([]string, 0, len(prefixes))
	for prefix, iri := range prefixes {
		if iri == namespace {
			bound = append(bound, prefix)
		}
	}
	if len(bound) == 0 {
		return "", false
	}
	sort.Strings(bound)
	return bound[0], true
}

// unknownOSLCProperty is the refusal of a property the model does not define,
// with the nearest spelling the prefixes allow and the namespaces left unnamed.
func unknownOSLCProperty(name, iri string, prefixes map[string]string, table *Table) *Error {
	names, unbound := PrefixedPropertyNames(prefixes, table)
	var b strings.Builder
	fmt.Fprintf(&b, "unknown OSLC query property %q", name)
	if local, ok := strings.CutPrefix(iri, rdf.SysML); ok && table.Derived(local) {
		if prefix, bound := namingPrefix(prefixes, rdf.OpenSysML); bound {
			fmt.Fprintf(&b, ": the metamodel does not define %s; OpenSysML derives it as %s:%s", local, prefix, local)
			return &Error{Kind: ErrUnknownProperty, Message: b.String(), Candidates: names}
		}
	}
	if local, ok := strings.CutPrefix(iri, rdf.OpenSysML); ok && table.Metamodel(local) {
		if prefix, bound := namingPrefix(prefixes, rdf.SysML); bound {
			fmt.Fprintf(&b, ": the metamodel defines %s, spelled %s:%s", local, prefix, local)
			return &Error{Kind: ErrUnknownProperty, Message: b.String(), Candidates: names}
		}
	}
	message := suggest.With(b.String(), name, suggest.Nearest(name, names))
	b.Reset()
	b.WriteString(message)
	namespaces := make([]string, 0, len(unbound))
	for namespace := range unbound {
		namespaces = append(namespaces, namespace)
	}
	sort.Strings(namespaces)
	for _, namespace := range namespaces {
		fmt.Fprintf(&b, "; %d properties of <%s> have no %s binding", len(unbound[namespace]), namespace, paramPrefix)
	}
	return &Error{Kind: ErrUnknownProperty, Message: b.String(), Candidates: names}
}

func resolveValue(name string, prefixes map[string]string) (string, error) {
	// A model qualified name separates its first two segments with "::", where a
	// prefixed name separates prefix from local part with one ":".
	if _, local, _ := strings.Cut(name, ":"); strings.HasPrefix(local, ":") {
		return "", errorf(ErrMalformed,
			"OSLC value %q is a model qualified name, not a prefixed name; write it as a quoted literal %q", name, name)
	}
	iri, err := expandName(name, prefixes)
	if err != nil {
		return "", err
	}
	return localValue(iri), nil
}

// localValue reads the value an IRI denotes: a term of the SysML namespace
// denotes the model value its local name spells. A `<uri>` and the prefixed
// name expanding to it must reduce alike.
func localValue(iri string) string {
	if strings.HasPrefix(iri, rdf.SysML) {
		return strings.TrimPrefix(iri, rdf.SysML)
	}
	return iri
}

func expandName(name string, prefixes map[string]string) (string, error) {
	prefix, local, ok := strings.Cut(name, ":")
	if !ok {
		return "", errorf(ErrMalformed, "OSLC name %q is not a prefixed name", name)
	}
	namespace, ok := prefixes[prefix]
	if !ok {
		return "", errorf(ErrMalformed, "OSLC prefix %q is unbound", prefix)
	}
	return namespace + local, nil
}

func clonePrefixes(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func splitCommaStrict(text, parameter string) ([]string, error) {
	if strings.TrimSpace(text) == "" {
		return nil, nil
	}
	parts := strings.Split(text, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			return nil, errorf(ErrMalformed, "%s contains an empty term", parameter)
		}
		out = append(out, part)
	}
	return out, nil
}
