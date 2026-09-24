package migrate

import (
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/translate/xmi/sysmlv1"
)

// qx is a DocumentQueries expression to write: a library operation applied
// to named arguments, or a literal written as is.
type qx struct {
	op   string
	args []qarg
	lit  string
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

// isCall reports whether the expression is an operation, not a literal.
func (q qx) isCall() bool { return q.op != "" }

// flat reports whether the expression and its arguments hold no nested
// operation, so it fits on one line.
func (q qx) flat() bool {
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

// writeQueryDef writes a query definition returning the expression.
func (m *migration) writeQueryDef(name string, prefix string, body qx) {
	m.w.block("calc def "+writeName(name)+" :> "+prefix+"Query", func() {
		m.w.lines(body.lines(prefix))
	})
}
