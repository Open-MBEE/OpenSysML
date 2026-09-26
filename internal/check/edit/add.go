package edit

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/lexer"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

type memberKind struct {
	languages  map[source.Kind]bool
	definition bool
	typed      bool
}

// memberKinds are the kinds an OpAddMember writes by name alone: not connectors
// (ConnectionKinds) nor connector definitions, which need ends; `fork f;` is neither typed nor a definition.
// A kind only some bodies offer (`subject`, `actor`) is refused for any other owner.
var memberKinds = map[string]memberKind{
	"package":          {languages: bothLangs},
	"ref":              {languages: sysmlOnly, typed: true},
	"return":           {languages: sysmlOnly, typed: true},
	"part def":         {languages: sysmlOnly, definition: true},
	"part":             {languages: sysmlOnly, typed: true},
	"attribute def":    {languages: sysmlOnly, definition: true},
	"attribute":        {languages: sysmlOnly, typed: true},
	"item def":         {languages: sysmlOnly, definition: true},
	"item":             {languages: sysmlOnly, typed: true},
	"port def":         {languages: sysmlOnly, definition: true},
	"port":             {languages: sysmlOnly, typed: true},
	"enum def":         {languages: sysmlOnly, definition: true},
	"enum":             {languages: sysmlOnly, typed: true},
	"individual def":   {languages: sysmlOnly, definition: true},
	"individual":       {languages: sysmlOnly, typed: true},
	"metadata def":     {languages: sysmlOnly, definition: true},
	"metadata":         {languages: sysmlOnly, typed: true},
	"view def":         {languages: sysmlOnly, definition: true},
	"view":             {languages: sysmlOnly, typed: true},
	"viewpoint def":    {languages: sysmlOnly, definition: true},
	"viewpoint":        {languages: sysmlOnly, typed: true},
	"rendering def":    {languages: sysmlOnly, definition: true},
	"rendering":        {languages: sysmlOnly, typed: true},
	"concern def":      {languages: sysmlOnly, definition: true},
	"concern":          {languages: sysmlOnly, typed: true},
	"calc def":         {languages: sysmlOnly, definition: true},
	"calc":             {languages: sysmlOnly, typed: true},
	"action def":       {languages: sysmlOnly, definition: true},
	"action":           {languages: sysmlOnly, typed: true},
	"state def":        {languages: sysmlOnly, definition: true},
	"state":            {languages: sysmlOnly, typed: true},
	"occurrence def":   {languages: sysmlOnly, definition: true},
	"occurrence":       {languages: sysmlOnly, typed: true},
	"allocation def":   {languages: sysmlOnly, definition: true},
	"binding def":      {languages: sysmlOnly, definition: true},
	"constraint def":   {languages: sysmlOnly, definition: true},
	"constraint":       {languages: sysmlOnly, typed: true},
	"requirement def":  {languages: sysmlOnly, definition: true},
	"requirement":      {languages: sysmlOnly, typed: true},
	"case def":         {languages: sysmlOnly, definition: true},
	"case":             {languages: sysmlOnly, typed: true},
	"analysis def":     {languages: sysmlOnly, definition: true},
	"analysis":         {languages: sysmlOnly, typed: true},
	"verification def": {languages: sysmlOnly, definition: true},
	"verification":     {languages: sysmlOnly, typed: true},
	"use case def":     {languages: sysmlOnly, definition: true},
	"use case":         {languages: sysmlOnly, typed: true},
	"subject":          {languages: sysmlOnly, typed: true},
	"actor":            {languages: sysmlOnly, typed: true},
	"stakeholder":      {languages: sysmlOnly, typed: true},
	"objective":        {languages: sysmlOnly, typed: true},
	"fork":             {languages: sysmlOnly},
	"join":             {languages: sysmlOnly},
	"merge":            {languages: sysmlOnly},
	"decide":           {languages: sysmlOnly},
	"class":            {languages: kermlOnly, definition: true},
	"struct":           {languages: kermlOnly, definition: true},
	"datatype":         {languages: kermlOnly, definition: true},
	"classifier":       {languages: kermlOnly, definition: true},
	"feature":          {languages: kermlOnly, typed: true},
	"step":             {languages: kermlOnly, typed: true},
	"expr":             {languages: kermlOnly, typed: true},
	"bool":             {languages: kermlOnly, typed: true},
	"behavior":         {languages: kermlOnly, definition: true},
	"function":         {languages: kermlOnly, definition: true},
	"predicate":        {languages: kermlOnly, definition: true},
	"metaclass":        {languages: kermlOnly, definition: true},
}

