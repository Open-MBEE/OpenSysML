package codegen

import (
	"fmt"
	"strings"
)

// cSeqPrelude is the collection runtime of a generated C program: an arena the
// run owns, the interpreter's element budget, and the shape-carrying sequence.
const cSeqPrelude = `
/* A collection value is null, one bare scalar, or a sequence (shape). */
enum { SYSML_NULL = 0, SYSML_ONE = 1, SYSML_MANY = 2 };

typedef struct sysml_block { struct sysml_block *next; size_t used, cap; char data[]; } sysml_block;
static sysml_block *sysml_arena;
static sysml_int sysml_elements;
static sysml_int sysml_max_elements = SYSML_DEFAULT_MAX_ELEMENTS;

typedef struct { sysml_block *block; size_t used; } sysml_mark;

static sysml_mark sysml_arena_mark(void) {
	return (sysml_mark){sysml_arena, sysml_arena ? sysml_arena->used : 0};
}

static void sysml_arena_release(sysml_mark m) {
	while (sysml_arena != m.block) {
		sysml_block *next = sysml_arena->next;
		free(sysml_arena);
		sysml_arena = next;
	}
	if (sysml_arena) sysml_arena->used = m.used;
}

/* Whether p lies in memory a release to m would reclaim. */
static bool sysml_above_mark(const void *p, sysml_mark m) {
	const char *c = p;
	for (sysml_block *b = sysml_arena; b; b = b->next) {
		if (c >= b->data && c < b->data + b->used) return b != m.block || (size_t)(c - b->data) >= m.used;
		if (b == m.block) break;
	}
	return false;
}

/* A run owns what it allocates until the next run begins, so its result outlives it. */
static void sysml_run_begin(void) {
	static sysml_mark mark;
	static bool ran;
	if (ran) sysml_arena_release(mark);
	mark = sysml_arena_mark();
	ran = true;
	sysml_elements = 0;
}

static void *sysml_alloc(size_t n) {
	n = (n + 15) & ~(size_t)15;
	if (!sysml_arena || sysml_arena->cap - sysml_arena->used < n) {
		size_t cap = n > (1 << 16) ? n : (1 << 16);
		sysml_block *b = malloc(sizeof *b + cap);
		if (!b) sysml_fail("out of memory");
		b->next = sysml_arena; b->used = 0; b->cap = cap;
		sysml_arena = b;
	}
	void *p = sysml_arena->data + sysml_arena->used;
	sysml_arena->used += n;
	return p;
}

static void sysml_failf(const char *fmt, ...) __attribute__((format(printf, 1, 2), noreturn));
static void sysml_failf(const char *fmt, ...) {
	static char msg[512];
	va_list ap;
	va_start(ap, fmt);
	vsnprintf(msg, sizeof msg, fmt, ap);
	va_end(ap);
	sysml_fail(msg);
}

/* Charges n materialized elements to the budget a statement releases at its end. */
static inline void sysml_charge(sysml_int n) {
	if (__builtin_expect(__builtin_add_overflow(sysml_elements, n, &sysml_elements) || sysml_elements > sysml_max_elements || sysml_elements < 0, 0))
		sysml_failf("collection element limit exceeded (%lld elements; raise OPENSYSML_MAX_ELEMENTS to allow more)", (long long)sysml_max_elements);
}

static void sysml_mult_fail(const char *where, sysml_int n, sysml_int lo, sysml_int hi) {
	if (n < lo) sysml_failf("%s: multiplicity violation: %lld value(s) bound to a feature with multiplicity lower bound %lld", where, (long long)n, (long long)lo);
	sysml_failf("%s: multiplicity violation: %lld value(s) bound to a feature with multiplicity upper bound %lld", where, (long long)n, (long long)hi);
}

/* The interpreter's description of a collection by shape: "null" or "a sequence",
   "sequence" when bare; one holding one element is described as one. */
static const char *sysml_describe(int8_t shape, bool bare, const char *one) {
	if (shape == SYSML_NULL) return "null";
	if (shape == SYSML_ONE) return one;
	return bare ? "sequence" : "a sequence";
}

/* Trims the blanks around a token in place. */
static char *sysml_trim(char *tok) {
	while (*tok == ' ') tok++;
	char *end = tok + strlen(tok);
	while (end > tok && end[-1] == ' ') *--end = 0;
	return tok;
}

static void sysml_seq_open(void) { fputc('[', stdout); }
static void sysml_seq_close(void) { fputc(']', stdout); fputc('\n', stdout); }
static void sysml_seq_sep(sysml_int i) { if (i) fputs(", ", stdout); }
`

