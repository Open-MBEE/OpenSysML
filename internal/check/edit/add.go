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
	body       bool
}

// memberKinds are the kinds an OpAddMember writes by name alone: not connectors
// (ConnectionKinds) nor connector definitions, which need ends; `fork f;` is neither typed nor a definition.
// A kind only some bodies offer (`subject`, `actor`) is refused for any other owner.
var memberKinds = map[string]memberKind{
	"package":               {languages: bothLangs},
	"ref":                   {languages: sysmlOnly, typed: true},
	"return":                {languages: sysmlOnly, typed: true},
	"part def":              {languages: sysmlOnly, definition: true},
	"part":                  {languages: sysmlOnly, typed: true},
	"attribute def":         {languages: sysmlOnly, definition: true},
	"attribute":             {languages: sysmlOnly, typed: true},
	"item def":              {languages: sysmlOnly, definition: true},
	"item":                  {languages: sysmlOnly, typed: true},
	"port def":              {languages: sysmlOnly, definition: true},
	"port":                  {languages: sysmlOnly, typed: true},
	"enum def":              {languages: sysmlOnly, definition: true},
	"enum":                  {languages: sysmlOnly, typed: true},
	"individual def":        {languages: sysmlOnly, definition: true},
	"individual":            {languages: sysmlOnly, typed: true},
	"metadata def":          {languages: sysmlOnly, definition: true},
	"metadata":              {languages: sysmlOnly, typed: true},
	"view def":              {languages: sysmlOnly, definition: true},
	"view":                  {languages: sysmlOnly, typed: true},
	"viewpoint def":         {languages: sysmlOnly, definition: true},
	"viewpoint":             {languages: sysmlOnly, typed: true},
	"rendering def":         {languages: sysmlOnly, definition: true},
	"rendering":             {languages: sysmlOnly, typed: true},
	"concern def":           {languages: sysmlOnly, definition: true},
	"concern":               {languages: sysmlOnly, typed: true},
	"calc def":              {languages: sysmlOnly, definition: true, body: true},
	"calc":                  {languages: sysmlOnly, typed: true, body: true},
	"action def":            {languages: sysmlOnly, definition: true},
	"action":                {languages: sysmlOnly, typed: true},
	"perform action":        {languages: sysmlOnly, typed: true},
	"perform":               {languages: sysmlOnly},
	"assert":                {languages: sysmlOnly},
	"assert not":            {languages: sysmlOnly},
	"assert constraint":     {languages: sysmlOnly, typed: true, body: true},
	"assert not constraint": {languages: sysmlOnly, typed: true, body: true},
	"exhibit state":         {languages: sysmlOnly, typed: true},
	"exhibit":               {languages: sysmlOnly},
	"entry action":          {languages: sysmlOnly, typed: true},
	"do action":             {languages: sysmlOnly, typed: true},
	"exit action":           {languages: sysmlOnly, typed: true},
	"state def":             {languages: sysmlOnly, definition: true},
	"state":                 {languages: sysmlOnly, typed: true},
	"occurrence def":        {languages: sysmlOnly, definition: true},
	"occurrence":            {languages: sysmlOnly, typed: true},
	"allocation def":        {languages: sysmlOnly, definition: true},
	"binding def":           {languages: sysmlOnly, definition: true},
	"constraint def":        {languages: sysmlOnly, definition: true, body: true},
	"constraint":            {languages: sysmlOnly, typed: true, body: true},
	"requirement def":       {languages: sysmlOnly, definition: true},
	"requirement":           {languages: sysmlOnly, typed: true},
	"case def":              {languages: sysmlOnly, definition: true, body: true},
	"case":                  {languages: sysmlOnly, typed: true, body: true},
	"analysis def":          {languages: sysmlOnly, definition: true, body: true},
	"analysis":              {languages: sysmlOnly, typed: true, body: true},
	"verification def":      {languages: sysmlOnly, definition: true, body: true},
	"verification":          {languages: sysmlOnly, typed: true, body: true},
	"use case def":          {languages: sysmlOnly, definition: true, body: true},
	"use case":              {languages: sysmlOnly, typed: true, body: true},
	"subject":               {languages: sysmlOnly, typed: true},
	"actor":                 {languages: sysmlOnly, typed: true},
	"stakeholder":           {languages: sysmlOnly, typed: true},
	"objective":             {languages: sysmlOnly, typed: true},
	"fork":                  {languages: sysmlOnly},
	"join":                  {languages: sysmlOnly},
	"merge":                 {languages: sysmlOnly},
	"decide":                {languages: sysmlOnly},
	"class":                 {languages: kermlOnly, definition: true},
	"struct":                {languages: kermlOnly, definition: true},
	"datatype":              {languages: kermlOnly, definition: true},
	"classifier":            {languages: kermlOnly, definition: true},
	"feature":               {languages: kermlOnly, typed: true},
	"step":                  {languages: kermlOnly, typed: true},
	"expr":                  {languages: kermlOnly, typed: true},
	"bool":                  {languages: kermlOnly, typed: true},
	"behavior":              {languages: kermlOnly, definition: true},
	"function":              {languages: kermlOnly, definition: true},
	"predicate":             {languages: kermlOnly, definition: true},
	"metaclass":             {languages: kermlOnly, definition: true},
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
	referenceName := ""
	assertReference := op.MemberKind == "assert" || op.MemberKind == "assert not"
	if assertReference {
		if _, err := checkFeatureReference(i, "asserted constraint", op.MemberName); err != nil {
			return splice{}, err
		}
		if op.Value != "" {
			return splice{}, &Error{
				Failure: FailureIllegalKind, OperationIndex: i,
				Message: fmt.Sprintf("kind %q cannot carry a value", op.MemberKind),
			}
		}
	} else if op.MemberName == "" {
		switch {
		case op.MemberKind == "return" && op.Type == "" && op.Multiplicity == "":
			return splice{}, &Error{
				Failure: FailureIllegalKind, OperationIndex: i,
				Message: "an unnamed return parameter needs a type or multiplicity",
			}
		case (op.MemberKind == "constraint" || op.MemberKind == "assert constraint" ||
			op.MemberKind == "assert not constraint") && (op.Type != "" || op.BodyExpression != ""):
		case op.MemberKind != "return" && len(op.Redefines) == 0:
			return splice{}, &Error{
				Failure: FailureInvalidName, OperationIndex: i,
				Message: "an empty member name requires redefines targets, kind return, or a constraint kind with a type or body expression",
			}
		}
	} else if op.MemberKind == "perform" || op.MemberKind == "exhibit" {
		role := "performed action"
		if op.MemberKind == "exhibit" {
			role = "exhibited state"
		}
		last, err := checkFeatureReference(i, role, op.MemberName)
		if err != nil {
			return splice{}, err
		}
		referenceName = last
	} else if err := checkName(i, op.MemberName); err != nil {
		e := err.(*Error)
		e.Failure = FailureInvalidName
		e.Message = fmt.Sprintf("member name %q is not an identifier", op.MemberName)
		return splice{}, e
	}
	if op.MemberKind == "metadata" && op.Value != "" {
		return splice{}, &Error{
			Failure: FailureIllegalKind, OperationIndex: i,
			Message: "kind \"metadata\" cannot carry a value",
		}
	}
	if op.Value != "" {
		valueOp := op
		valueOp.Target = op.MemberName
		if err := m.checkValue(i, valueOp); err != nil {
			return splice{}, err
		}
	}
	if op.BodyExpression != "" {
		if !kind.body {
			message := fmt.Sprintf("kind %q cannot state a body expression", op.MemberKind)
			if refusal, ok := noResultBodyKinds[op.MemberKind]; ok {
				message = fmt.Sprintf("kind %q cannot state a body expression: %s", op.MemberKind, refusal)
			}
			return splice{}, &Error{
				Failure: FailureIllegalKind, OperationIndex: i,
				Message: message,
			}
		}
		target := op.MemberName
		if target == "" {
			target = op.Owner
		}
		if err := m.checkExpression(i, "body expression", target, op.BodyExpression); err != nil {
			return splice{}, err
		}
	}
	if op.IsDefault && (kind.definition || !kind.typed || memberPrefixExcluded(op.MemberKind)) {
		return splice{}, &Error{
			Failure: FailureIllegalKind, OperationIndex: i,
			Message: fmt.Sprintf("kind %q cannot carry a default value", op.MemberKind),
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
	if op.IsAbstract && (op.MemberKind == "enum def" ||
		(!kind.definition && (!kind.typed || memberPrefixExcluded(op.MemberKind)))) {
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
	if len(op.Redefines) > 0 && kind.definition {
		return splice{}, &Error{
			Failure: FailureIllegalKind, OperationIndex: i,
			Message: fmt.Sprintf("definition kind %q cannot carry redefines targets; use specializes", op.MemberKind),
		}
	}
	if len(op.Redefines) > 0 &&
		(op.MemberKind == "metadata" || op.MemberKind == "perform" || op.MemberKind == "exhibit" ||
			op.MemberKind == "assert" || op.MemberKind == "assert not") {
		return splice{}, &Error{
			Failure: FailureIllegalKind, OperationIndex: i,
			Message: fmt.Sprintf("kind %q cannot carry redefines targets", op.MemberKind),
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
			Message: "return parameters are only admitted in calculation, constraint and case bodies",
		}
	}
	switch op.MemberKind {
	case "assert", "assert not", "assert constraint", "assert not constraint", "exhibit state", "exhibit":
		if !parser.BodyAdmitsBehaviorUsage(owner) {
			return splice{}, &Error{
				Failure: FailureIllegalKind, OperationIndex: i,
				Message: fmt.Sprintf("%s is not admitted in the body of %s",
					op.MemberKind, ownerName(op.Owner)),
			}
		}
	case "entry action", "do action", "exit action":
		if !stateBodyOwner(owner) {
			return splice{}, &Error{
				Failure: FailureIllegalKind, OperationIndex: i,
				Message: fmt.Sprintf("%s is only admitted in a state body, which %s does not open",
					op.MemberKind, ownerName(op.Owner)),
			}
		}
		if hasStateSubaction(owner, memberSubactionKind(op.MemberKind)) {
			return splice{}, &Error{
				Failure: FailureIllegalKind, OperationIndex: i,
				Message: fmt.Sprintf("%s already has a %s action", ownerName(op.Owner), memberSubactionKind(op.MemberKind)),
			}
		}
	}
	if op.MemberKind == "return" {
		for _, member := range ast.DeclMembers(owner) {
			if membership, ok := member.(*ast.Membership); ok && membership != nil {
				member = membership.Member
			}
			if usage, ok := member.(*ast.Usage); ok && usage.IsResult {
				return splice{}, &Error{
					Failure: FailureIllegalKind, OperationIndex: i,
					Message: "a calculation, constraint or case body already has a return parameter",
				}
			}
		}
	}
	takenName := op.MemberName
	if assertReference {
		takenName = ""
	} else if referenceName != "" {
		takenName = referenceName
	}
	if takenName != "" && ownerScope != nil && len(ownerScope.LookupLocalAll(symbolName(takenName))) > 0 {
		return splice{}, &Error{
			Failure:        FailureMemberNameTaken,
			OperationIndex: i,
			Message:        fmt.Sprintf("%s already declares %q", op.Owner, takenName),
		}
	}
	indent := m.ownerMemberIndent(owner)
	unit := m.memberIndentUnit(owner)
	ins := m.memberInsertion(owner, writeMember(op, kind, indent, unit))
	return splice{span: ins.span, text: ins.text, opIndex: i, target: op.Owner}, nil
}

func memberPrefixExcluded(kind string) bool {
	switch kind {
	case "package", "subject", "actor", "stakeholder", "objective",
		"fork", "join", "merge", "decide", "metadata", "return",
		"perform", "perform action", "assert", "assert not", "assert constraint", "assert not constraint",
		"exhibit state", "exhibit", "entry action", "do action", "exit action":
		return true
	default:
		return false
	}
}

var noResultBodyKinds = map[string]string{
	"requirement def": "a requirement body has no result expression; add a require constraint instead",
	"requirement":     "a requirement body has no result expression; add a require constraint instead",
	"concern def":     "a requirement body has no result expression; add a require constraint instead",
	"concern":         "a requirement body has no result expression; add a require constraint instead",
	"viewpoint def":   "a requirement body has no result expression; add a require constraint instead",
	"viewpoint":       "a requirement body has no result expression; add a require constraint instead",
	"objective":       "a requirement body has no result expression; add a require constraint instead",
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
	case *ast.Package, *ast.Namespace, *ast.Definition, *ast.Usage, *ast.SubstateMember:
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

func writeMember(op Operation, kind memberKind, indent, unit string) string {
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
		prefix = append(prefix, "ref")
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
	if op.BodyExpression != "" {
		return writeBodyExpression(header, op.BodyExpression, indent, unit)
	}
	return header + ";"
}

func writeBodyExpression(header, expression, indent, unit string) string {
	lines := strings.Split(strings.ReplaceAll(expression, "\r\n", "\n"), "\n")
	if len(lines) == 1 {
		return header + " { " + strings.TrimSpace(lines[0]) + " }"
	}
	for i := range lines {
		lines[i] = strings.TrimRight(lines[i], " \t\r")
	}
	common := ""
	hasCommon := false
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		leading := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
		if !hasCommon {
			common = leading
			hasCommon = true
			continue
		}
		for !strings.HasPrefix(leading, common) && common != "" {
			common = common[:len(common)-1]
		}
	}
	for i, line := range lines {
		if strings.TrimSpace(line) == "" {
			lines[i] = ""
			continue
		}
		lines[i] = strings.TrimPrefix(line, common)
	}
	for len(lines) > 0 && lines[0] == "" {
		lines = lines[1:]
	}
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	var text strings.Builder
	text.WriteString(header)
	text.WriteString(" {\n")
	for i, line := range lines {
		if line != "" {
			text.WriteString(indent)
			text.WriteString(unit)
			text.WriteString(line)
		}
		if i+1 < len(lines) {
			text.WriteByte('\n')
		}
	}
	text.WriteByte('\n')
	text.WriteString(indent)
	text.WriteByte('}')
	return text.String()
}

func (m Model) memberIndentUnit(owner ast.Node) string {
	ownerIndent := lineIndent(m.Source.Bytes(), owner.Span().Offset)
	memberIndent := m.memberIndent(owner.Span())
	if strings.HasPrefix(memberIndent, ownerIndent) && len(memberIndent) > len(ownerIndent) {
		return memberIndent[len(ownerIndent):]
	}
	return memberIndentStyle(m.Source.Bytes())
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
			if len(members) > 0 && isCalculationResultMember(members[len(members)-1]) {
				return m.memberInsertionBeforeResult(members[len(members)-1], text)
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

func isCalculationResultMember(member ast.Node) bool {
	if ast.IsExpression(member) {
		return true
	}
	condition, ok := member.(*ast.ConstraintMember)
	return ok && condition.Keyword == "" && condition.Expression != nil
}

func (m Model) memberInsertionBeforeResult(result ast.Node, text string) insertion {
	content := m.Source.Bytes()
	offset := result.Span().Offset
	lineStart := offset
	for lineStart > 0 && content[lineStart-1] != '\n' {
		lineStart--
	}
	leading := content[lineStart:offset]
	if onlyWhitespace(leading) {
		anchor := precedingFullLineTriviaStart(content, lineStart)
		return insertion{
			span: source.Span{Offset: anchor},
			text: string(leading) + text + "\n",
			at:   len(leading),
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

func precedingFullLineTriviaStart(content []byte, lineStart int) int {
	anchor := lineStart
	inBlockComment := false
	blockEndAnchor := lineStart
	for anchor > 0 {
		lineEnd := anchor - 1
		previousLine := lineEnd
		for previousLine > 0 && content[previousLine-1] != '\n' {
			previousLine--
		}
		line := strings.TrimSpace(string(content[previousLine:lineEnd]))
		if inBlockComment {
			if opening := strings.LastIndex(line, "/*"); opening >= 0 {
				if strings.TrimSpace(line[:opening]) != "" {
					anchor = blockEndAnchor
					break
				}
				inBlockComment = false
			} else if closing := strings.LastIndex(line, "*/"); closing >= 0 {
				after := strings.TrimSpace(line[closing+2:])
				if after != "" && !strings.HasPrefix(after, "//") {
					anchor = blockEndAnchor
					break
				}
			}
			anchor = previousLine
			continue
		}
		if line == "" || strings.HasPrefix(line, "//") {
			anchor = previousLine
			continue
		}
		opening := strings.Index(line, "/*")
		if opening >= 0 {
			if strings.TrimSpace(line[:opening]) != "" {
				break
			}
			closing := strings.Index(line[opening+2:], "*/")
			if closing < 0 {
				inBlockComment = true
				blockEndAnchor = anchor
				anchor = previousLine
				continue
			}
			after := strings.TrimSpace(line[opening+closing+4:])
			if after == "" || strings.HasPrefix(after, "//") {
				anchor = previousLine
				continue
			}
			break
		}
		if closing := strings.LastIndex(line, "*/"); closing >= 0 &&
			strings.TrimSpace(line[closing+2:]) == "" {
			inBlockComment = true
			blockEndAnchor = anchor
			anchor = previousLine
			continue
		}
		break
	}
	return anchor
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
	case *ast.SubstateMember:
		return d.Span(), false
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
	return base + memberIndentStyle(content)
}

func memberIndentStyle(content []byte) string {
	if strings.Contains(string(content), "\t") {
		return "\t"
	}
	return "    "
}