// MemberKinds lists the member kinds legal in a language, sorted.
func MemberKinds(lang source.Kind) []string {
	return legalKinds(lang, func(name string) map[source.Kind]bool { return memberKinds[name].languages },
		mapKeys(memberKinds))
}

// MemberKindTyped reports whether an OpAddMember of kind may carry a Type.
func MemberKindTyped(kind string) bool {
	return memberKinds[kind].typed
}

// MemberKindOwnerBound reports whether only some bodies offer a member of kind:
// `subject` belongs in a requirement or case, `part` anywhere.
func MemberKindOwnerBound(kind string) bool {
	return parser.MemberOwner(kind) != ""
}

// MemberKindAdmittedBy reports whether the body of owner, a declaration an
// OpAddMember may name, offers a member of kind.
func MemberKindAdmittedBy(owner ast.Node, kind string) bool {
	return parser.BodyAdmitsMember(owner, kind)
}

func legalKinds(lang source.Kind, languages func(string) map[source.Kind]bool, names []string) []string {
	out := []string{}
	for _, name := range names {
		if languages(name)[lang] {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

func mapKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func (m Model) addMemberSplice(i int, op Operation) (splice, error) {
	kind, ok := memberKinds[op.MemberKind]
	if !ok || !kind.languages[m.Source.Kind()] {
		return splice{}, &Error{
			Failure:        FailureIllegalKind,
			OperationIndex: i,
			Message: fmt.Sprintf("kind %q is not legal in %s source %q",
				op.MemberKind, m.Source.Kind(), m.Source.Name()),
		}
	}
	if op.MemberName == "" {
		switch {
		case op.MemberKind == "return" && op.Type == "" && op.Multiplicity == "":
			return splice{}, &Error{
				Failure: FailureIllegalKind, OperationIndex: i,
				Message: "an unnamed return parameter needs a type or multiplicity",
			}
		case op.MemberKind != "return" && len(op.Redefines) == 0:
			return splice{}, &Error{
				Failure: FailureInvalidName, OperationIndex: i,
				Message: "an empty member name requires redefines targets or kind return",
			}
		}
	} else if err := checkName(i, op.MemberName); err != nil {
		e := err.(*Error)
		e.Failure = FailureInvalidName
		e.Message = fmt.Sprintf("member name %q is not an identifier", op.MemberName)
		return splice{}, e
	}
	if op.Value != "" {
		valueOp := op
		valueOp.Target = op.MemberName
		if err := m.checkValue(i, valueOp); err != nil {
			return splice{}, err
		}
	}
	if op.IsDefault && op.Value == "" {
		return splice{}, &Error{
			Failure: FailureInvalidValue, OperationIndex: i,
			Message: "default requires a nonempty value expression",
		}
	}
	if op.Direction != "" && op.Direction != "in" && op.Direction != "out" && op.Direction != "inout" {
		return splice{}, &Error{
			Failure: FailureInvalidValue, OperationIndex: i,
			Message: fmt.Sprintf("direction %q is not in, out or inout", op.Direction),
		}
	}
	if !kind.typed && op.Type != "" {
		return splice{}, &Error{
			Failure:        FailureIllegalKind,
			OperationIndex: i,
			Message:        fmt.Sprintf("kind %q cannot carry a typing target", op.MemberKind),
		}
	}
	if !kind.typed && !kind.definition && (op.Multiplicity != "" || op.Value != "") {
		return splice{}, &Error{
			Failure:        FailureIllegalKind,
			OperationIndex: i,
			Message:        fmt.Sprintf("kind %q takes a name alone", op.MemberKind),
		}
	}
	if kind.definition && op.Value != "" {
		return splice{}, &Error{
			Failure: FailureIllegalKind, OperationIndex: i,
			Message: fmt.Sprintf("definition kind %q cannot carry a value", op.MemberKind),
		}
	}
	if !kind.definition && len(op.Specializes) > 0 {
		return splice{}, &Error{
			Failure:        FailureIllegalKind,
			OperationIndex: i,
			Message:        fmt.Sprintf("kind %q is a usage and cannot carry specializes targets", op.MemberKind),
		}
	}
	if op.IsAbstract && !kind.definition && (!kind.typed || memberPrefixExcluded(op.MemberKind)) {
		return splice{}, &Error{
			Failure: FailureIllegalKind, OperationIndex: i,
			Message: fmt.Sprintf("kind %q cannot be abstract", op.MemberKind),
		}
	}
	if op.Direction != "" &&
		(kind.definition || !kind.typed || memberPrefixExcluded(op.MemberKind)) {
		return splice{}, &Error{
			Failure: FailureIllegalKind, OperationIndex: i,
			Message: fmt.Sprintf("kind %q cannot carry a direction", op.MemberKind),
		}
	}
	if op.IsDefault && (kind.definition || !kind.typed) {
		return splice{}, &Error{
			Failure: FailureIllegalKind, OperationIndex: i,
			Message: fmt.Sprintf("kind %q cannot carry a default value", op.MemberKind),
		}
	}
	if len(op.Redefines) > 0 && kind.definition {
		return splice{}, &Error{
			Failure: FailureIllegalKind, OperationIndex: i,
			Message: fmt.Sprintf("definition kind %q cannot carry redefines targets; use specializes", op.MemberKind),
		}
	}
	if op.MemberKind == "return" && (op.IsAbstract || op.Direction != "" || len(op.Redefines) > 0) {
		return splice{}, &Error{
			Failure: FailureIllegalKind, OperationIndex: i,
			Message: "return parameters cannot be abstract, directional or redefining",
		}
	}
	for _, target := range op.Redefines {
		if err := checkEnd(i, "redefines", target); err != nil {
			return splice{}, err
		}
	}
	owner, ownerScope, err := m.addOwner(op.Owner)
	if err != nil {
		e := err.(*Error)
		e.OperationIndex = i
		return splice{}, e
	}
	if !parser.BodyAdmitsMember(owner, op.MemberKind) {
		return splice{}, &Error{
			Failure:        FailureIllegalKind,
			OperationIndex: i,
			Message: fmt.Sprintf("kind %q is only declared in a %s body, which %s does not open",
				op.MemberKind, parser.MemberOwner(op.MemberKind), ownerName(op.Owner)),
		}
	}
	if op.MemberKind == "return" && !parser.BodyIsCalculation(owner) {
		return splice{}, &Error{
			Failure: FailureIllegalKind, OperationIndex: i,
			Message: "return parameters are only admitted in calculation and case bodies",
		}
	}
	if op.MemberKind == "return" {
		for _, member := range ast.DeclMembers(owner) {
			if usage, ok := member.(*ast.Usage); ok && usage.IsResult {
				return splice{}, &Error{
					Failure: FailureIllegalKind, OperationIndex: i,
					Message: "a calculation or case body already has a return parameter",
				}
			}
		}
	}
	if op.MemberName != "" && ownerScope != nil && len(ownerScope.LookupLocalAll(op.MemberName)) > 0 {
		return splice{}, &Error{
			Failure:        FailureMemberNameTaken,
			OperationIndex: i,
			Message:        fmt.Sprintf("%s already declares %q", op.Owner, op.MemberName),
		}
	}
	ins := m.memberInsertion(owner, writeMember(op, kind))
	return splice{span: ins.span, text: ins.text, opIndex: i, target: op.Owner}, nil
}

func memberPrefixExcluded(kind string) bool {
	switch kind {
	case "package", "subject", "actor", "stakeholder", "objective",
		"fork", "join", "merge", "decide", "return":
		return true
	default:
		return false
	}
}

func (m Model) addOwner(fqn string) (ast.Node, *symbols.Scope, error) {
	rootScope := m.Index.DocumentRoot(m.Source.Name())
	if fqn == "" {
		return m.Root, rootScope, nil
	}
	var local *symbols.Symbol
	for _, sym := range m.declared(fqn) {
		if sym.DocName == m.Source.Name() {
			if local != nil {
				return nil, nil, &Error{Failure: FailureAmbiguousTarget,
					Message: fmt.Sprintf("%q names several declarations", fqn)}
			}
			local = sym
		}
	}
	if local == nil {
		return nil, nil, &Error{Failure: FailureOwnerUnknown,
			Message: fmt.Sprintf("no namespace named %q in this model", fqn)}
	}
	if local.Scope == nil {
		return nil, nil, &Error{Failure: FailureOwnerNotNamespace,
			Message: fmt.Sprintf("%q cannot contain members", fqn)}
	}
	switch local.Decl.(type) {
	case *ast.Package, *ast.Namespace, *ast.Definition, *ast.Usage:
		return local.Decl, local.Scope, nil
	default:
		return nil, nil, &Error{Failure: FailureOwnerNotNamespace,
			Message: fmt.Sprintf("%q cannot contain members", fqn)}
	}
}

// ownerName names an add-member owner for a message: the document for "".
func ownerName(fqn string) string {
	if fqn == "" {
		return "the document"
	}
	return fmt.Sprintf("%q", fqn)
}

func writeMember(op Operation, kind memberKind) string {
	prefix := make([]string, 0, 3)
	if op.Direction != "" {
		prefix = append(prefix, op.Direction)
	}
	if op.IsAbstract {
		prefix = append(prefix, "abstract")
	}
	switch op.MemberKind {
	case "return":
		prefix = append(prefix, "return")
	case "ref":
		if op.Direction == "" {
			prefix = append(prefix, "ref")
		}
	default:
		prefix = append(prefix, op.MemberKind)
	}
	header := strings.Join(prefix, " ")
	if op.MemberName != "" {
		header += " " + op.MemberName
	}
	if kind.definition && len(op.Specializes) > 0 {
		header += " specializes " + strings.Join(op.Specializes, ", ")
	} else if !kind.definition && op.Type != "" {
		header += " : " + op.Type
	}
	if len(op.Redefines) > 0 {
		header += " :>> " + strings.Join(op.Redefines, ", ")
	}
	if op.Multiplicity != "" {
		header += " " + op.Multiplicity
	}
	if op.Value != "" {
		if op.IsDefault {
			header += " default = " + op.Value
		} else {
			header += " = " + op.Value
		}
	}
	return header + ";"
}

// insertion is the splice adding a member to an owner: text replaces span, and
// the member's own notation starts at offset at within text.
type insertion struct {
	span source.Span
	text string
	at   int
}

// ownerMemberIndent is the indentation a member of owner is written at.
func (m Model) ownerMemberIndent(owner ast.Node) string {
	if owner == m.Root {
		return ""
	}
	return m.memberIndent(owner.Span())
}

// memberInsertion places text, one member's notation with its later lines
// already indented for owner, where a new member of owner goes: before the
// closing brace of its body, or in the body opened for a bodyless owner.
func (m Model) memberInsertion(owner ast.Node, text string) insertion {
	if owner == m.Root {
		prefix := ""
		if len(m.Source.Bytes()) > 0 && m.Source.Bytes()[len(m.Source.Bytes())-1] != '\n' {
			prefix = "\n"
		}
		return insertion{span: source.Span{Offset: m.Source.Len()}, text: prefix + text + "\n", at: len(prefix)}
	}
	body, hasBody := bodyInfo(owner)
	ownerIndent := lineIndent(m.Source.Bytes(), owner.Span().Offset)
	indent := m.ownerMemberIndent(owner)
	if hasBody {
		if parser.BodyIsCalculation(owner) {
			members := ast.DeclMembers(owner)
			if len(members) > 0 && ast.IsExpression(members[len(members)-1]) {
				return m.memberInsertionBeforeResult(members[len(members)-1], text, indent)
			}
		}
		rbrace := lastToken(m.Source, body, lexer.RBrace)
		closeOffset := rbrace.Span.Offset
		lineStart := closeOffset
		for lineStart > 0 && m.Source.Bytes()[lineStart-1] != '\n' {
			lineStart--
		}
		closeIndent := string(m.Source.Bytes()[lineStart:closeOffset])
		if closeIndent != "" && !onlyWhitespace([]byte(closeIndent)) {
			lineStart = closeOffset
			closeIndent = ownerIndent
		}
		prefix := "\n"
		if lineStart > 0 && m.Source.Bytes()[lineStart-1] == '\n' {
			prefix = ""
		}
		return insertion{
			span: source.Span{Offset: lineStart, Len: closeOffset - lineStart},
			text: prefix + indent + text + "\n" + closeIndent,
			at:   len(prefix) + len(indent),
		}
	}
	semi := lastToken(m.Source, owner.Span(), lexer.Semicolon)
	open := " {\n" + indent
	return insertion{
		span: source.Span{Offset: semi.Span.Offset, Len: semi.Span.Len},
		text: open + text + "\n" + ownerIndent + "}",
		at:   len(open),
	}
}

func (m Model) memberInsertionBeforeResult(result ast.Node, text, indent string) insertion {
	content := m.Source.Bytes()
	offset := result.Span().Offset
	lineStart := offset
	for lineStart > 0 && content[lineStart-1] != '\n' {
		lineStart--
	}
	leading := content[lineStart:offset]
	if onlyWhitespace(leading) {
		return insertion{
			span: source.Span{Offset: lineStart, Len: len(leading)},
			text: indent + text + "\n" + string(leading),
			at:   len(indent),
		}
	}
	prefix := ""
	if offset == 0 || (content[offset-1] != ' ' && content[offset-1] != '\t' && content[offset-1] != '\n') {
		prefix = " "
	}
	return insertion{
		span: source.Span{Offset: offset},
		text: prefix + text + " ",
		at:   len(prefix),
	}
}

func bodyInfo(node ast.Node) (source.Span, bool) {
	switch d := node.(type) {
	case *ast.Package:
		return d.Span(), d.HasBody
	case *ast.Namespace:
		return d.Span(), d.HasBody
	case *ast.Definition:
		return d.Span(), d.HasBody
	case *ast.Usage:
		return d.Span(), d.HasBody
	case *ast.TransitionMember:
		return d.Span(), d.HasBody
	case *ast.SuccessionEdge:
		return d.Span(), d.HasBody
	default:
		return source.Span{}, false
	}
}

func lastToken(sf *source.SourceFile, span source.Span, kind lexer.Kind) lexer.Token {
	var found lexer.Token
	lx := lexer.New(sf)
	for tok := lx.Next(); tok.Kind != lexer.EOF; tok = lx.Next() {
		if tok.Span.Offset >= span.End() {
			break
		}
		if tok.Kind == kind {
			found = tok
		}
	}
	return found
}

func lineIndent(content []byte, offset int) string {
	start := offset
	for start > 0 && content[start-1] != '\n' {
		start--
	}
	i := start
	for i < offset && (content[i] == ' ' || content[i] == '\t') {
		i++
	}
	return string(content[start:i])
}

func (m Model) memberIndent(owner source.Span) string {
	content := m.Source.Bytes()
	base := lineIndent(content, owner.Offset)
	start := owner.Offset
	for start < owner.End() {
		end := start
		for end < owner.End() && content[end] != '\n' {
			end++
		}
		i := start
		for i < end && (content[i] == ' ' || content[i] == '\t') {
			i++
		}
		if i < end && i > start {
			prefix := string(content[start:i])
			if len(prefix) > len(base) {
				return prefix
			}
		}
		if end == owner.End() {
			break
		}
		start = end + 1
	}
	style := "\t"
	if !strings.Contains(string(content), "\t") {
		style = "    "
	}
	return base + style
}