// cSeqTemplate is the runtime over one element type; ELEM is the C type, SFX
// the suffix naming it, PRINT prints one element without a newline.
const cSeqTemplate = `
typedef struct { int8_t shape; sysml_int len; ELEM *data; } sysml_seq_SFX;

static inline sysml_seq_SFX sysml_null_SFX(void) { return (sysml_seq_SFX){SYSML_NULL, 0, NULL}; }

static inline sysml_seq_SFX sysml_one_SFX(ELEM v) {
	ELEM *p = sysml_alloc(sizeof *p);
	*p = v;
	return (sysml_seq_SFX){SYSML_ONE, 1, p};
}

/* An uninitialized sequence of n elements, charged to the budget. */
static sysml_seq_SFX sysml_many_SFX(sysml_int n) {
	sysml_charge(n);
	return (sysml_seq_SFX){SYSML_MANY, n, n ? sysml_alloc((size_t)n * sizeof(ELEM)) : NULL};
}

/* Copies s out of the arena when a release to m would reclaim it; NULL when it would not. */
static ELEM *sysml_save_SFX(sysml_seq_SFX s, sysml_mark m) {
	if (!s.len || !sysml_above_mark(s.data, m)) return NULL;
	ELEM *t = malloc((size_t)s.len * sizeof(ELEM));
	if (!t) sysml_fail("out of memory");
	memcpy(t, s.data, (size_t)s.len * sizeof(ELEM));
	return t;
}

/* Moves a saved copy back into the arena after the release. */
static void sysml_restore_SFX(sysml_seq_SFX *s, ELEM *t) {
	if (!t) return;
	s->data = sysml_alloc((size_t)s->len * sizeof(ELEM));
	memcpy(s->data, t, (size_t)s->len * sizeof(ELEM));
	free(t);
}

static sysml_seq_SFX sysml_concat_SFX(int n, const sysml_seq_SFX *parts) {
	sysml_int total = 0;
	for (int i = 0; i < n; i++) total += parts[i].len;
	sysml_seq_SFX r = sysml_many_SFX(total);
	sysml_int k = 0;
	for (int i = 0; i < n; i++) {
		if (parts[i].len) memcpy(r.data + k, parts[i].data, (size_t)parts[i].len * sizeof(ELEM));
		k += parts[i].len;
	}
	return r;
}

/* The one value bound to a [1] feature at where. */
static inline ELEM sysml_one_of_SFX(sysml_seq_SFX s, const char *where) {
	if (__builtin_expect(s.len != 1, 0)) sysml_mult_fail(where, s.len, 1, 1);
	return s.data[0];
}

/* The one scalar an operator needs; fmt takes the shape found, and the
   description of the other operand where the message names both. */
static inline ELEM sysml_scalar_SFX(sysml_seq_SFX s, const char *fmt, bool bare, const char *other) {
	if (__builtin_expect(s.shape != SYSML_ONE, 0)) sysml_failf(fmt, sysml_describe(s.shape, bare, ""), other);
	return s.data[0];
}

static inline sysml_seq_SFX sysml_check_SFX(sysml_seq_SFX s, sysml_int lo, sysml_int hi, const char *where) {
	if (__builtin_expect(s.len < lo || (hi >= 0 && s.len > hi), 0)) sysml_mult_fail(where, s.len, lo, hi);
	return s;
}

/* Same shape, same elements in order: the '==' of collections; every empty one is null. */
static sysml_bool sysml_eq_SFX(sysml_seq_SFX a, sysml_seq_SFX b) {
	if (a.len == 0 || b.len == 0) return a.len == 0 && b.len == 0;
	if (a.shape != b.shape || a.len != b.len) return false;
	for (sysml_int i = 0; i < a.len; i++) if (a.data[i] != b.data[i]) return false;
	return true;
}

/* Same elements in order whatever the shape: SequenceFunctions::equals and same. */
static sysml_bool sysml_equals_SFX(sysml_seq_SFX a, sysml_seq_SFX b) {
	if (a.len != b.len) return false;
	for (sysml_int i = 0; i < a.len; i++) if (a.data[i] != b.data[i]) return false;
	return true;
}

static inline ELEM sysml_index_SFX(sysml_seq_SFX s, sysml_int i) {
	if (__builtin_expect(i < 1 || i > s.len, 0)) sysml_failf("index out of range: sequence index %lld is outside 1..%lld", (long long)i, (long long)s.len);
	return s.data[i - 1];
}

static sysml_bool sysml_contains_SFX(sysml_seq_SFX s, ELEM v) {
	for (sysml_int i = 0; i < s.len; i++) if (s.data[i] == v) return true;
	return false;
}

static sysml_bool sysml_includes_SFX(sysml_seq_SFX a, sysml_seq_SFX b) {
	for (sysml_int i = 0; i < b.len; i++) if (!sysml_contains_SFX(a, b.data[i])) return false;
	return true;
}

static sysml_bool sysml_includes_only_SFX(sysml_seq_SFX a, sysml_seq_SFX b) {
	return sysml_includes_SFX(a, b) && sysml_includes_SFX(b, a);
}

static sysml_bool sysml_excludes_SFX(sysml_seq_SFX a, sysml_seq_SFX b) {
	for (sysml_int i = 0; i < b.len; i++) if (sysml_contains_SFX(a, b.data[i])) return false;
	return true;
}

static sysml_seq_SFX sysml_union_SFX(sysml_seq_SFX a, sysml_seq_SFX b) {
	sysml_seq_SFX parts[2] = {a, b};
	return sysml_concat_SFX(2, parts);
}

/* The elements of a that b holds (keep) or does not (!keep), in a's order. */
static sysml_seq_SFX sysml_sift_SFX(sysml_seq_SFX a, sysml_seq_SFX b, sysml_bool keep) {
	sysml_int n = 0;
	for (sysml_int i = 0; i < a.len; i++) if (sysml_contains_SFX(b, a.data[i]) == keep) n++;
	sysml_seq_SFX r = sysml_many_SFX(n);
	sysml_int k = 0;
	for (sysml_int i = 0; i < a.len; i++) if (sysml_contains_SFX(b, a.data[i]) == keep) r.data[k++] = a.data[i];
	return r;
}

static sysml_seq_SFX sysml_including_at_SFX(sysml_seq_SFX a, sysml_seq_SFX b, sysml_int i) {
	if (i < 1 || i > a.len + 1) sysml_failf("index out of range: SequenceFunctions::includingAt insertion index %lld is outside 1..%lld", (long long)i, (long long)(a.len + 1));
	sysml_seq_SFX parts[3] = {{SYSML_MANY, i - 1, a.data}, b, {SYSML_MANY, a.len - (i - 1), a.data + (i - 1)}};
	return sysml_concat_SFX(3, parts);
}

static sysml_seq_SFX sysml_subsequence_SFX(sysml_seq_SFX s, sysml_int start, sysml_int end, sysml_bool has_end) {
	if (!has_end) end = s.len;
	if (start < 1) sysml_failf("index out of range: SequenceFunctions::subsequence start index %lld is outside 1..%lld", (long long)start, (long long)s.len);
	if (start > end) return sysml_many_SFX(0);
	if (end > s.len) sysml_failf("index out of range: SequenceFunctions::subsequence end index %lld is outside 1..%lld", (long long)end, (long long)s.len);
	sysml_seq_SFX part = {SYSML_MANY, end - start + 1, s.data + (start - 1)};
	return sysml_concat_SFX(1, &part);
}

static sysml_seq_SFX sysml_excluding_at_SFX(sysml_seq_SFX s, sysml_int start, sysml_int end, sysml_bool has_end) {
	if (!has_end) end = start;
	if (start < 1 || start > s.len) sysml_failf("index out of range: SequenceFunctions::excludingAt start index %lld is outside 1..%lld", (long long)start, (long long)s.len);
	if (end < start || end > s.len) sysml_failf("index out of range: SequenceFunctions::excludingAt end index %lld is outside %lld..%lld", (long long)end, (long long)start, (long long)s.len);
	sysml_seq_SFX parts[2] = {{SYSML_MANY, start - 1, s.data}, {SYSML_MANY, s.len - end, s.data + end}};
	return sysml_concat_SFX(2, parts);
}

static inline sysml_seq_SFX sysml_head_SFX(sysml_seq_SFX s) {
	return s.len ? (sysml_seq_SFX){SYSML_ONE, 1, s.data} : sysml_null_SFX();
}

static inline sysml_seq_SFX sysml_last_SFX(sysml_seq_SFX s) {
	return s.len ? (sysml_seq_SFX){SYSML_ONE, 1, s.data + s.len - 1} : sysml_null_SFX();
}

static sysml_seq_SFX sysml_tail_SFX(sysml_seq_SFX s) {
	if (s.len == 0) return sysml_many_SFX(0);
	sysml_seq_SFX part = {SYSML_MANY, s.len - 1, s.data + 1};
	return sysml_concat_SFX(1, &part);
}

/* A sequence collected element by element, charged as it grows. */
static void sysml_push_SFX(sysml_seq_SFX *r, sysml_int *cap, ELEM v) {
	sysml_charge(1);
	if (r->len == *cap) {
		*cap = *cap ? *cap * 2 : 8;
		ELEM *data = sysml_alloc((size_t)*cap * sizeof(ELEM));
		if (r->len) memcpy(data, r->data, (size_t)r->len * sizeof(ELEM));
		r->data = data;
	}
	r->data[r->len++] = v;
}

static void sysml_append_SFX(sysml_seq_SFX *r, sysml_int *cap, sysml_seq_SFX s) {
	for (sysml_int i = 0; i < s.len; i++) sysml_push_SFX(r, cap, s.data[i]);
}

static void sysml_print_seq_SFX(sysml_seq_SFX s) {
	if (s.shape == SYSML_NULL) { puts("null"); return; }
	if (s.shape == SYSML_ONE) { PRINT(s.data[0]); fputc('\n', stdout); return; }
	sysml_seq_open();
	for (sysml_int i = 0; i < s.len; i++) { sysml_seq_sep(i); PRINT(s.data[i]); }
	sysml_seq_close();
}

/* Parses a collection argument in the notation the interpreter reads and
   prints: null, a bare value, (a, b, ...) with (a) a bare value, [a, b, ...]. */
static sysml_seq_SFX sysml_parse_seq_SFX(const char *s, const char *name) {
	if (strcmp(s, "null") == 0) return sysml_null_SFX();
	if (s[0] != '(' && s[0] != '[') return sysml_one_SFX(sysml_parse_ELEMNAME(s, name));
	size_t n = strlen(s);
	if (n < 2 || s[n - 1] != (s[0] == '(' ? ')' : ']')) {
		fprintf(stderr, "argument %s: %s is not a sequence (a, b, ...)\n", name, s);
		exit(2);
	}
	char *body = strndup(s + 1, n - 2);
	if (!body) sysml_fail("out of memory");
	sysml_int count = 1;
	for (size_t i = 0; body[i]; i++) if (body[i] == ',') count++;
	if (s[0] == '(' && count == 1 && n > 2) {
		sysml_seq_SFX r = sysml_one_SFX(sysml_parse_ELEMNAME(sysml_trim(body), name));
		free(body);
		return r;
	}
	sysml_seq_SFX r = {SYSML_MANY, 0, n > 2 ? sysml_alloc((size_t)count * sizeof(ELEM)) : NULL};
	if (n > 2) {
		char *tok = body;
		for (char *comma; (comma = strchr(tok, ',')); tok = comma + 1) {
			*comma = 0;
			r.data[r.len++] = sysml_parse_ELEMNAME(sysml_trim(tok), name);
		}
		r.data[r.len++] = sysml_parse_ELEMNAME(sysml_trim(tok), name);
	}
	free(body);
	return r;
}
`

