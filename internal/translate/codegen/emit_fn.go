package codegen

import (
	"fmt"
	"strconv"
	"strings"
)

// goFnPrelude is the function-value runtime of a generated Go program.
const goFnPrelude = `
// sysmlFn is a function value: the function it is, an index into sysmlFnNames,
// and for a closure the run of the body declaring it and what it captures.
type sysmlFn struct {
	c    int32
	run  int64
	env  *[]any
	self *sysmlRec
}

// sysmlFnKey identifies a function value as '==' does.
type sysmlFnKey struct {
	c    int32
	run  int64
	self *sysmlRec
}

// sysmlRec is a data value with features, identified by itself.
type sysmlRec struct {
	t string
	f []any
}

var sysmlRuns int64

// sysmlNextRun is the identity of a run of a body declaring a closure.
func sysmlNextRun() int64 {
	sysmlRuns++
	return sysmlRuns
}

func sysmlFnEq(a, b sysmlFn) bool { return a.c == b.c && a.run == b.run && a.self == b.self }

// sysmlOpt is a value that may be unset: a required feature's materialized
// value nothing was written to, identified by u (0 for a value).
type sysmlOpt[T any] struct {
	v T
	u uint64
}

type sysmlHolder interface{ held() (any, bool) }

func (o sysmlOpt[T]) held() (any, bool) { return o.v, o.u != 0 }

var sysmlUnsets uint64

// sysmlFresh marks an unset identity no read has materialized yet.
const sysmlFresh = 1 << 63

// sysmlNewUnset is a fresh identity whose low bits are 2 more than the least
// Integer its feature's type admits (1 for any Integer).
func sysmlNewUnset(tag uint64) uint64 {
	sysmlUnsets++
	return sysmlUnsets<<2 | tag | sysmlFresh
}

// sysmlReadOpt reads the record feature value f, spending the step that
// materializes an unset one on its first read.
func sysmlReadOpt[T any](f *any) sysmlOpt[T] {
	o := (*f).(sysmlOpt[T])
	if o.u&sysmlFresh != 0 {
		o.u &^= sysmlFresh
		*f = o
		sysmlStep(1)
	}
	return o
}

// sysmlNarrowOpt checks o against the range lo of the feature it is written
// to; an unset value conforms when its own feature's range is within it,
// checked only where strict.
func sysmlNarrowOpt(o sysmlOpt[sysmlInt], lo int64, typ, where string, strict bool) sysmlOpt[sysmlInt] {
	switch {
	case o.u == 0 && where == "":
		o.v = sysmlAtLeast(o.v, lo, typ)
	case o.u == 0:
		o.v = sysmlAtLeastAt(o.v, lo, typ, where)
	case strict && int64(o.u&3)-2 < lo:
		msg := "type mismatch: cannot write <unset> (instance) to a feature typed by " + typ
		if where != "" {
			msg = where + ": " + msg
		}
		sysmlFail(msg)
	}
	return o
}

func sysmlNeed[T any](o sysmlOpt[T], msg string) T {
	if o.u != 0 {
		sysmlFail(msg)
	}
	return o.v
}
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
		if x.Self != nil {
			return fmt.Sprintf("sysmlFn{c: %d, self: %s}", x.Case.ID, e.expr(x.Self)), true
		}
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
	case FnSelf:
		return goLocal(x.Name) + ".self", true
	case RecNew:
		values := make([]string, len(x.Fields))
		for i, v := range x.Fields {
			values[i] = e.expr(v)
		}
		return fmt.Sprintf("&sysmlRec{t: %q, f: []any{%s}}", x.Rec.Short, strings.Join(values, ", ")), true
	case RecGet:
		if x.T.MayUnset() {
			return fmt.Sprintf("sysmlReadOpt[%s](&%s.f[%d])", goType(x.T.Concrete()), e.expr(x.X), x.Field), true
		}
		return fmt.Sprintf("%s.f[%d].(%s)", e.expr(x.X), x.Field, goType(x.T)), true
	case Narrowed:
		if x.X.Type().MayUnset() {
			return fmt.Sprintf("sysmlNarrowOpt(%s, %d, %q, %q, true)", e.expr(x.X), x.R.Lower(), x.R.String(), x.Where), true
		}
		return fmt.Sprintf("sysmlAtLeastAt(%s, %d, %q, %q)", e.expr(x.X), x.R.Lower(), x.R.String(), x.Where), true
	case NewUnset:
		return fmt.Sprintf("%s{u: sysmlNewUnset(%d)}", goType(x.T), unsetTag(x.R)), true
	case Lift:
		return fmt.Sprintf("%s{v: %s}", goType(x.T), e.expr(x.X)), true
	case Need:
		return fmt.Sprintf("sysmlNeed(%s, %s)", e.expr(x.X), strconv.Quote(x.Fail)), true
	case Strip:
		return fmt.Sprintf("(%s).v", e.expr(x.X)), true
	case Relabel:
		return fmt.Sprintf("%s{v: %s, u: (%s).u}", goType(x.T), e.expr(x.V), e.expr(x.Of)), true
	case IsUnset:
		return fmt.Sprintf("((%s).u != 0)", e.expr(x.X)), true
	case SameUnset:
		return fmt.Sprintf("(((%s).u == (%s).u) != %t)", e.expr(x.L), e.expr(x.R), x.Neq), true
	case Named:
		return e.expr(x.X), true
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
	if len(p.FnCases) == 0 && len(p.Records) == 0 {
		return ""
	}
	env := 1
	names := []string{cString("")}
	if len(p.FnCases) > 0 {
		names = names[:0]
	}
	for _, k := range p.FnCases {
		names = append(names, cString(k.Name))
		env = max(env, len(k.Env))
	}
	return fmt.Sprintf(`
