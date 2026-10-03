package codegen

import (
	"fmt"
	"strings"
)

// goEnumPrelude is the enumeration-literal runtime of a generated Go program.
const goEnumPrelude = `
// sysmlEnum is an enumeration literal, identified by its index in sysmlLiterals.
type sysmlEnum int32

// sysmlEnumParser reads a literal of the enumeration whose literals are
// sysmlLiterals[base:base+n], written as printed or by its qualified name.
func sysmlEnumParser(base, n int, enum string) func(s, name string) sysmlEnum {
	return func(s, name string) sysmlEnum {
		for i := base; i < base+n; i++ {
			if s == sysmlLiterals[i] || s == sysmlLiteralNames[i] {
				return sysmlEnum(i)
			}
		}
		fmt.Fprintf(os.Stderr, "argument %s: %s is not a literal of %s\n", name, s, enum)
		os.Exit(2)
		return 0
	}
}

// sysmlRefused fails with msg where a value of type T is expected.
func sysmlRefused[T any](msg string) T {
	sysmlFail(msg)
	var zero T
	return zero
}

// sysmlIgnore evaluates an operand whose value a failure does not describe.
func sysmlIgnore(any) string { return "" }
`

// goEnumTables declares the printed and the qualified name of every literal.
func goEnumTables(p *Program) string {
	var printed, qualified []string
	for _, enum := range p.Enums {
		for i, lit := range enum.Literals {
			printed = append(printed, fmt.Sprintf("%q", enum.Literal(i)))
			qualified = append(qualified, fmt.Sprintf("%q", enum.Name+"::"+lit))
		}
	}
	return fmt.Sprintf("\nvar sysmlLiterals = []string{%s}\n\nvar sysmlLiteralNames = []string{%s}\n",
		strings.Join(printed, ", "), strings.Join(qualified, ", "))
}

// goEnumParser is the argument parser of a parameter of enumeration type t.
func goEnumParser(t Type) string {
	e := t.Elem().Enum
	return fmt.Sprintf("sysmlEnumParser(%d, %d, %q)", e.Base, len(e.Literals), e.Name)
}

// refusal emits a Refusal: its operands evaluated left to right, then its failure.
func (e *goEmitter) refusal(x Refusal) string {
	var msg []string
	for i, op := range x.Operands {
		v := e.expr(op)
		switch {
		case !x.Describe:
			msg = append(msg, "sysmlIgnore("+v+")")
			continue
		case op.Type().IsEnum():
			msg = append(msg, fmt.Sprintf("%q", x.Parts[i]+"the enumeration literal "), "sysmlLiterals["+v+"]")
		case op.Type().IsFn():
			msg = append(msg, fmt.Sprintf("%q", x.Parts[i]+"the function "), "sysmlFnNames["+v+".c]")
		default:
			msg = append(msg, fmt.Sprintf("%q", x.Parts[i]), "sysmlElemKind("+v+")")
		}
	}
	if x.Describe {
		msg = append(msg, fmt.Sprintf("%q", x.Parts[len(x.Operands)]))
	} else {
		msg = append(msg, fmt.Sprintf("%q", strings.Join(x.Parts, "")))
	}
	return fmt.Sprintf("sysmlRefused[%s](%s)", goType(x.T), strings.Join(msg, " + "))
}