// cSeqTyped is the runtime that differs by element type: ranges and
// aggregation over numbers, truth over Booleans, widening to Real.
const cSeqTyped = `
static sysml_seq_int sysml_nonnegative_seq(sysml_seq_int s, const char *type) {
	for (sysml_int i = 0; i < s.len; i++) sysml_nonnegative(s.data[i], type);
	return s;
}

static sysml_seq_int sysml_range(sysml_int lo, sysml_int hi) {
	if (lo > hi) return sysml_many_int(0);
	sysml_int n;
	if (__builtin_sub_overflow(hi, lo, &n) || __builtin_add_overflow(n, 1, &n)) n = INT64_MAX;
	sysml_seq_int r = sysml_many_int(n);
	for (sysml_int i = 0; i < n; i++) r.data[i] = lo + i;
	return r;
}

/* The Real copy of an Integer collection, charged like any other materialized collection. */
static sysml_seq_real sysml_widen(sysml_seq_int s) {
	sysml_charge(s.len);
	sysml_seq_real r = {s.shape, s.len, s.len ? sysml_alloc((size_t)s.len * sizeof(sysml_real)) : NULL};
	for (sysml_int i = 0; i < s.len; i++) r.data[i] = (sysml_real)s.data[i];
	return r;
}

static sysml_int sysml_isum(sysml_seq_int s, const char *op) {
	sysml_int acc = 0;
	for (sysml_int i = 0; i < s.len; i++)
		if (__builtin_add_overflow(acc, s.data[i], &acc)) sysml_failf("arithmetic overflow: %s exceeds the Integer range", op);
	return acc;
}

static sysml_int sysml_iproduct(sysml_seq_int s, const char *op) {
	sysml_int acc = 1;
	for (sysml_int i = 0; i < s.len; i++)
		if (__builtin_mul_overflow(acc, s.data[i], &acc)) sysml_failf("arithmetic overflow: %s exceeds the Integer range", op);
	return acc;
}

static sysml_real sysml_rsum(sysml_seq_real s, const char *op) {
	sysml_real acc = 0;
	for (sysml_int i = 0; i < s.len; i++) {
		acc += s.data[i];
		if (isinf(acc)) sysml_failf("arithmetic overflow: %s is not a finite Real", op);
	}
	return acc;
}

static sysml_real sysml_rproduct(sysml_seq_real s, const char *op) {
	sysml_real acc = 1;
	for (sysml_int i = 0; i < s.len; i++) {
		acc *= s.data[i];
		if (isinf(acc)) sysml_failf("arithmetic overflow: %s is not a finite Real", op);
	}
	return acc;
}

static sysml_bool sysml_all_true(sysml_seq_bool s) {
	for (sysml_int i = 0; i < s.len; i++) if (!s.data[i]) return false;
	return true;
}

static sysml_bool sysml_any_true(sysml_seq_bool s) {
	for (sysml_int i = 0; i < s.len; i++) if (s.data[i]) return true;
	return false;
}

static void sysml_print_int(sysml_int v) { printf("%" PRId64, v); }
static void sysml_print_bool(sysml_bool v) { fputs(v ? "true" : "false", stdout); }

static void sysml_read_max_elements(void) {
	const char *raw = getenv("OPENSYSML_MAX_ELEMENTS");
	if (!raw) return;
	const char *s = raw;
	while (*s == ' ' || *s == '\t' || *s == '\n') s++;
	if (!*s) return;
	char *end;
	errno = 0;
	long long n = strtoll(s, &end, 10);
	while (*end == ' ' || *end == '\t' || *end == '\n') end++;
	if (*end || errno) {
		fprintf(stderr, "OPENSYSML_MAX_ELEMENTS=\"%s\" is not an integer: set it to a positive number of collection elements (default %lld)\n", raw, (long long)SYSML_DEFAULT_MAX_ELEMENTS);
		exit(2);
	}
	if (n <= 0) {
		fprintf(stderr, "OPENSYSML_MAX_ELEMENTS=\"%s\" must be greater than zero: the budget is what stops a runaway run (default %lld)\n", raw, (long long)SYSML_DEFAULT_MAX_ELEMENTS);
		exit(2);
	}
	sysml_max_elements = n;
}
`