typedef struct sysml_rec sysml_rec;
/* A value that may be unset: a required feature's materialized value nothing
   was written to, identified by u (0 for a value). */
static uint64_t sysml_unsets;
/* The low bits of an unset identity are 2 more than the least Integer its
   feature's type admits (1 for any Integer). */
/* SYSML_FRESH marks an unset identity no read has materialized yet. */
#define SYSML_FRESH ((uint64_t)1 << 63)
static inline uint64_t sysml_new_unset(uint64_t tag) { return ++sysml_unsets << 2 | tag | SYSML_FRESH; }
/* sysml_read_F reads a record feature value, spending the step that
   materializes an unset one on its first read. */
#define SYSML_OPT(F, T) \
	typedef struct { T v; uint64_t u; } sysml_opt_##F; \
	static inline T sysml_need_##F(sysml_opt_##F o, const char *msg) { if (o.u) sysml_fail(msg); return o.v; } \
	static inline sysml_opt_##F sysml_read_##F(sysml_opt_##F *o) { \
		if (o->u & SYSML_FRESH) { o->u &= ~SYSML_FRESH; sysml_step(1); } \
		return *o; \
	}
SYSML_OPT(i, sysml_int)
/* sysml_narrow_opt checks o against the range lo of the feature it is written
   to; an unset value conforms when its own feature's range is within it,
   checked only where strict. */
static inline sysml_opt_i sysml_narrow_opt(sysml_opt_i o, sysml_int lo, const char *type, const char *where, bool strict) {
	if (!o.u) {
		o.v = where ? sysml_at_least_at(o.v, lo, type, where) : sysml_at_least(o.v, lo, type);
	} else if (strict && (int64_t)(o.u & 3) - 2 < lo) {
		static char msg[512];
		snprintf(msg, sizeof msg, "%%s%%stype mismatch: cannot write <unset> (instance) to a feature typed by %%s", where ? where : "", where ? ": " : "", type);
		sysml_fail(msg);
	}
	return o;
}
SYSML_OPT(r, sysml_real)
SYSML_OPT(b, sysml_bool)
SYSML_OPT(n, sysml_num)
SYSML_OPT(e, sysml_enum)
SYSML_OPT(rec, sysml_rec *)
typedef union {
	sysml_int i; sysml_real r; sysml_bool b; sysml_num n; sysml_enum e; int64_t run; sysml_rec *rec;
	sysml_opt_i u_i; sysml_opt_r u_r; sysml_opt_b u_b; sysml_opt_n u_n; sysml_opt_e u_e; sysml_opt_rec u_rec;
} sysml_cap;
/* A data value with features, identified by its address. It outlives every
   arena release; a run owns the records it makes until the next run begins. */
struct sysml_rec { const char *t; sysml_rec *next; sysml_cap f[1]; };
static sysml_rec *sysml_recs;
static sysml_rec *sysml_rec_new(const char *t, size_t n) {
	sysml_rec *r = calloc(1, sizeof(sysml_rec) + (n ? n - 1 : 0) * sizeof(sysml_cap));
	if (!r) sysml_fail("out of memory");
	r->t = t;
	r->next = sysml_recs;
	sysml_recs = r;
	return r;
}
static void sysml_recs_release(void) {
	while (sysml_recs) {
		sysml_rec *r = sysml_recs;
		sysml_recs = r->next;
		free(r);
	}
}
/* A function value: the function it is, an index into sysml_fn_names, for a
   closure the run of the body declaring it and what it captures, and for a
   record's calc the record it was read off. */