// cEnumRuntime is the enumeration-literal runtime of a generated C program:
// the literal tables, and a reader of each enumeration's literals.
func cEnumRuntime(p *Program) string {
	var printed, qualified []string
	for _, enum := range p.Enums {
		for i, lit := range enum.Literals {
			printed = append(printed, cString(enum.Literal(i)))
			qualified = append(qualified, cString(enum.Name+"::"+lit))
		}
	}
	printed, qualified = append(printed, "NULL"), append(qualified, "NULL")
	var b strings.Builder
	fmt.Fprintf(&b, `
/* An enumeration literal, identified by its index in sysml_literals. */
typedef int32_t sysml_enum;
static const char *const sysml_literals[] = {%s};
static const char *const sysml_literal_names[] = {%s};

static void sysml_print_enum_value(sysml_enum v) { fputs(sysml_literals[v], stdout); }
static void sysml_print_enum(sysml_enum v) { puts(sysml_literals[v]); }
static void sysml_format_enum(sysml_enum v, char *out, size_t size) { snprintf(out, size, "%%s", sysml_literals[v]); }
static const char *sysml_show_enum(sysml_enum v) { return sysml_literals[v]; }

/* Reads a literal of sysml_literals[base, base+n), written as printed or by its qualified name. */
static sysml_enum sysml_parse_literal(const char *s, const char *name, int base, int n, const char *enumeration) {
	for (int i = base; i < base + n; i++)
		if (!strcmp(s, sysml_literals[i]) || !strcmp(s, sysml_literal_names[i])) return (sysml_enum)i;
	fprintf(stderr, "argument %%s: %%s is not a literal of %%s\n", name, s, enumeration);
	exit(2);
}
`, strings.Join(printed, ", "), strings.Join(qualified, ", "))
	for _, enum := range p.Enums {
		fmt.Fprintf(&b, "static sysml_enum sysml_parse_%s(const char *s, const char *name) { return sysml_parse_literal(s, name, %d, %d, %s); }\n",
			cSeqSuffix(EnumType(enum)), enum.Base, len(enum.Literals), cString(enum.Name))
	}
	return b.String()
}

// cEnumSeqRuntime instantiates the collection runtime over each enumeration's
// literals, and the String a literal prints as.
func cEnumSeqRuntime(p *Program) string {
	var b strings.Builder
	b.WriteString("#define SYSML_KIND_ENUM(v) \"enumeration literal\"\n")
	for _, enum := range p.Enums {
		sfx := cSeqSuffix(EnumType(enum))
		r := strings.NewReplacer("ELEMNAME", sfx, "ELEM", "sysml_enum", "SFX", sfx, "PRINT", "sysml_print_enum_value",
			"FORMAT", "sysml_format_enum", "KINDOF", "SYSML_KIND_ENUM", "KEY", "(uint64_t)", "SKIP", "false && ",
			"EQ", "SYSML_SCALAR_EQ", "SHOW", "sysml_show_enum", "SAVEELEMS", "SYSML_NO_ELEMS", "RESTOREELEMS", "SYSML_NO_ELEMS")
		b.WriteString(r.Replace(cSeqTemplate))
	}
	b.WriteString("\nstatic sysml_str sysml_literal_str(sysml_enum v) { return (sysml_str){(sysml_int)strlen(sysml_literals[v]), sysml_literals[v]}; }\n")
	return b.String()
}

// refusal emits a Refusal: its operands evaluated left to right, then its failure.
func (e *cEmitter) refusal(x Refusal) string {
	return e.sequenced(x.Operands, func(v []string) string {
		var args []string
		var ignored strings.Builder
		for i, op := range x.Operands {
			switch {
			case !x.Describe:
				fmt.Fprintf(&ignored, "(void)(%s), ", v[i])
			case op.Type().IsEnum():
				args = append(args, cString(x.Parts[i]+"the enumeration literal "), "sysml_literals["+v[i]+"]")
			case op.Type().IsFn():
				args = append(args, cString(x.Parts[i]+"the function "), "sysml_fn_names["+v[i]+".c]")
			case op.Type() == TypeNum:
				args = append(args, cString(x.Parts[i]), "sysml_num_kind("+v[i]+")")
			default:
				args = append(args, cString(x.Parts[i]), cString(map[Type]string{TypeInt: "an Integer", TypeReal: "a Real", TypeBool: "a Boolean"}[op.Type()]))
			}
		}
		if x.Describe {
			args = append(args, cString(x.Parts[len(x.Operands)]))
		} else {
			args = append(args, cString(strings.Join(x.Parts, "")))
		}
		return fmt.Sprintf("(%ssysml_failf(\"%s\", %s), %s)", ignored.String(), strings.Repeat("%s", len(args)), strings.Join(args, ", "), cZero(x.T))
	})
}
