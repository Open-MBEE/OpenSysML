package edit

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/core/lexer"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

// connectionKind is one connector-like usage kind: the languages that write it,
// whether it may state a type after its name, and how it writes its two ends.
type connectionKind struct {
	languages map[source.Kind]bool
	typed     bool
	ends      func(lang source.Kind, from, to string) string
}

var (
	sysmlOnly  = map[source.Kind]bool{source.KindSysML: true}
	kermlOnly  = map[source.Kind]bool{source.KindKerML: true}
	bothLangs  = map[source.Kind]bool{source.KindSysML: true, source.KindKerML: true}
	connectEnd = func(_ source.Kind, from, to string) string { return "connect " + from + " to " + to }
	fromToEnd  = func(_ source.Kind, from, to string) string { return "from " + from + " to " + to }
	firstEnd   = func(_ source.Kind, from, to string) string { return "first " + from + " then " + to }
)

// connectionKinds are the usages an OpAddConnection can write. A binding is
// `bind a = b` in SysML and `of a = b` in KerML.
var connectionKinds = map[string]connectionKind{
	"connection": {languages: sysmlOnly, typed: true, ends: connectEnd},
	"interface":  {languages: sysmlOnly, typed: true, ends: connectEnd},
	"allocation": {languages: sysmlOnly, typed: true,
		ends: func(_ source.Kind, from, to string) string { return "allocate " + from + " to " + to }},
	"binding": {languages: bothLangs,
		ends: func(lang source.Kind, from, to string) string {
			if lang == source.KindKerML {
				return "of " + from + " = " + to
			}
			return "bind " + from + " = " + to
		}},
	"flow":       {languages: bothLangs, typed: true, ends: fromToEnd},
	"succession": {languages: bothLangs, ends: firstEnd},
	"transition": {languages: sysmlOnly, ends: firstEnd},
	"connector":  {languages: kermlOnly, typed: true, ends: fromToEnd},
}

// ConnectionKinds lists the connection kinds legal in a language, sorted.
func ConnectionKinds(lang source.Kind) []string {
	return legalKinds(lang, func(name string) map[source.Kind]bool { return connectionKinds[name].languages },
		mapKeys(connectionKinds))
}

func (m Model) addConnectionSplice(i int, op Operation) (splice, error) {
	lang := m.Source.Kind()
	kind, ok := connectionKinds[op.MemberKind]
	if !ok || !kind.languages[lang] {
		return splice{}, &Error{
			Failure:        FailureIllegalKind,
			OperationIndex: i,
			Message: fmt.Sprintf("connection kind %q is not legal in %s source %q",
				op.MemberKind, lang, m.Source.Name()),
		}
	}
	if op.MemberName != "" {
		if err := checkName(i, op.MemberName); err != nil {
			e := err.(*Error)
			e.Message = fmt.Sprintf("connection name %q is not an identifier", op.MemberName)
			return splice{}, e
		}
	}
	if !kind.typed && op.Type != "" {
		return splice{}, &Error{
			Failure:        FailureIllegalKind,
			OperationIndex: i,
			Message:        fmt.Sprintf("connection kind %q cannot carry a typing target", op.MemberKind),
		}
	}
	if err := checkEnd(i, "from", op.From); err != nil {
		return splice{}, err
	}
	if err := checkEnd(i, "to", op.To); err != nil {
		return splice{}, err
	}
	owner, ownerScope, err := m.addOwner(op.Owner)
	if err != nil {
		e := err.(*Error)
		e.OperationIndex = i
		return splice{}, e
	}
	if op.MemberName != "" && ownerScope != nil && len(ownerScope.LookupLocalAll(op.MemberName)) > 0 {
		return splice{}, &Error{
			Failure:        FailureMemberNameTaken,
			OperationIndex: i,
			Message:        fmt.Sprintf("%s already declares %q", op.Owner, op.MemberName),
		}
	}
	text := writeConnection(op, kind, lang)
	span, replacement := m.memberInsertion(owner, text)
	return splice{span: span, text: replacement, opIndex: i, target: op.Owner}, nil
}

func writeConnection(op Operation, kind connectionKind, lang source.Kind) string {
	header := op.MemberKind
	if op.MemberName != "" {
		header += " " + op.MemberName
	}
	if op.Type != "" {
		header += " : " + op.Type
	}
	return header + " " + kind.ends(lang, op.From, op.To) + ";"
}

// checkEnd refuses a connection end that is not written as a feature reference:
// names joined by `.` or `::`. Whether it names anything is answered by
// analyzing the edited model, where it has a scope.
func checkEnd(i int, role, end string) error {
	refuse := func(reason string) error {
		return &Error{
			Failure:        FailureInvalidName,
			OperationIndex: i,
			Message:        fmt.Sprintf("connection end %q (%s) %s", end, role, reason),
		}
	}
	if end == "" {
		return refuse("is empty")
	}
	lx := lexer.New(source.New("<end>", []byte(end)))
	wantName := true
	for tok := lx.Next(); tok.Kind != lexer.EOF; tok = lx.Next() {
		switch {
		case wantName && (tok.Kind == lexer.Identifier || tok.Kind == lexer.UnrestrictedName):
			if tok.Unterminated {
				return refuse("is an unterminated quoted name")
			}
		case !wantName && (tok.Kind == lexer.Dot || tok.Kind == lexer.ColonColon):
		default:
			return refuse("is not a feature reference")
		}
		wantName = !wantName
	}
	if wantName {
		return refuse("is not a feature reference")
	}
	return nil
}