typedef struct { int32_t c; int64_t run; sysml_rec *self; sysml_cap env[%d]; } sysml_fn;
static const char *const sysml_fn_names[] = {%s};
static int64_t sysml_runs;
static inline int64_t sysml_next_run(void) { return ++sysml_runs; }
static inline bool sysml_fn_eq(sysml_fn a, sysml_fn b) { return a.c == b.c && a.run == b.run && a.self == b.self; }
static inline uint64_t sysml_fn_key(sysml_fn f) { return ((((uint64_t)f.run * 0x100000001B3ULL) ^ (uint64_t)(uintptr_t)f.self) * 0x100000001B3ULL) ^ (uint64_t)f.c; }
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
		"EQ", "sysml_fn_eq", "SHOW", "sysml_show_fn", "SAVEELEMS", "SYSML_NO_ELEMS", "RESTOREELEMS", "SYSML_NO_ELEMS",
		"KOPEN", `" ("`, "KCLOSE", `")"`)
	return "#define SYSML_KIND_FN(v) \"function\"\n" + r.Replace(cSeqTemplate)
}

// cRecSeqRuntime instantiates the collection runtime over records.
func cRecSeqRuntime(p *Program) string {
	if len(p.Records) == 0 {
		return ""
	}
	r := strings.NewReplacer("ELEMNAME", "rec", "ELEM", "sysml_rec *", "SFX", "rec", "PRINT", "sysml_print_rec_value",
		"FORMAT", "sysml_format_rec", "KINDOF", "SYSML_KIND_REC", "KEY", "sysml_rec_key", "SKIP", "SYSML_NEVER",
		"EQ", "SYSML_SCALAR_EQ", "SHOW", "sysml_show_rec", "SAVEELEMS", "SYSML_NO_ELEMS", "RESTOREELEMS", "SYSML_NO_ELEMS",
		"KOPEN", `""`, "KCLOSE", `""`)
	return `#define SYSML_KIND_REC(v) ""
static inline uint64_t sysml_rec_key(sysml_rec *r) { return (uint64_t)(uintptr_t)r; }
static void sysml_format_rec(sysml_rec *r, char *out, size_t size) { snprintf(out, size, "%s object", r->t); }
static void sysml_print_rec_value(sysml_rec *r) { printf("%s object", r->t); }
static const char *sysml_show_rec(sysml_rec *r) {
	static char text[256];
	sysml_format_rec(r, text, sizeof text);
	return text;
}
` + r.Replace(cSeqTemplate)
}

// cCapField is the member of sysml_cap a captured binding of type t is held in.
func cCapField(t Type) string {
	if t.MayUnset() {
		return "u_" + cCapField(t.Concrete())
	}
	switch {
	case t.IsRec():
		return "rec"
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
		if x.Self != nil {
			return fmt.Sprintf("((sysml_fn){.c = %d, .self = %s})", x.Case.ID, e.expr(x.Self)), true
		}
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
	case FnSelf:
		return cLocal(x.Name) + ".self", true
	case RecNew:
		e.temps++
		r := fmt.Sprintf("sysml_t%d", e.temps)
		var b strings.Builder
		fmt.Fprintf(&b, "({ sysml_rec *%s = sysml_rec_new(%s, %d); ", r, cString(x.Rec.Short), len(x.Fields))
		for i, v := range x.Fields {
			fmt.Fprintf(&b, "%s->f[%d].%s = %s; ", r, i, cCapField(v.Type()), e.expr(v))
		}
		fmt.Fprintf(&b, "%s; })", r)
		return b.String(), true
	case RecGet:
		if x.T.MayUnset() {
			return fmt.Sprintf("sysml_read_%s(&(%s)->f[%d].%s)", cCapField(x.T.Concrete()), e.expr(x.X), x.Field, cCapField(x.T)), true
		}
		return fmt.Sprintf("(%s)->f[%d].%s", e.expr(x.X), x.Field, cCapField(x.T)), true
	case Narrowed:
		if x.X.Type().MayUnset() {
			return fmt.Sprintf("sysml_narrow_opt(%s, %d, \"%s\", %s, true)", e.expr(x.X), x.R.Lower(), x.R, cWhere(x.Where)), true
		}
		return fmt.Sprintf("sysml_at_least_at(%s, %d, \"%s\", %s)", e.expr(x.X), x.R.Lower(), x.R, cWhere(x.Where)), true
	case NewUnset:
		return fmt.Sprintf("((%s){.u = sysml_new_unset(%d)})", cType(x.T), unsetTag(x.R)), true
	case Lift:
		return fmt.Sprintf("((%s){.v = %s})", cType(x.T), e.expr(x.X)), true
	case Need:
		return fmt.Sprintf("sysml_need_%s(%s, %s)", cCapField(x.Type()), e.expr(x.X), cString(x.Fail)), true
	case Strip:
		return fmt.Sprintf("(%s).v", e.expr(x.X)), true
	case Relabel:
		return fmt.Sprintf("((%s){.v = %s, .u = (%s).u})", cType(x.T), e.expr(x.V), e.expr(x.Of)), true
	case IsUnset:
		return fmt.Sprintf("((%s).u != 0)", e.expr(x.X)), true
	case SameUnset:
		return fmt.Sprintf("(((%s).u == (%s).u) != %t)", e.expr(x.L), e.expr(x.R), x.Neq), true
	case Named:
		return e.expr(x.X), true
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