// cSeqSuffix names the element type of a collection in the C runtime.
func cSeqSuffix(t Type) string {
	switch t.Elem() {
	case TypeInt:
		return "int"
	case TypeReal:
		return "real"
	}
	return "bool"
}

// cSeqRuntime is the collection runtime instantiated for every element type.
func cSeqRuntime() string {
	var b strings.Builder
	b.WriteString(cSeqPrelude)
	// Element printers precede the template; the Real printer is the scalar one.
	b.WriteString("\nstatic void sysml_print_int(sysml_int v);\nstatic void sysml_print_bool(sysml_bool v);\nstatic void sysml_print_real_value(sysml_real r);\n")
	for _, t := range []Type{TypeInt, TypeReal, TypeBool} {
		print := "sysml_print_" + cSeqSuffix(t)
		if t == TypeReal {
			print = "sysml_print_real_value"
		}
		r := strings.NewReplacer("ELEMNAME", cSeqSuffix(t), "ELEM", cType(t), "SFX", cSeqSuffix(t), "PRINT", print)
		b.WriteString(r.Replace(cSeqTemplate))
	}
	b.WriteString(cSeqTyped)
	return b.String()
}

// cWhere is a C string literal for the binding a diagnostic names.
func cWhere(where string) string {
	return fmt.Sprintf("%q", where)
}

