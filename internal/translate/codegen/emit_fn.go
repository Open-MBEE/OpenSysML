package codegen

import (
	"fmt"
	"strings"
)

// goFnPrelude is the function-value runtime of a generated Go program.
const goFnPrelude = `
// sysmlFn is a function value: the function it is, an index into sysmlFnNames,
// and for a closure the run of the body declaring it and what it captures.
type sysmlFn struct {
	c   int32
	run int64
	env *[]any
}

// sysmlFnKey identifies a function value as '==' does.
type sysmlFnKey struct {
	c   int32
	run int64
}

var sysmlRuns int64

// sysmlNextRun is the identity of a run of a body declaring a closure.
func sysmlNextRun() int64 {
	sysmlRuns++
	return sysmlRuns
}

func sysmlFnEq(a, b sysmlFn) bool { return a.c == b.c && a.run == b.run }
`

// goFnTables is the printed name of every function a value may be.
func goFnTables(p *Program) string {
	names := make([]string, len(p.FnCases))
	for i, k := range p.FnCases {
		names[i] = fmt.Sprintf("%q", k.Name)
	}
	return fmt.Sprintf("\nvar sysmlFnNames = []string{%s}\n", strings.Join(names, ", "))
}

// fnExpr emits a function-value expression; ok is false for any other node.
func (e *goEmitter) fnExpr(x Expr) (string, bool) {
	switch x := x.(type) {
	case FnLit:
		if !x.Case.Closure {
			return fmt.Sprintf("sysmlFn{c: %d}", x.Case.ID), true
		}
		env := ""
		if len(x.Env) > 0 {
			values := make([]string, len(x.Env))
			for i, v := range x.Env {
				values[i] = e.expr(v)
			}
			env = fmt.Sprintf(", env: &[]any{%s}", strings.Join(values, ", "))
		}
		return fmt.Sprintf("sysmlFn{c: %d, run: %s%s}", x.Case.ID, e.expr(x.Run), env), true
	case FnWiden:
		return e.expr(x.X), true
	case FnEnv:
		return fmt.Sprintf("(*%s.env)[%d].(%s)", goLocal(x.Name), x.I, goType(x.T)), true
	case FnDispatch:
		h := goLocal(x.Name)
		var b strings.Builder
		fmt.Fprintf(&b, "func() %s { %s := %s; _ = %[2]s; switch %[2]s.c { ", goType(x.T), h, e.expr(x.F))
		for i, k := range x.F.Type().Fns.Cases {
			fmt.Fprintf(&b, "case %d: return %s; ", k.ID, e.expr(x.Cases[i]))
		}
		b.WriteString("}; panic(\"unreachable\") }()")
		return b.String(), true
	}
	return "", false
}

// cFnRuntime is the function-value runtime of a generated C program: a value
// holds its captured bindings inline, so it outlives any arena release.
func cFnRuntime(p *Program) string {
	if len(p.FnCases) == 0 {
		return ""
	}
	env := 1
	names := make([]string, len(p.FnCases))
	for i, k := range p.FnCases {
		names[i] = cString(k.Name)
		env = max(env, len(k.Env))
	}
	return fmt.Sprintf(`
typedef union { sysml_int i; sysml_real r; sysml_bool b; sysml_num n; sysml_enum e; int64_t run; } sysml_cap;
/* A function value: the function it is, an index into sysml_fn_names, and for a
   closure the run of the body declaring it and what it captures. */
typedef struct { int32_t c; int64_t run; sysml_cap env[%d]; } sysml_fn;
static const char *const sysml_fn_names[] = {%s};
static int64_t sysml_runs;
static inline int64_t sysml_next_run(void) { return ++sysml_runs; }
static inline bool sysml_fn_eq(sysml_fn a, sysml_fn b) { return a.c == b.c && a.run == b.run; }
static inline uint64_t sysml_fn_key(sysml_fn f) { return ((uint64_t)f.run * 0x100000001B3ULL) ^ (uint64_t)f.c; }
static void sysml_print_fn_value(sysml_fn f) { fputs(sysml_fn_names[f.c], stdout); }
static void sysml_print_fn(sysml_fn f) { puts(sysml_fn_names[f.c]); }
static void sysml_format_fn(sysml_fn f, char *out, size_t size) { snprintf(out, size, "%%s", sysml_fn_names[f.c]); }
static const char *sysml_show_fn(sysml_fn f) { return sysml_fn_names[f.c]; }
static sysml_fn sysml_parse_fn(const char *s, const char *name) { fprintf(stderr, "argument %%s: %%s is not a function value\n", name, s); exit(2); }
`, env, strings.Join(names, ", "))
}

// cFnSeqRuntime instantiates the collection runtime over function values.
func cFnSeqRuntime(p *Program) string {
	if len(p.FnCases) == 0 {
		return ""
	}
	r := strings.NewReplacer("ELEMNAME", "fn", "ELEM", "sysml_fn", "SFX", "fn", "PRINT", "sysml_print_fn_value",
		"FORMAT", "sysml_format_fn", "KINDOF", "SYSML_KIND_FN", "KEY", "sysml_fn_key", "SKIP", "SYSML_NEVER",
		"EQ", "sysml_fn_eq", "SHOW", "sysml_show_fn", "SAVEELEMS", "SYSML_NO_ELEMS", "RESTOREELEMS", "SYSML_NO_ELEMS")
	return "#define SYSML_KIND_FN(v) \"function\"\n" + r.Replace(cSeqTemplate)
}

// cCapField is the member of sysml_cap a captured binding of type t is held in.
func cCapField(t Type) string {
	switch {
	case t.IsEnum():
		return "e"
	case t == TypeInt:
		return "i"
	case t == TypeReal:
		return "r"
	case t == TypeBool:
		return "b"
	case t == TypeNum:
		return "n"
	}
	return "run"
}

// fnExpr emits a function-value expression; ok is false for any other node.
func (e *cEmitter) fnExpr(x Expr) (string, bool) {
	switch x := x.(type) {
	case FnLit:
		if !x.Case.Closure {
			return fmt.Sprintf("((sysml_fn){.c = %d})", x.Case.ID), true
		}
		env := ""
		if len(x.Env) > 0 {
			values := make([]string, len(x.Env))
			for i, v := range x.Env {
				values[i] = fmt.Sprintf("{.%s = %s}", cCapField(v.Type()), e.expr(v))
			}
			env = fmt.Sprintf(", .env = {%s}", strings.Join(values, ", "))
		}
		return fmt.Sprintf("((sysml_fn){.c = %d, .run = %s%s})", x.Case.ID, e.expr(x.Run), env), true
	case FnWiden:
		return e.expr(x.X), true
	case FnEnv:
		return fmt.Sprintf("%s.env[%d].%s", cLocal(x.Name), x.I, cCapField(x.T)), true
	case FnDispatch:
		e.temps++
		r := fmt.Sprintf("sysml_t%d", e.temps)
		h := cLocal(x.Name)
		var b strings.Builder
		fmt.Fprintf(&b, "({ sysml_fn %[1]s = %[2]s; (void)%[1]s; %[3]s %[4]s = %[5]s; switch (%[1]s.c) { ", h, e.expr(x.F), cType(x.T), r, cZero(x.T))
		for i, k := range x.F.Type().Fns.Cases {
			fmt.Fprintf(&b, "case %d: %s = %s; break; ", k.ID, r, e.expr(x.Cases[i]))
		}
		fmt.Fprintf(&b, "} %s; })", r)
		return b.String(), true
	}
	return "", false
}
