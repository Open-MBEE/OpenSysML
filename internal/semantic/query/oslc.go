package query

import (
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Namespace IRIs, mirroring the RDF vocabulary in internal/translate/rdf.
const (
	rdfNS   = "http://www.w3.org/1999/02/22-rdf-syntax-ns#"
	xsdNS   = "http://www.w3.org/2001/XMLSchema#"
	sysmlNS = "https://www.omg.org/spec/SysML#"
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

// DefaultPrefixes are the prefixes available without an oslc.prefix binding.
var DefaultPrefixes = map[string]string{
	"sysml": sysmlNS,
	"rdf":   rdfNS,
	"xsd":   xsdNS,
}

var oslcPropertyMappings = map[string]string{
	rdfNS + "type":                PropertyType,
	sysmlNS + "qualifiedName":     PropertyQualifiedName,
	sysmlNS + "name":              PropertyName,
	sysmlNS + "declaredName":      PropertyDeclaredName,
	sysmlNS + "shortName":         PropertyShortName,
	sysmlNS + "declaredShortName": PropertyDeclaredShortName,
	sysmlNS + "documentation":     PropertyDocumentation,
	sysmlNS + "owner":             PropertyOwner,
	sysmlNS + "isAbstract":        PropertyIsAbstract,
	sysmlNS + "type":              PropertyElementType,
	sysmlNS + "multiplicityLower": PropertyMultiplicityLower,
	sysmlNS + "multiplicityUpper": PropertyMultiplicityUpper,
}

// ParseOSLC parses a where expression. It also accepts a query-parameter
// string, allowing callers to pass an HTTP-style OSLC query directly.
func ParseOSLC(text string) (Query, error) {
	if isParameterText(text) {
		return ParseParameters(text)
	}
	p := oslcParser{s: text, prefixes: clonePrefixes(DefaultPrefixes)}
	q, err := p.parseCompound()
	if err != nil {
		return Query{}, err
	}
	return q, nil
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

// ParseParameters parses oslc.where, oslc.select, oslc.orderBy, and
// oslc.prefix parameters. Parameter text follows URL query decoding rules.
func ParseParameters(text string) (Query, error) {
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
	if _, present := values[paramSearch]; present {
		return Query{}, errorf(ErrUnsupportedSearchTerms, "oslc.searchTerms is not implemented: the implementation performs element identification, not free-text search")
	}
	if properties, present := values[paramProperties]; present && len(properties) > 0 {
		if strings.TrimSpace(properties[0]) == "*" {
			return Query{}, wildcardError(paramProperties)
		}
		return Query{}, errorf(ErrMalformed, "oslc.properties is not implemented: name the properties to report with %s", paramSelect)
	}
	selectTerms, err := splitCommaStrict(values.Get(paramSelect), paramSelect)
	if err != nil {
		return Query{}, err
	}
	q := Query{}
	if order := values.Get(paramOrderBy); order != "" {
		terms, err := parseOrder(order, prefixes)
		if err != nil {
			return Query{}, err
		}
		q.OrderBy = terms
	}
	if where, present := values[paramWhere]; present {
		p := oslcParser{s: where[0], prefixes: prefixes}
		parsed, err := p.parseCompound()
		if err != nil {
			return Query{}, err
		}
		q.Where = parsed.Where
	}
	var selected []string
	for _, name := range selectTerms {
		if name == "*" {
			return Query{}, wildcardError(paramSelect)
		}
		if strings.ContainsAny(name, "{}") {
			return Query{}, scopedError()
		}
		resolved, err := resolveProperty(name, prefixes)
		if err != nil {
			return Query{}, err
		}
		// A property selected twice can only be reported once, so naming it
		// under two spellings is a misuse rather than two columns.
		if first, repeated := q.Spelling[resolved]; repeated {
			if first == name {
				return Query{}, errorf(ErrMalformed, "%s names %q twice", paramSelect, name)
			}
			return Query{}, errorf(ErrMalformed, "%s names one property twice: %q and %q are the same property",
				paramSelect, first, name)
		}
		if q.Spelling == nil {
			q.Spelling = make(map[string]string)
		}
		q.Spelling[resolved] = name
		selected = append(selected, resolved)
	}
	q.Select = selected
	return q, nil
}

// omitting names, per parameter, what leaving it out asks for: an empty value
// is a misuse, since the parameter as written narrows nothing.
var omitting = map[string]string{
	paramWhere:   "select every element",
	paramSelect:  "report every property",
	paramOrderBy: "keep declaration order",
	paramPrefix:  "use the default prefixes",
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

func parseOrder(text string, prefixes map[string]string) ([]OrderTerm, error) {
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
		if strings.ContainsAny(raw, "{}") {
			return nil, scopedError()
		}
		if raw == "*" {
			return nil, wildcardError(paramOrderBy)
		}
		name, err := resolveProperty(raw, prefixes)
		if err != nil {
			return nil, err
		}
		out = append(out, OrderTerm{Property: name, Desc: desc})
	}
	return out, nil
}

type oslcParser struct {
	s        string
	pos      int
	prefixes map[string]string
}

func (p *oslcParser) parseCompound() (Query, error) {
	var q Query
	for {
		p.skipSpace()
		if p.pos >= len(p.s) {
			if len(q.Where) == 0 {
				return Query{}, errorf(ErrMalformed, "OSLC compound term is empty")
			}
			return q, nil
		}
		identifier, err := p.identifier()
		if err != nil {
			return Query{}, err
		}
		p.skipSpace()
		if p.peek('{') {
			return Query{}, scopedError()
		}
		prop, err := resolveProperty(identifier, p.prefixes)
		if err != nil {
			return Query{}, err
		}
		if p.consumeWord("in") {
			p.skipSpace()
			if !p.consume('[') {
				return Query{}, errorf(ErrMalformed, "OSLC in term for %q must contain a value list", identifier)
			}
			var vals []string
			for {
				p.skipSpace()
				value, err := p.value()
				if err != nil {
					return Query{}, err
				}
				vals = append(vals, value)
				p.skipSpace()
				if p.consume(']') {
					break
				}
				if !p.consume(',') {
					return Query{}, errorf(ErrMalformed, "OSLC in term for %q needs ',' or ']'", identifier)
				}
			}
			q.Where = append(q.Where, Predicate{Property: prop, Operator: In, Values: vals})
		} else {
			op := p.operator()
			if op == "" {
				return Query{}, errorf(ErrMalformed, "OSLC term for %q has no comparison operator", identifier)
			}
			p.skipSpace()
			value, err := p.value()
			if err != nil {
				return Query{}, err
			}
			q.Where = append(q.Where, Predicate{Property: prop, Operator: op, Values: []string{value}})
		}
		p.skipSpace()
		if p.pos >= len(p.s) {
			return q, nil
		}
		if !p.consumeWord("and") {
			return Query{}, errorf(ErrMalformed, "OSLC compound terms may be joined only by \" and \"")
		}
		p.skipSpace()
		if p.pos >= len(p.s) {
			return Query{}, errorf(ErrMalformed, "OSLC compound term has a trailing \"and\"")
		}
	}
}

func (p *oslcParser) identifier() (string, error) {
	if p.peek('*') {
		p.pos++
		return "*", nil
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
		if r, _ := utf8.DecodeRuneInString(p.s[end:]); !unicode.IsSpace(r) {
			return false
		}
	}
	p.pos = end
	return true
}

func resolveProperty(name string, prefixes map[string]string) (string, error) {
	if name == "*" {
		return "", wildcardError("oslc.where")
	}
	// The Go API names these two "@type" and "@id"; OSLC query text cannot.
	switch name {
	case PropertyType:
		return "", errorf(ErrMalformed, "OSLC query text spells the metamodel type \"rdf:type\", not %q", name)
	case PropertyID:
		return "", errorf(ErrMalformed,
			"%q is not an OSLC query property: element identity is reported for every result", name)
	}
	iri, err := expandName(name, prefixes)
	if err != nil {
		return "", err
	}
	if property, ok := oslcPropertyMappings[iri]; ok {
		return property, nil
	}
	return "", unknownOSLCProperty(name, prefixes)
}

// propertyNamespaces are the namespaces the OSLC property mapping spells its
// properties in.
var propertyNamespaces = []string{rdfNS, sysmlNS}

// PrefixedPropertyNames returns the property names OSLC query text spells
// under prefixes, in stable order, along with the local names of every
// namespace those bindings leave unnamed, keyed by namespace. An oslc.prefix
// binding decides what the parser accepts, so a diagnostic reads the same map.
func PrefixedPropertyNames(prefixes map[string]string) (names []string, unbound map[string][]string) {
	unbound = make(map[string][]string)
	for iri := range oslcPropertyMappings {
		namespace, local := splitPropertyIRI(iri)
		if prefix, bound := namingPrefix(prefixes, namespace); bound {
			names = append(names, prefix+":"+local)
			continue
		}
		unbound[namespace] = append(unbound[namespace], local)
	}
	sort.Strings(names)
	for _, locals := range unbound {
		sort.Strings(locals)
	}
	return names, unbound
}

func splitPropertyIRI(iri string) (namespace, local string) {
	for _, namespace := range propertyNamespaces {
		if strings.HasPrefix(iri, namespace) {
			return namespace, strings.TrimPrefix(iri, namespace)
		}
	}
	return "", iri
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

func unknownOSLCProperty(name string, prefixes map[string]string) *Error {
	names, unbound := PrefixedPropertyNames(prefixes)
	var b strings.Builder
	fmt.Fprintf(&b, "unknown OSLC query property %q", name)
	if len(names) > 0 {
		fmt.Fprintf(&b, "; this implementation reads %s", strings.Join(names, ", "))
	}
	namespaces := make([]string, 0, len(unbound))
	for namespace := range unbound {
		namespaces = append(namespaces, namespace)
	}
	sort.Strings(namespaces)
	for _, namespace := range namespaces {
		fmt.Fprintf(&b, "; it also reads %s of <%s>, which no %s binding names",
			strings.Join(unbound[namespace], ", "), namespace, paramPrefix)
	}
	return errorf(ErrUnknownProperty, "%s", b.String())
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
	if strings.HasPrefix(iri, sysmlNS) {
		return strings.TrimPrefix(iri, sysmlNS)
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

func scopedError() *Error {
	return errorf(ErrUnsupportedScopedTerm, "scoped_term is not implemented: OSLC scoped terms are intended to match a resource based on a property of a related resource; this evaluator performs element identification over the parsed model")
}

func wildcardError(property string) *Error {
	return errorf(ErrUnsupportedWildcard, "the OSLC '*' wildcard is not implemented for %q: this evaluator identifies concrete model elements", property)
}