// seqExpr emits a collection expression; ok is false for a scalar node.
func (e *cEmitter) seqExpr(x Expr) (string, bool) {
	switch x := x.(type) {
	case NullLit:
		return fmt.Sprintf("sysml_null_%s()", cSeqSuffix(x.T)), true
	case SeqLit:
		return e.seqLit(x), true
	case ToMany:
		return fmt.Sprintf("sysml_one_%s(%s)", cSeqSuffix(x.X.Type()), e.expr(x.X)), true
	case ToOne:
		sfx := cSeqSuffix(x.X.Type())
		if x.Where != "" {
			return fmt.Sprintf("sysml_one_of_%s(%s, %s)", sfx, e.expr(x.X), cWhere(x.Where)), true
		}
		other := `""`
		if x.Other != nil {
			other = fmt.Sprintf("sysml_describe(%s.shape, %t, %q)", e.expr(x.Other), x.Bare, x.OtherOne)
		}
		return fmt.Sprintf("sysml_scalar_%s(%s, %s, %t, %s)", sfx, e.expr(x.X), cWhere(x.Fail), x.Bare, other), true
	case Let:
		return fmt.Sprintf("({ %s %s = %s; (void)%s; %s; })", cType(x.Value.Type()), cLocal(x.Name), e.expr(x.Value), cLocal(x.Name), e.expr(x.In)), true
	case Checked:
		return e.checked(x), true
	case Coalesce:
		e.temps++
		l := fmt.Sprintf("sysml_t%d", e.temps)
		return fmt.Sprintf("({ %s %s = %s; %s.len ? %s : %s; })", cType(x.T), l, e.expr(x.L), l, l, e.expr(x.R)), true
	case SeqEq:
		return e.sequenced([]Expr{x.L, x.R}, func(v []string) string {
			eq := fmt.Sprintf("sysml_eq_%s(%s, %s)", cSeqSuffix(x.L.Type()), v[0], v[1])
			if x.Neq {
				return "(!" + eq + ")"
			}
			return eq
		}), true
	case Index:
		return e.sequenced([]Expr{x.Seq, x.I}, func(v []string) string {
			return fmt.Sprintf("sysml_index_%s(%s, %s)", cSeqSuffix(x.Seq.Type()), v[0], v[1])
		}), true
	case RangeExpr:
		return e.sequenced([]Expr{x.Lo, x.Hi}, func(v []string) string {
			return fmt.Sprintf("sysml_range(%s, %s)", v[0], v[1])
		}), true
	case SeqCall:
		return e.sequenced(x.Args, func(v []string) string { return e.seqCall(x, v) }), true
	case Fold:
		return e.fold(x), true
	}
	return "", false
}

