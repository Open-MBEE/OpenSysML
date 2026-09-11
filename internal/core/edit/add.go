package edit

import (
	"fmt"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/lexer"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

type memberKind struct {
	languages  map[source.Kind]bool
	definition bool
	typed      bool
}

var memberKinds = map[string]memberKind{
	"package":       {languages: map[source.Kind]bool{source.KindSysML: true, source.KindKerML: true}},
	"part def":      {languages: map[source.Kind]bool{source.KindSysML: true}, definition: true},
	"part":          {languages: map[source.Kind]bool{source.KindSysML: true}, typed: true},
	"attribute def": {languages: map[source.Kind]bool{source.KindSysML: true}, definition: true},
	"attribute":     {languages: map[source.Kind]bool{source.KindSysML: true}, typed: true},
	"item def":      {languages: map[source.Kind]bool{source.KindSysML: true}, definition: true},
	"item":          {languages: map[source.Kind]bool{source.KindSysML: true}, typed: true},
	"port def":      {languages: map[source.Kind]bool{source.KindSysML: true}, definition: true},
	"port":          {languages: map[source.Kind]bool{source.KindSysML: true}, typed: true},
	"enum def":      {languages: map[source.Kind]bool{source.KindSysML: true}, definition: true},
	"calc def":      {languages: map[source.Kind]bool{source.KindSysML: true}, definition: true},
	"calc":          {languages: map[source.Kind]bool{source.KindSysML: true}, typed: true},
	"class":         {languages: map[source.Kind]bool{source.KindKerML: true}, definition: true},
	"struct":        {languages: map[source.Kind]bool{source.KindKerML: true}, definition: true},
	"datatype":      {languages: map[source.Kind]bool{source.KindKerML: true}, definition: true},
	"classifier":    {languages: map[source.Kind]bool{source.KindKerML: true}, definition: true},
	"feature":       {languages: map[source.Kind]bool{source.KindKerML: true}, typed: true},
	"assoc":         {languages: map[source.Kind]bool{source.KindKerML: true}, definition: true},
	"association":   {languages: map[source.Kind]bool{source.KindKerML: true}, definition: true},
	"behavior":      {languages: map[source.Kind]bool{source.KindKerML: true}, definition: true},
	"function":      {languages: map[source.Kind]bool{source.KindKerML: true}, definition: true},
	"predicate":     {languages: map[source.Kind]bool{source.KindKerML: true}, definition: true},
	"interaction":   {languages: map[source.Kind]bool{source.KindKerML: true}, definition: true},
	"metaclass":     {languages: map[source.Kind]bool{source.KindKerML: true}, definition: true},
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
	if err := checkName(i, op.MemberName); err != nil {
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
	if !kind.typed && op.Type != "" {
		return splice{}, &Error{
			Failure:        FailureIllegalKind,
			OperationIndex: i,
			Message:        fmt.Sprintf("kind %q cannot carry a typing target", op.MemberKind),
		}
	}
	if !kind.definition && len(op.Specializes) > 0 {
		return splice{}, &Error{
			Failure:        FailureIllegalKind,
			OperationIndex: i,
			Message:        fmt.Sprintf("kind %q is a usage and cannot carry specializes targets", op.MemberKind),
		}
	}
	owner, ownerScope, err := m.addOwner(op.Owner)
	if err != nil {
		e := err.(*Error)
		e.OperationIndex = i
		return splice{}, e
	}
	if ownerScope != nil && len(ownerScope.LookupLocalAll(op.MemberName)) > 0 {
		return splice{}, &Error{
			Failure:        FailureMemberNameTaken,
			OperationIndex: i,
			Message:        fmt.Sprintf("%s already declares %q", op.Owner, op.MemberName),
		}
	}
	text := writeMember(op, kind)
	span, replacement := m.memberInsertion(owner, text)
	return splice{span: span, text: replacement, opIndex: i, target: op.Owner}, nil
}

func (m Model) addOwner(fqn string) (ast.Node, *symbols.Scope, error) {
	rootScope := m.Index.DocumentRoot(m.Source.Name())
	if fqn == "" {
		return m.Root, rootScope, nil
	}
	syms := m.Index.LookupQualifiedFrom(fqn, fqn)
	var local *symbols.Symbol
	for _, sym := range syms {
		if sym != nil && m.Index.GetFQN(sym) == fqn && sym.DocName == m.Source.Name() {
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
		if usage, ok := local.Decl.(*ast.Usage); ok && !usage.HasBody {
			return nil, nil, &Error{Failure: FailureOwnerNotNamespace,
				Message: fmt.Sprintf("%q cannot contain members without a body", fqn)}
		}
		return local.Decl, local.Scope, nil
	default:
		return nil, nil, &Error{Failure: FailureOwnerNotNamespace,
			Message: fmt.Sprintf("%q cannot contain members", fqn)}
	}
}

func writeMember(op Operation, kind memberKind) string {
	header := op.MemberKind + " " + op.MemberName
	if kind.definition && len(op.Specializes) > 0 {
		header += " specializes " + strings.Join(op.Specializes, ", ")
	} else if !kind.definition && op.Type != "" {
		header += " : " + op.Type
	}
	if op.Multiplicity != "" {
		header += " " + op.Multiplicity
	}
	if op.Value != "" {
		header += " = " + op.Value
	}
	return header + ";"
}

func (m Model) memberInsertion(owner ast.Node, text string) (source.Span, string) {
	if owner == m.Root {
		prefix := ""
		if len(m.Source.Bytes()) > 0 && m.Source.Bytes()[len(m.Source.Bytes())-1] != '\n' {
			prefix = "\n"
		}
		return source.Span{Offset: m.Source.Len()}, prefix + text + "\n"
	}
	body, hasBody := bodyInfo(owner)
	ownerIndent := lineIndent(m.Source.Bytes(), owner.Span().Offset)
	indent := m.memberIndent(owner.Span())
	if hasBody {
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
		return source.Span{Offset: lineStart, Len: closeOffset - lineStart},
			prefix + indent + text + "\n" + closeIndent
	}
	semi := lastToken(m.Source, owner.Span(), lexer.Semicolon)
	return source.Span{Offset: semi.Span.Offset, Len: semi.Span.Len},
		" {\n" + indent + text + "\n" + ownerIndent + "}"
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
