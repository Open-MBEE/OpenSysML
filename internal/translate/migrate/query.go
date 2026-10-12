package migrate

import (
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/translate/xmi/sysmlv1"
)

// qx is a DocumentQueries expression to write: a library operation applied
// to named arguments, or a literal written as is.
type qx struct {
	op     string
	args   []qarg
	lit    string
	shared *qx
}

// qarg is one named argument: a single value or a list.
type qarg struct {
	name string
	val  qx
	list []qx
	many bool
}

func qlit(s string) qx { return qx{lit: s} }
func qstr(s string) qx { return qlit(stringLiteral(s)) }
func qint(n int) qx    { return qlit(strconv.Itoa(n)) }
func qcall(op string, args ...qarg) qx {
	return qx{op: op, args: args}
}

// qshared marks an expression for query-scope hoisting.
func qshared(src qx) qx {
	if !src.isCall() {
		return src
	}
	return qx{shared: &src}
}

// qempty is the empty sequence, a row set with no rows that walks no scope.
func qempty() qx { return qlit("()") }

func qarg1(name string, v qx) qarg     { return qarg{name: name, val: v} }
func qlist(name string, vs ...qx) qarg { return qarg{name: name, list: vs, many: true} }

// qstrs writes a list argument of string literals.
func qstrs(name string, ss ...string) qarg {
	vs := make([]qx, len(ss))
	for i, s := range ss {
		vs[i] = qstr(s)
	}
	return qlist(name, vs...)
}

// isCall reports whether the expression is an operation or a shared call.
func (q qx) isCall() bool { return q.shared != nil || q.op != "" }

func (q qx) unshared() qx {
	for q.shared != nil {
		q = *q.shared
	}
	return q
}

// flat reports whether the expression and its arguments hold no nested
// operation, so it fits on one line.
func (q qx) flat() bool {
	q = q.unshared()
	for _, a := range q.args {
		if a.val.isCall() {
			return false
		}
		for _, v := range a.list {
			if v.isCall() {
				return false
			}
		}
	}
	return true
}

// lines writes the expression, its operations qualified by prefix, as lines
// to indent one level deeper for each nested argument.
func (q qx) lines(prefix string) []string {
	q = q.unshared()
	if !q.isCall() {
		return []string{q.lit}
	}
	if q.flat() {
		parts := make([]string, len(q.args))
		for i, a := range q.args {
			parts[i] = a.name + " = " + a.flatValue(prefix)
		}
		return []string{prefix + q.op + "(" + strings.Join(parts, ", ") + ")"}
	}
	out := []string{prefix + q.op + "("}
	for i, a := range q.args {
		ls := a.lines(prefix)
		ls[0] = a.name + " = " + ls[0]
		if i < len(q.args)-1 {
			ls[len(ls)-1] += ","
		}
		out = append(out, indentLines(ls)...)
	}
	out[len(out)-1] += ")"
	return out
}

// text writes the expression on one line, its operations qualified by prefix.
func (q qx) text(prefix string) string {
	q = q.unshared()
	if !q.isCall() {
		return q.lit
	}
	parts := make([]string, len(q.args))
	for i, a := range q.args {
		parts[i] = a.name + " = " + a.text(prefix)
	}
	return prefix + q.op + "(" + strings.Join(parts, ", ") + ")"
}

// text writes an argument's value on one line.
func (a qarg) text(prefix string) string {
	if !a.many {
		return a.val.text(prefix)
	}
	parts := make([]string, len(a.list))
	for i, v := range a.list {
		parts[i] = v.text(prefix)
	}
	return "(" + strings.Join(parts, ", ") + ")"
}

// flatValue writes an argument whose value holds no operation.
func (a qarg) flatValue(prefix string) string {
	if !a.many {
		return a.val.lines(prefix)[0]
	}
	parts := make([]string, len(a.list))
	for i, v := range a.list {
		parts[i] = v.lines(prefix)[0]
	}
	return "(" + strings.Join(parts, ", ") + ")"
}

// lines writes an argument's value, a list as one element per line.
func (a qarg) lines(prefix string) []string {
	if !a.many {
		return a.val.lines(prefix)
	}
	flat := true
	for _, v := range a.list {
		if v.isCall() {
			flat = false
		}
	}
	if flat {
		return []string{a.flatValue(prefix)}
	}
	out := []string{"("}
	for i, v := range a.list {
		ls := v.lines(prefix)
		if i < len(a.list)-1 {
			ls[len(ls)-1] += ","
		}
		out = append(out, indentLines(ls)...)
	}
	out[len(out)-1] += ")"
	return out
}

func indentLines(ls []string) []string {
	out := make([]string, len(ls))
	for i, l := range ls {
		out[i] = "    " + l
	}
	return out
}

// queryPrefix is how the DocumentQueries library is named from inside host:
// from the global namespace when a member of host's scopes, or of a
// synthesized declaration being written, shadows it.
func (m *migration) queryPrefix(host *sysmlv1.Element) string {
	if m.hidden("DocumentQueries") || m.shadowsLibrary("DocumentQueries", host) {
		return "$::DocumentQueries::"
	}
	return "DocumentQueries::"
}

// queryElementType is how KerML::Root::Element is named from inside host.
func (m *migration) queryElementType(host *sysmlv1.Element) string {
	if m.hidden("KerML") || m.shadowsLibrary("KerML", host) {
		return "$::KerML::Root::Element"
	}
	return "KerML::Root::Element"
}

// queryParameter is an expression hoisted into a query definition.
type queryParameter struct {
	name  string
	value qx
}

// hoistShared rewrites marked subexpressions as query parameters.
func hoistShared(body qx, prefix string) (qx, []queryParameter) {
	var parameters []queryParameter
	names := map[string]string{}
	var rewrite func(qx) qx
	rewrite = func(q qx) qx {
		if q.shared != nil {
			value := rewrite(*q.shared)
			key := value.text(prefix)
			if name, ok := names[key]; ok {
				return qlit(name)
			}
			name := "candidates"
			if len(parameters) > 0 {
				name += strconv.Itoa(len(parameters) + 1)
			}
			names[key] = name
			parameters = append(parameters, queryParameter{name: name, value: value})
			return qlit(name)
		}
		if len(q.args) == 0 {
			return q
		}
		args := make([]qarg, len(q.args))
		copy(args, q.args)
		q.args = args
		for i := range q.args {
			if q.args[i].many {
				q.args[i].list = append([]qx(nil), q.args[i].list...)
				for j := range q.args[i].list {
					q.args[i].list[j] = rewrite(q.args[i].list[j])
				}
			} else {
				q.args[i].val = rewrite(q.args[i].val)
			}
		}
		return q
	}
	return rewrite(body), parameters
}

// writeQueryDef writes a query definition returning the expression.
func (m *migration) writeQueryDef(name string, prefix string, host *sysmlv1.Element, body qx) {
	body, parameters := hoistShared(body, prefix)
	m.w.block("calc def "+writeName(name)+" :> "+prefix+"Query", func() {
		for _, parameter := range parameters {
			lines := parameter.value.lines(prefix)
			lines[0] = "in " + parameter.name + " : " + m.queryElementType(host) + "[0..*] ordered = " + lines[0]
			lines[len(lines)-1] += ";"
			m.w.lines(lines)
		}
		m.w.lines(body.lines(prefix))
	})
}