// seqLit concatenates the operands' elements, evaluated left to right.
func (e *cEmitter) seqLit(x SeqLit) string {
	sfx := cSeqSuffix(x.T)
	if len(x.Elems) == 0 {
		return fmt.Sprintf("sysml_many_%s(0)", sfx)
	}
	parts := make([]Expr, len(x.Elems))
	for i, el := range x.Elems {
		if el.Type().Scalar() {
			parts[i] = ToMany{X: el}
		} else {
			parts[i] = el
		}
	}
	return e.sequenced(parts, func(v []string) string {
		return fmt.Sprintf("sysml_concat_%s(%d, (sysml_seq_%s[]){%s})", sfx, len(v), sfx, strings.Join(v, ", "))
	})
}

// checked binds a collection: multiplicity first, then the elements' range.
func (e *cEmitter) checked(x Checked) string {
	sfx := cSeqSuffix(x.X.Type())
	v := e.expr(x.X)
	if x.M != MultAny {
		v = fmt.Sprintf("sysml_check_%s(%s, %d, %d, %s)", sfx, v, x.M.Lower, x.M.Upper, cWhere(x.Where))
	}
	if x.R != RangeAny {
		v = fmt.Sprintf("sysml_nonnegative_seq(%s, \"%s\")", v, x.R)
	}
	return v
}

// seqCall applies a value operation to its evaluated operands.
func (e *cEmitter) seqCall(x SeqCall, v []string) string {
	sfx := cSeqSuffix(x.Args[0].Type())
	switch x.Op {
	case SeqSize:
		return v[0] + ".len"
	case SeqIsEmpty:
		return "(" + v[0] + ".len == 0)"
	case SeqNotEmpty:
		return "(" + v[0] + ".len != 0)"
	case SeqIncludes:
		return fmt.Sprintf("sysml_includes_%s(%s, %s)", sfx, v[0], v[1])
	case SeqIncludesOnly:
		return fmt.Sprintf("sysml_includes_only_%s(%s, %s)", sfx, v[0], v[1])
	case SeqExcludes:
		return fmt.Sprintf("sysml_excludes_%s(%s, %s)", sfx, v[0], v[1])
	case SeqEquals, SeqSame:
		return fmt.Sprintf("sysml_equals_%s(%s, %s)", sfx, v[0], v[1])
	case SeqUnion, SeqIncluding:
		return fmt.Sprintf("sysml_union_%s(%s, %s)", sfx, v[0], v[1])
	case SeqIntersection:
		return fmt.Sprintf("sysml_sift_%s(%s, %s, true)", sfx, v[0], v[1])
	case SeqExcluding:
		return fmt.Sprintf("sysml_sift_%s(%s, %s, false)", sfx, v[0], v[1])
	case SeqIncludingAt:
		return fmt.Sprintf("sysml_including_at_%s(%s, %s, %s)", sfx, v[0], v[1], v[2])
	case SeqSubsequence, SeqExcludingAt:
		name := "subsequence"
		if x.Op == SeqExcludingAt {
			name = "excluding_at"
		}
		end, has := "0", "false"
		if len(v) == 3 {
			end, has = v[2], "true"
		}
		return fmt.Sprintf("sysml_%s_%s(%s, %s, %s, %s)", name, sfx, v[0], v[1], end, has)
	case SeqHead:
		return fmt.Sprintf("sysml_head_%s(%s)", sfx, v[0])
	case SeqTail:
		return fmt.Sprintf("sysml_tail_%s(%s)", sfx, v[0])
	case SeqLast:
		return fmt.Sprintf("sysml_last_%s(%s)", sfx, v[0])
	case SeqAllTrue:
		return fmt.Sprintf("sysml_all_true(%s)", v[0])
	case SeqAnyTrue:
		return fmt.Sprintf("sysml_any_true(%s)", v[0])
	case SeqSum, SeqProduct:
		fn := map[SeqOp]string{SeqSum: "sum", SeqProduct: "product"}[x.Op]
		prefix := "i"
		if x.T == TypeReal {
			prefix = "r"
		}
		return fmt.Sprintf("sysml_%s%s(%s, %q)", prefix, fn, v[0], x.Op.Name())
	}
	e.err = fmt.Errorf("codegen: C emitter has no case for collection operation %s", x.Op)
	return "0"
}

// fold emits a body operation as a loop in a statement expression; the
// body's parameters and locals are block-scoped C variables.
func (e *cEmitter) fold(x Fold) string {
	e.temps++
	n := e.temps
	seq := fmt.Sprintf("sysml_s%d", n)
	elem := x.Seq.Type().Elem()
	sfx := cSeqSuffix(elem)
	var b strings.Builder
	fmt.Fprintf(&b, "({ %s %s = %s; ", cType(x.Seq.Type()), seq, e.expr(x.Seq))
	// bind opens the loop body with the parameters bound to args.
	bind := func(args ...string) string {
		var s strings.Builder
		for i, p := range x.Body.Params {
			fmt.Fprintf(&s, "%s %s = %s; ", cType(p.Type), cLocal(p.Name), args[i])
		}
		return s.String()
	}
	at := seq + ".data[sysml_i]"
	body := e.expr(x.Body.Body)
	loop := fmt.Sprintf("for (sysml_int sysml_i = 0; sysml_i < %s.len; sysml_i++) { ", seq)
	switch x.Op {
	case SeqSelect, SeqReject:
		keep := "true"
		if x.Op == SeqReject {
			keep = "false"
		}
		fmt.Fprintf(&b, "sysml_seq_%s sysml_r%d = {SYSML_MANY, 0, %s.len ? sysml_alloc((size_t)%s.len * sizeof(%s)) : NULL}; ", sfx, n, seq, seq, cType(elem))
		fmt.Fprintf(&b, "%s%sif (%s == %s) sysml_r%d.data[sysml_r%d.len++] = %s; } ", loop, bind(at), body, keep, n, n, at)
		fmt.Fprintf(&b, "sysml_charge(sysml_r%d.len); sysml_r%d; })", n, n)
	case SeqSelectOne:
		fmt.Fprintf(&b, "sysml_seq_%s sysml_r%d = sysml_null_%s(); ", sfx, n, sfx)
		fmt.Fprintf(&b, "%s%sif (%s) { sysml_r%d = (sysml_seq_%s){SYSML_ONE, 1, %s.data + sysml_i}; break; } } ", loop, bind(at), body, n, sfx, seq)
		fmt.Fprintf(&b, "sysml_r%d; })", n)
	case SeqCollect:
		rsfx := cSeqSuffix(x.T)
		fmt.Fprintf(&b, "sysml_seq_%s sysml_r%d = {SYSML_MANY, 0, NULL}; sysml_int sysml_c%d = 0; ", rsfx, n, n)
		add := fmt.Sprintf("sysml_push_%s(&sysml_r%d, &sysml_c%d, %s)", rsfx, n, n, body)
		if x.Body.Body.Type().Many() {
			add = fmt.Sprintf("sysml_append_%s(&sysml_r%d, &sysml_c%d, %s)", rsfx, n, n, body)
		}
		fmt.Fprintf(&b, "%s%s%s; } sysml_r%d; })", loop, bind(at), add, n)
	case SeqForAll, SeqExists:
		universal := x.Op == SeqForAll
		fmt.Fprintf(&b, "sysml_bool sysml_r%d = %t; ", n, universal)
		fmt.Fprintf(&b, "%s%sif (%s != %t) { sysml_r%d = %t; break; } } ", loop, bind(at), body, universal, n, !universal)
		fmt.Fprintf(&b, "sysml_r%d; })", n)
	case SeqReduce:
		fmt.Fprintf(&b, "sysml_seq_%s sysml_r%d = sysml_null_%s(); if (%s.len) { %s sysml_a%d = %s.data[0]; ", sfx, n, sfx, seq, cType(elem), n, seq)
		fmt.Fprintf(&b, "for (sysml_int sysml_i = 1; sysml_i < %s.len; sysml_i++) { %ssysml_a%d = %s; } ", seq, bind(fmt.Sprintf("sysml_a%d", n), at), n, body)
		fmt.Fprintf(&b, "sysml_r%d = sysml_one_%s(sysml_a%d); } sysml_r%d; })", n, sfx, n, n)
	case SeqMinimize, SeqMaximize:
		less := "<"
		if x.Op == SeqMaximize {
			less = ">"
		}
		fmt.Fprintf(&b, "if (!%s.len) sysml_fail(\"multiplicity violation: %s requires a collection of at least one element\"); ", seq, x.Op.Name())
		fmt.Fprintf(&b, "%s sysml_r%d = 0; ", cType(x.T), n)
		fmt.Fprintf(&b, "%s%s%s sysml_v%d = %s; if (sysml_i == 0 || sysml_v%d %s sysml_r%d) sysml_r%d = sysml_v%d; } ", loop, bind(at), cType(x.T), n, body, n, less, n, n, n)
		fmt.Fprintf(&b, "sysml_r%d; })", n)
	default:
		e.err = fmt.Errorf("codegen: C emitter has no case for body operation %s", x.Op)
	}
	return b.String()
}

// forEach iterates a collection; a bare scalar is not iterable.
func (e *cEmitter) forEach(s ForEach) {
	e.temps++
	seq := fmt.Sprintf("sysml_s%d", e.temps)
	elem := s.Seq.Type().Elem()
	e.linef("{ %s %s = %s;", cType(s.Seq.Type()), seq, e.expr(s.Seq))
	e.indent++
	e.linef("if (%s.shape == SYSML_ONE) sysml_fail(\"type mismatch: 'for' iterates a collection, and %s is not one\");", seq, article(elem))
	mark := e.arenaMark()
	e.linef("for (sysml_int sysml_i = 0; sysml_i < %s.len; sysml_i++) {", seq)
	e.indent++
	e.linef("%s %s = %s.data[sysml_i];", cType(elem), cLocal(s.Var), seq)
	e.block(s.Body)
	e.compact(escapingSeqs(s), mark)
	e.indent--
	e.linef("}")
	e.indent--
	e.linef("}")
}

// article names a scalar type as the interpreter's diagnostics do.
func article(t Type) string {
	if t == TypeInt {
		return "an Integer"
	}
	return "a " + t.String()
}
