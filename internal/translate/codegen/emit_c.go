package codegen

import (
	"fmt"
	"io"
	"math"
	"math/big"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

const cAssign = "%s = %s;"

// cBudgetDefines are the interpreter's budget defaults the C runtime reads.
func cBudgetDefines() string {
	return fmt.Sprintf("#define SYSML_MAX_CALC_DEPTH %d\n#define SYSML_DEFAULT_MAX_STEPS %d\n#define SYSML_DEFAULT_MAX_INTEGER_BITS %d\n#define SYSML_MIN_MAX_INTEGER_BITS %d\n",
		runtime.DefaultMaxCalcDepth, runtime.DefaultMaxSteps, runtime.DefaultMaxIntegerBits, runtime.MinMaxIntegerBits)
}

// cScalarRuntime is the runtime every generated C program carries: the budgets,
// the arena, the unbounded Integer, finite-only binary64 and their notation.
func cScalarRuntime() string {
	return cBudgetDefines() + cPrelude + cBigRuntime + cNumeric
}

// cPrelude is the base of cScalarRuntime: failure, the step budget and the arena.
const cPrelude = `#include <errno.h>
#include <inttypes.h>
#include <locale.h>
#include <stdarg.h>
#include <math.h>
#include <setjmp.h>
#include <stdbool.h>
#include <stddef.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>


/* A machine word: a length, a position, a count. */
typedef int64_t sysml_int;
/* An Integer: small while big is NULL, as the interpreter holds one. */
typedef struct sysml_big sysml_big;
typedef struct { int64_t small; const sysml_big *big; } sysml_integer;
#define SYSML_I(v) ((sysml_integer){(v), NULL})
typedef double sysml_real;
typedef bool sysml_bool;

static jmp_buf sysml_escape;
static const char *sysml_error_message;
static int sysml_depth;

static void sysml_fail(const char *msg) {
	sysml_error_message = msg;
	longjmp(sysml_escape, 1);
}

/* What a record or a closure keeps outlives every arena release: a run owns
   it until the next run begins. */
typedef struct sysml_keeper { struct sysml_keeper *next; max_align_t data[]; } sysml_keeper;
static sysml_keeper *sysml_keepers;

static void *sysml_keep(const void *p, size_t n) {
	sysml_keeper *k = malloc(sizeof *k + n);
	if (!k) sysml_fail("out of memory");
	k->next = sysml_keepers;
	sysml_keepers = k;
	return memcpy(k->data, p, n);
}

static void sysml_keep_release(void) {
	while (sysml_keepers) {
		sysml_keeper *k = sysml_keepers;
		sysml_keepers = k->next;
		free(k);
	}
}

typedef struct sysml_block { struct sysml_block *next; size_t used, cap; char data[]; } sysml_block;
static sysml_block *sysml_arena;

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

static bool sysml_in_arena(const void *p) { return sysml_above_mark(p, (sysml_mark){NULL, 0}); }

/* A run owns what it allocates until the next run begins, so its result outlives it. */
static void sysml_run_begin(void) {
	static sysml_mark mark;
	static bool ran;
	if (ran) sysml_arena_release(mark);
	mark = sysml_arena_mark();
	ran = true;
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
	static char *msg;
	va_list ap, again;
	va_start(ap, fmt);
	va_copy(again, ap);
	int n = vsnprintf(NULL, 0, fmt, ap);
	va_end(ap);
	free(msg);
	msg = malloc((size_t)n + 1);
	if (!msg) sysml_fail("out of memory");
	vsnprintf(msg, (size_t)n + 1, fmt, again);
	va_end(again);
	sysml_fail(msg);
}

static inline void sysml_enter(void) {
	if (__builtin_expect(sysml_depth >= SYSML_MAX_CALC_DEPTH, 0))
		sysml_fail("calc recursion limit exceeded");
	sysml_depth++;
}

static inline void sysml_leave(void) { sysml_depth--; }

static sysml_int sysml_steps;
static sysml_int sysml_max_steps = SYSML_DEFAULT_MAX_STEPS;

static void sysml_step_fail(void) __attribute__((noreturn));
static void sysml_step_fail(void) {
	static char msg[128];
	sysml_steps = sysml_max_steps < INT64_MAX ? sysml_max_steps + 1 : sysml_max_steps;
	snprintf(msg, sizeof msg, "evaluation step limit exceeded (%lld steps; raise OPENSYSML_MAX_STEPS to allow more)", (long long)sysml_max_steps);
	sysml_fail(msg);
	__builtin_unreachable();
}

/* Spends n evaluation steps of the run's budget, checking the room left
   before spending it so the counter never overflows. */
static inline void sysml_step(sysml_int n) {
	if (__builtin_expect(n > sysml_max_steps - sysml_steps, 0)) sysml_step_fail();
	sysml_steps += n;
}

/* The positive budget the variable env sets, def when it is unset or blank. */
static sysml_int sysml_read_budget(const char *env, const char *counts, sysml_int def) {
	const char *raw = getenv(env);
	if (!raw) return def;
	const char *s = raw;
	while (*s == ' ' || *s == '\t' || *s == '\n') s++;
	if (!*s) return def;
	char *end;
	errno = 0;
	long long n = strtoll(s, &end, 10);
	while (*end == ' ' || *end == '\t' || *end == '\n') end++;
	if (*end || errno) {
		fprintf(stderr, "%s=\"%s\" is not an integer: set it to a positive number of %s (default %lld)\n", env, raw, counts, (long long)def);
		exit(2);
	}
	if (n <= 0) {
		fprintf(stderr, "%s=\"%s\" must be greater than zero: the budget is what stops a runaway run (default %lld)\n", env, raw, (long long)def);
		exit(2);
	}
	return n;
}

`

// cNumeric is the scalar runtime over the Integer of cBigRuntime.
const cNumeric = `
static inline sysml_integer sysml_at_least_at(sysml_integer v, int64_t lo, const char *type, const char *where) {
	if (__builtin_expect(v.big ? v.big->neg : v.small < lo, 0)) {
		int64_t n;
		const char *digits = sysml_int_digits(v, &n);
		sysml_failf("%s: type mismatch: cannot write %.*s (an Integer) to a feature typed by %s", where, (int)n, digits, type);
	}
	return v;
}

static inline sysml_integer sysml_at_least(sysml_integer v, int64_t lo, const char *type) {
	if (__builtin_expect(v.big ? v.big->neg : v.small < lo, 0)) {
		int64_t n;
		const char *digits = sysml_int_digits(v, &n);
		sysml_failf("type mismatch: cannot write %.*s (an Integer) to a feature typed by %s", (int)n, digits, type);
	}
	return v;
}


/* An Integer in exact Rational arithmetic, computed in binary64 only over values binary64 holds. */
static inline sysml_real sysml_to_real_exact(sysml_integer a) {
	if (__builtin_expect(a.big || a.small > (1LL << 53) || a.small < -(1LL << 53), 0)) sysml_fail("unsupported: exact Rational arithmetic over an Integer beyond 2^53, which binary64 does not hold exactly");
	return (sysml_real)a.small;
}

/* r with an exact Rational's unsigned zero. */
static inline sysml_real sysml_unsigned_zero(sysml_real r) { return r == 0 ? 0.0 : r; }

/* The exact whole number r, which binary64 holds below 2^53. */
static inline sysml_real sysml_whole(sysml_real r) {
	if (__builtin_expect(r >= 9007199254740992.0 || r <= -9007199254740992.0, 0)) sysml_fail("unsupported: exact Rational arithmetic reaching 2^53, beyond which binary64 does not hold a whole number exactly");
	return sysml_unsigned_zero(r);
}

/* Orders the exact quotient a/b, b nonzero, against the Integer c. */
static int sysml_cmp_qi(sysml_integer a, sysml_integer b, sysml_integer c) {
	int c0 = sysml_icmp(a, sysml_mul_exact(c, b));
	return sysml_isign(b) < 0 ? -c0 : c0;
}

/* Orders the exact quotient a/b, b nonzero, against the Real r. */
static int sysml_cmp_qr(sysml_integer a, sysml_integer b, sysml_real r) {
	if (isnan(r)) return 0;
	if (isinf(r)) return r > 0 ? -1 : 1;
	int e;
	/* r is m * 2^e for the whole number m. */
	sysml_integer m = SYSML_I((int64_t)ldexp(frexp(r, &e), 53));
	e -= 53;
	sysml_integer rhs = sysml_mul_exact(m, b);
	if (e >= 0) rhs = sysml_shift(rhs, e);
	else a = sysml_shift(a, -e);
	int c0 = sysml_icmp(a, rhs);
	return sysml_isign(b) < 0 ? -c0 : c0;
}

static inline sysml_integer sysml_nonzero(sysml_integer b) {
	if (__builtin_expect(sysml_isign(b) == 0, 0)) sysml_fail("division by zero");
	return b;
}

/* A value of a Real-typed feature: an Integer unless real is set. */
typedef struct { bool real; sysml_integer i; sysml_real r; } sysml_num;
typedef struct { sysml_int len; const char *p; } sysml_str;
/* A collection as a record or a closure keeps it, whatever its elements. */
typedef struct { int8_t shape; sysml_int len; void *data; } sysml_kept;

static inline sysml_num sysml_ni(sysml_integer i) { return (sysml_num){false, i, 0}; }
static inline sysml_num sysml_nr(sysml_real r) { return (sysml_num){true, SYSML_I(0), r}; }
static inline sysml_real sysml_num_real(sysml_num n) { return n.real ? n.r : sysml_to_real(n.i); }

/* Orders two numbers exactly, whatever their kinds. */
static inline int sysml_ncmp(sysml_num a, sysml_num b) {
	if (!a.real && !b.real) return sysml_icmp(a.i, b.i);
	if (!a.real) return sysml_cmp_ir(a.i, b.r);
	if (!b.real) return -sysml_cmp_ir(b.i, a.r);
	return a.r < b.r ? -1 : (a.r > b.r ? 1 : 0);
}

/* '===' of two numbers: the same kind and value. */
static inline sysml_bool sysml_nsame(sysml_num a, sysml_num b) { return a.real == b.real && sysml_ncmp(a, b) == 0; }
static inline sysml_bool sysml_num_eq(sysml_num a, sysml_num b) { return sysml_ncmp(a, b) == 0; }
static inline const char *sysml_num_kind(sysml_num n) { return n.real ? "a Real" : "an Integer"; }

static inline sysml_real sysml_finite(sysml_real r) {
	if (__builtin_expect(!isfinite(r), 0)) sysml_fail("arithmetic overflow: result is not a finite Real");
	return r;
}

/* Library functions: a NaN result is a domain error, an infinity an overflow. */
#define SYSML_PI 3.141592653589793

static inline sysml_real sysml_lib_real(sysml_real r) {
	if (__builtin_expect(isnan(r), 0)) sysml_fail("arithmetic domain error: argument outside the function's domain");
	if (__builtin_expect(isinf(r), 0)) sysml_fail("arithmetic overflow: result is not a finite Real");
	return r;
}

/* The Integer a whole Real is, exactly. */
static inline sysml_integer sysml_lib_int(sysml_real x) {
	if (__builtin_expect(isnan(x), 0)) sysml_fail("arithmetic domain error: argument outside the function's domain");
	return sysml_int_of_real(x);
}

static inline sysml_integer sysml_iabs(sysml_integer a) { return sysml_isign(a) < 0 ? sysml_neg(a) : a; }

static inline sysml_integer sysml_imax(sysml_integer a, sysml_integer b) { return sysml_icmp(a, b) > 0 ? a : b; }
static inline sysml_integer sysml_imin(sysml_integer a, sysml_integer b) { return sysml_icmp(a, b) < 0 ? a : b; }

static inline sysml_integer sysml_natural_arg(sysml_integer v) {
	if (__builtin_expect(sysml_isign(v) < 0, 0)) sysml_fail("type mismatch: requires Natural arguments");
	return v;
}

/* Both arguments are checked, first then second, before comparing. */
static inline sysml_integer sysml_natural_max(sysml_integer a, sysml_integer b) {
	sysml_natural_arg(a); sysml_natural_arg(b);
	return sysml_imax(a, b);
}
static inline sysml_integer sysml_natural_min(sysml_integer a, sysml_integer b) {
	sysml_natural_arg(a); sysml_natural_arg(b);
	return sysml_imin(a, b);
}

/* Zero signs follow Go's math.Max/Min: max prefers +0, min prefers -0. */
static inline sysml_real sysml_rmax(sysml_real a, sysml_real b) {
	if (a == 0 && b == 0) return signbit(a) ? b : a;
	return a > b ? a : b;
}

static inline sysml_real sysml_rmin(sysml_real a, sysml_real b) {
	if (a == 0 && b == 0) return signbit(a) ? a : b;
	return a < b ? a : b;
}

static inline sysml_real sysml_tan(sysml_real t) { return sysml_lib_real(sin(t) / cos(t)); }
static inline sysml_real sysml_cot(sysml_real t) { return sysml_lib_real(cos(t) / sin(t)); }

static inline sysml_real sysml_ln(sysml_real x) {
	if (__builtin_expect(x <= 0, 0)) sysml_fail("arithmetic domain error: the logarithm of a non-positive argument is not a Real (requires x > 0.0)");
	return sysml_lib_real(log(x));
}

static inline sysml_real sysml_log(sysml_real x, sysml_real base) {
	if (__builtin_expect(x <= 0, 0)) sysml_fail("arithmetic domain error: the logarithm of a non-positive argument is not a Real (requires x > 0.0)");
	if (__builtin_expect(base <= 0, 0)) sysml_fail("arithmetic domain error: a non-positive base has no logarithm (requires base > 0.0)");
	if (__builtin_expect(base == 1, 0)) sysml_fail("arithmetic domain error: base 1.0 has no logarithm");
	if (base == 10) return sysml_lib_real(log10(x));
	if (base == 2) return sysml_lib_real(log2(x));
	return sysml_lib_real(log(x) / log(base));
}

static inline sysml_real sysml_atan2(sysml_real y, sysml_real x) {
	if (__builtin_expect(y == 0 && x == 0, 0)) sysml_fail("arithmetic domain error: atan2(0.0, 0.0) has no angle");
	return sysml_lib_real(atan2(y, x));
}

static inline sysml_real sysml_rdiv(sysml_real a, sysml_real b) {
	if (__builtin_expect(b == 0, 0)) sysml_fail("division by zero");
	return sysml_finite(a / b);
}

static inline sysml_real sysml_rmod(sysml_real a, sysml_real b) {
	if (__builtin_expect(b == 0, 0)) sysml_fail("division by zero");
	return sysml_finite(fmod(a, b));
}

/* a ** n for a non-negative n, refused before it is computed when the result
   would exceed the size budget. */
static sysml_integer sysml_ipow(sysml_integer a, sysml_integer n) {
	if (sysml_isign(n) == 0) return SYSML_I(1);
	if (!a.big && (a.small == 0 || a.small == 1)) return a;
	if (!a.big && a.small == -1) return sysml_low_bit(n) ? a : SYSML_I(1);
	sysml_mag x;
	sysml_view(a, &x);
	/* |a| >= 2, so a ** n has at least (bitlen(a)-1)*n+1 bits. */
	int64_t lower;
	if (n.big || __builtin_mul_overflow(sysml_mag_bits(x.d, x.n) - 1, n.small, &lower) || __builtin_add_overflow(lower, 1, &lower)) lower = INT64_MAX;
	if (lower > 64 && lower > sysml_max_integer_bits) sysml_size_fail(lower);
	sysml_integer res = SYSML_I(1);
	for (int64_t k = n.small;;) {
		if (k & 1) res = sysml_mul_exact(res, a);
		if ((k >>= 1) == 0) break;
		a = sysml_mul_exact(a, a);
	}
	return sysml_sized(res);
}

static sysml_real sysml_rpow(sysml_real base, sysml_real exp) {
	if (base == 0 && exp < 0) sysml_fail("arithmetic domain: 0 ** negative exponent is undefined");
	if (base < 0 && exp != trunc(exp)) sysml_fail("arithmetic domain: negative base with fractional exponent is not a Real");
	return sysml_finite(pow(base, exp));
}

/* The decimal point snprintf writes and strtod reads under the active locale. */
static const char *sysml_locale_point(void) {
	const char *dp = localeconv()->decimal_point;
	return (dp == NULL || *dp == 0) ? "." : dp;
}

/* Rewrites the locale's decimal point in buf, if present, as '.'. */
static void sysml_dot_point(char *buf) {
	const char *dp = sysml_locale_point();
	if (strcmp(dp, ".") == 0) return;
	char *at = strstr(buf, dp);
	if (at == NULL) return;
	*at = '.';
	memmove(at + 1, at + strlen(dp), strlen(at + strlen(dp)) + 1);
}

/* strtod over decimal notation s ('.' point), whatever the active locale's point is;
 * errno is ERANGE after an overflow or underflow. */
static double sysml_strtod_decimal(const char *s) {
	const char *dp = sysml_locale_point();
	const char *dot = strchr(s, '.');
	errno = 0;
	if (dot == NULL || strcmp(dp, ".") == 0) return strtod(s, NULL);
	size_t head = (size_t)(dot - s), n = strlen(s) + strlen(dp);
	char small[64];
	char *buf = n <= sizeof small ? small : malloc(n);
	if (buf == NULL) {
		fputs("out of memory\n", stderr);
		exit(1);
	}
	memcpy(buf, s, head);
	strcpy(buf + head, dp);
	strcat(buf + head, dot + 1);
	errno = 0;
	double v = strtod(buf, NULL);
	int saved = errno;
	if (buf != small) free(buf);
	errno = saved;
	return v;
}

/* Writes r to buf as [-]d[.ddd]e[+-]xx with the fewest digits that read back as r,
 * trying the nearest p-digit decimal then its upper neighbour (wider above powers of two). */
static void sysml_shortest(sysml_real r, char *buf, size_t size) {
	for (int prec = 1; prec <= 17; prec++) {
		snprintf(buf, size, "%.*e", prec - 1, r);
		sysml_dot_point(buf);
		if (sysml_strtod_decimal(buf) == r) return;
		char up[40];
		snprintf(up, sizeof up, "%s", buf);
		char *first = up + (up[0] == '-');
		char *d = strchr(up, 'e') - 1;
		while (d >= first && (*d == '.' || *d == '9')) {
			if (*d == '9') *d = '0';
			d--;
		}
		if (d < first) continue; /* carried out: a shorter spelling already failed */
		(*d)++;
		if (sysml_strtod_decimal(up) == r) {
			snprintf(buf, size, "%s", up);
			return;
		}
	}
}

/* Appends c to the text being formatted into out, which holds size bytes. */
static void sysml_put(char *out, size_t size, size_t *n, char c) {
	if (*n + 1 < size) out[(*n)++] = c;
	out[*n] = 0;
}

/* Formats r as the interpreter does: shortest round-trip digits, positional
 * for 1e-4 <= |r| < 1e21 and for 0, exponent form otherwise, always with a
 * fraction or exponent so a whole Real still reads as a Real. */
static void sysml_format_real(sysml_real r, char *out, size_t size) {
	char buf[40];
	sysml_shortest(r, buf, sizeof buf);
	sysml_real a = fabs(r);
	if (r != 0 && (a < 1e-4 || a >= 1e21)) {
		snprintf(out, size, "%s", buf);
		return;
	}
	/* buf is [-]d[.ddd]e[+-]xx: reassemble the digits around the point. */
	char digits[24];
	int nd = 0, exp10 = 0;
	size_t n = 0;
	const char *p = buf;
	out[0] = 0;
	if (*p == '-') { sysml_put(out, size, &n, '-'); p++; }
	for (; *p && *p != 'e'; p++) if (*p != '.') digits[nd++] = *p;
	exp10 = atoi(p + 1);
	int intdigits = exp10 + 1;
	if (intdigits <= 0) {
		sysml_put(out, size, &n, '0');
		sysml_put(out, size, &n, '.');
		for (int i = intdigits; i < 0; i++) sysml_put(out, size, &n, '0');
		for (int i = 0; i < nd; i++) sysml_put(out, size, &n, digits[i]);
	} else {
		for (int i = 0; i < intdigits; i++) sysml_put(out, size, &n, i < nd ? digits[i] : '0');
		if (nd > intdigits) {
			sysml_put(out, size, &n, '.');
			for (int i = intdigits; i < nd; i++) sysml_put(out, size, &n, digits[i]);
		} else {
			sysml_put(out, size, &n, '.');
			sysml_put(out, size, &n, '0');
		}
	}
}

static void sysml_print_real_value(sysml_real r) {
	char text[64];
	sysml_format_real(r, text, sizeof text);
	fputs(text, stdout);
}

static void sysml_print_real(sysml_real r) {
	sysml_print_real_value(r);
	fputc('\n', stdout);
}

/* A number as the interpreter prints it, in the arena. */
static const char *sysml_show_num(sysml_num n) {
	if (!n.real) return sysml_show_int(n.i);
	char *text = sysml_alloc(64);
	sysml_format_real(n.r, text, 64);
	return text;
}

static void sysml_print_num_value(sysml_num n) { fputs(sysml_show_num(n), stdout); }

static void sysml_print_num(sysml_num n) {
	sysml_print_num_value(n);
	fputc('\n', stdout);
}

/* An Integer argument, of any size; it lives as long as the program. */
static sysml_integer sysml_parse_int(const char *s, const char *name) {
	bool ok;
	sysml_integer v = sysml_int_of_digits(s, &ok);
	if (!ok) {
		fprintf(stderr, "argument %s: %s is not an Integer\n", name, s);
		exit(2);
	}
	if (v.big) {
		size_t size = sizeof(sysml_big) + (size_t)v.big->n * sizeof(uint32_t);
		sysml_big *b = malloc(size);
		if (!b) { fputs("out of memory\n", stderr); exit(1); }
		memcpy(b, v.big, size);
		v.big = b;
	}
	return v;
}

/* The Integer size budget OPENSYSML_MAX_INTEGER_BITS sets. */
static void sysml_read_max_integer_bits(void) {
	const char *raw = getenv("OPENSYSML_MAX_INTEGER_BITS");
	if (!raw) return;
	const char *s = raw;
	while (*s == ' ' || *s == '\t' || *s == '\n') s++;
	if (!*s) return;
	char *end;
	errno = 0;
	long long n = strtoll(s, &end, 10);
	while (*end == ' ' || *end == '\t' || *end == '\n') end++;
	if (*end || errno) {
		fprintf(stderr, "OPENSYSML_MAX_INTEGER_BITS=\"%s\" is not an integer: set it to a positive number of bits of one Integer (default %d)\n", raw, SYSML_DEFAULT_MAX_INTEGER_BITS);
		exit(2);
	}
	if (n <= 0) {
		fprintf(stderr, "OPENSYSML_MAX_INTEGER_BITS=\"%s\" must be greater than zero: the budget is what stops a runaway run (default %d)\n", raw, SYSML_DEFAULT_MAX_INTEGER_BITS);
		exit(2);
	}
	if (n < SYSML_MIN_MAX_INTEGER_BITS) {
		fprintf(stderr, "OPENSYSML_MAX_INTEGER_BITS=\"%s\" must be at least %d: every machine-word Integer fits within it (default %d)\n", raw, SYSML_MIN_MAX_INTEGER_BITS, SYSML_DEFAULT_MAX_INTEGER_BITS);
		exit(2);
	}
	sysml_max_integer_bits = n;
}

/* Whether s is decimal Real notation: optional sign, digits with an optional fraction, optional decimal exponent. */
static bool sysml_real_notation(const char *s) {
	if (*s == '+' || *s == '-') s++;
	int digits = 0;
	for (; *s >= '0' && *s <= '9'; s++) digits++;
	if (*s == '.') {
		s++;
		for (; *s >= '0' && *s <= '9'; s++) digits++;
	}
	if (digits == 0) return false;
	if (*s == 'e' || *s == 'E') {
		s++;
		if (*s == '+' || *s == '-') s++;
		int exponent = 0;
		for (; *s >= '0' && *s <= '9'; s++) exponent++;
		if (exponent == 0) return false;
	}
	return *s == 0;
}

/* Whether the significand of decimal notation s has a nonzero digit, so a zero it parsed to is an underflow. */
static bool sysml_nonzero_notation(const char *s) {
	for (; *s && *s != 'e' && *s != 'E'; s++) {
		if (*s >= '1' && *s <= '9') return true;
	}
	return false;
}

/* Whether s is decimal Integer notation: an optional sign, then digits. */
static bool sysml_int_notation(const char *s) {
	if (*s == '+' || *s == '-') s++;
	if (!*s) return false;
	for (; *s; s++) if (*s < '0' || *s > '9') return false;
	return true;
}

static sysml_real sysml_parse_real(const char *s, const char *name) {
	if (!sysml_real_notation(s)) {
		fprintf(stderr, "argument %s: %s is not a finite Real in decimal notation\n", name, s);
		exit(2);
	}
	double v = sysml_strtod_decimal(s);
	if ((isinf(v) && errno == ERANGE) || (v == 0 && sysml_nonzero_notation(s))) {
		fprintf(stderr, "argument %s: arithmetic overflow: %s is outside the Real range\n", name, s);
		exit(1);
	}
	return v == 0 ? 0.0 : v;
}

static sysml_num sysml_parse_num(const char *s, const char *name) {
	if (sysml_int_notation(s)) return sysml_ni(sysml_parse_int(s, name));
	return sysml_nr(sysml_parse_real(s, name));
}

static sysml_bool sysml_parse_bool(const char *s, const char *name) {
	if (strcmp(s, "true") == 0) return true;
	if (strcmp(s, "false") == 0) return false;
	fprintf(stderr, "argument %s: %s is not a Boolean\n", name, s);
	exit(2);
}
`

// EmitC writes program as a self-contained C translation unit, with a main that
// reads argv and prints the result when withMain is set (errors: status 1, stderr).
func EmitC(w io.Writer, p *Program, withMain bool) error {
	e := &cEmitter{w: w}
	e.collections = p.Collections
	e.records = len(p.Records) > 0
	e.raw(cScalarRuntime())
	e.raw(cEnumRuntime(p))
	e.raw(cFnRuntime(p))
	if len(p.Records) > 0 || len(p.FnCases) > 0 {
		e.raw(cKeepNumbers)
	}
	if p.Collections {
		e.raw(fmt.Sprintf("#define SYSML_DEFAULT_MAX_ELEMENTS %d\n", runtime.DefaultMaxElements))
		e.raw(cSeqRuntime())
		e.raw(cEnumSeqRuntime(p))
		e.raw(cFnSeqRuntime(p))
		e.raw(cRecSeqRuntime(p))
		if len(p.Records) > 0 || len(p.FnCases) > 0 {
			e.raw(cKeepOptStr)
		}
	}
	for _, fn := range p.Funcs {
		e.linef("static %s %s(%s);", cType(fn.Result), fn.Ident, cParams(fn))
	}
	e.raw("\n")
	for _, fn := range p.Funcs {
		e.function(fn)
	}
	e.entry(p.Entry, withMain)
	return e.err
}

type cEmitter struct {
	w      io.Writer
	err    error
	indent int
	result string
	// resultRange is checked on each return of the function being emitted.
	resultRange Range
	temps       int
	// collections brackets every statement with the element budget's release.
	collections bool
	// records frees the previous run's records when a run begins.
	records bool
}

// pure is an operand whose evaluation cannot fail, so its order is immaterial.
func pure(x Expr) bool {
	switch x := x.(type) {
	case IntLit, RealLit, BoolLit, StrLit, EnumLit, Var, NullLit:
		return true
	case ToReal:
		return pure(x.X)
	case ToNum:
		return pure(x.X)
	}
	return false
}

// sequenced evaluates operands left to right into temporaries, as the
// interpreter does, then applies body to them; C leaves argument order open.
func (e *cEmitter) sequenced(operands []Expr, body func(names []string) string) string {
	names := make([]string, len(operands))
	var pre strings.Builder
	for i, x := range operands {
		if pure(x) {
			names[i] = e.expr(x)
			continue
		}
		e.temps++
		names[i] = fmt.Sprintf("sysml_t%d", e.temps)
		fmt.Fprintf(&pre, "%s %s = %s; ", cType(x.Type()), names[i], e.expr(x))
	}
	if pre.Len() == 0 {
		return body(names)
	}
	return fmt.Sprintf("({ %s%s; })", pre.String(), body(names))
}

// narrowed checks v against the range of the feature it is written to.
// An unset value is checked only where strict.
func cNarrowed(v string, t Type, r Range, strict bool) string {
	switch {
	case r == RangeAny:
		return v
	case t.MayUnset():
		return fmt.Sprintf("sysml_narrow_opt(%s, %d, \"%s\", NULL, %t)", v, r.Lower(), r, strict)
	}
	return fmt.Sprintf("sysml_at_least(%s, %d, \"%s\")", v, r.Lower(), r)
}

func (e *cEmitter) raw(s string) {
	if e.err != nil {
		return
	}
	_, e.err = io.WriteString(e.w, s)
}

func (e *cEmitter) linef(format string, args ...any) {
	e.raw(strings.Repeat("\t", e.indent) + fmt.Sprintf(format, args...) + "\n")
}

func cType(t Type) string {
	if t.MayUnset() {
		return "sysml_opt_" + cCapField(t.Concrete())
	}
	if t.IsEnum() || t.IsFn() || t.IsRec() {
		if t.Many() {
			return "sysml_seq_" + cSeqSuffix(t)
		}
		if t.IsRec() {
			return "sysml_rec *"
		}
		if t.IsFn() {
			return "sysml_fn"
		}
		return "sysml_enum"
	}
	if t == TypeRun {
		return "int64_t"
	}
	switch t {
	case TypeInt:
		return "sysml_integer"
	case TypeReal:
		return "sysml_real"
	case TypeBool:
		return "sysml_bool"
	case TypeNum:
		return "sysml_num"
	case TypeString:
		return "sysml_str"
	case TypeSeqInt, TypeSeqReal, TypeSeqBool, TypeSeqNum, TypeSeqString:
		return "sysml_seq_" + cSeqSuffix(t)
	}
	return "void"
}

func cParams(fn *Func) string {
	if len(fn.Params) == 0 {
		return "void"
	}
	parts := make([]string, len(fn.Params))
	for i, p := range fn.Params {
		parts[i] = cType(p.Type) + " " + cLocal(p.Name)
	}
	return strings.Join(parts, ", ")
}

// cLocal namespaces a SysML local so it can never collide with a C keyword or
// a prelude symbol.
func cLocal(name string) string {
	var b strings.Builder
	b.WriteString("v_")
	encodeName(&b, name)
	return b.String()
}

func (e *cEmitter) function(fn *Func) {
	e.linef("static %s %s(%s) {", cType(fn.Result), fn.Ident, cParams(fn))
	e.indent++
	e.linef("sysml_enter();")
	if fn.Run != "" {
		e.linef("int64_t %s = sysml_next_run(); (void)%[1]s;", cLocal(fn.Run))
	}
	for _, p := range fn.Params {
		switch {
		case p.Type.Many():
			if p.Held {
				e.linef(cAssign, cLocal(p.Name), e.expr(Held{X: Var{Name: p.Name, T: p.Type}, Many: p.Mult.MultiValued()}))
			}
			if p.Mult != MultAny || p.Range != RangeAny || p.Unique {
				v := e.checked(Checked{X: Var{Name: p.Name, T: p.Type}, M: p.Mult, R: p.Range, Unique: p.Unique, Where: paramWhere(p.Name)})
				e.linef(cAssign, cLocal(p.Name), v)
			}
		case p.Range != RangeAny:
			e.linef(cAssign, cLocal(p.Name), cNarrowed(cLocal(p.Name), p.Type, p.Range, true))
		}
	}
	e.result = cType(fn.Result)
	e.resultRange = fn.ResultRange
	e.block(fn.Body)
	e.linef("sysml_fail(\"calc %s completed without returning a value\");", fn.Name)
	e.indent--
	e.linef("}")
	e.raw("\n")
}

// block emits statements; with collections, the elements a statement
// materializes are released when it ends, as the interpreter's step does.
func (e *cEmitter) block(stmts []Stmt) {
	releases := false
	for _, s := range stmts {
		if _, ok := s.(Return); !ok {
			releases = true
		}
	}
	if !releases {
		for _, s := range stmts {
			e.stmt(s)
		}
		return
	}
	e.temps++
	held := fmt.Sprintf("sysml_h%d", e.temps)
	if e.collections {
		e.linef("sysml_int %s = sysml_elements;", held)
	}
	for _, s := range stmts {
		if _, ok := s.(Return); ok {
			e.stmt(s)
			continue
		}
		mark := e.arenaMark()
		e.stmt(s)
		if len(escapingSeqs(s)) == 0 {
			e.linef("sysml_arena_release(%s);", mark)
		}
		if e.collections {
			e.linef("sysml_elements = %s;", held)
		}
	}
}

// arenaMark records the arena's top in a fresh local and returns its name.
func (e *cEmitter) arenaMark() string {
	e.temps++
	mark := fmt.Sprintf("sysml_m%d", e.temps)
	e.linef("sysml_mark %s = sysml_arena_mark();", mark)
	return mark
}

// compact releases the arena to mark, keeping the collections and Strings a
// loop pass stored into variables that outlive it.
func (e *cEmitter) compact(kept []Var, mark string) {
	saved := make([]string, len(kept))
	for i, v := range kept {
		e.temps++
		saved[i] = fmt.Sprintf("sysml_k%d", e.temps)
		switch v.T.Concrete() {
		case TypeString:
			e.linef("char *%s = sysml_save_text(%s, %s);", saved[i], cText(v), mark)
			continue
		case TypeInt, TypeNum:
			e.linef("sysml_big *%s = sysml_save_big(%s, %s);", saved[i], cWhole(v), mark)
			continue
		}
		e.linef("%s *%s = sysml_save_%s(%s, %s);", cType(v.T.Elem()), saved[i], cSeqSuffix(v.T), cLocal(v.Name), mark)
	}
	e.linef("sysml_arena_release(%s);", mark)
	for i, v := range kept {
		switch v.T.Concrete() {
		case TypeString:
			e.linef("sysml_restore_text(&%s, %s);", cText(v), saved[i])
			continue
		case TypeInt, TypeNum:
			e.linef("sysml_restore_big(&%s, %s);", cWhole(v), saved[i])
			continue
		}
		e.linef("sysml_restore_%s(&%s, %s);", cSeqSuffix(v.T), cLocal(v.Name), saved[i])
	}
}

// cText is the String a String variable holds; one that may be unset holds an
// empty one while it is.
func cText(v Var) string {
	if v.T.MayUnset() {
		return cLocal(v.Name) + ".v"
	}
	return cLocal(v.Name)
}

// cWhole is the Integer an Integer or number variable holds; a Real number
// and an unset variable hold zero there.
func cWhole(v Var) string {
	name := cText(v)
	if v.T.Concrete() == TypeNum {
		return name + ".i"
	}
	return name
}

// arenaHeld reports whether a value of type t may point into the arena: a
// collection, a String, or an Integer beyond int64.
func arenaHeld(t Type) bool {
	if t.Many() {
		return true
	}
	switch t.Concrete() {
	case TypeString, TypeInt, TypeNum:
		return true
	}
	return false
}

// escapingSeqs lists the collection, String and Integer variables a statement stores
// into that outlive it: its own declaration and assignments to enclosing variables.
func escapingSeqs(s Stmt) []Var {
	var out []Var
	seen := map[string]bool{}
	var walk func(s Stmt, inner map[string]bool)
	walk = func(s Stmt, inner map[string]bool) {
		store := func(name string, t Type) {
			if !arenaHeld(t) || inner[name] || seen[name] {
				return
			}
			seen[name] = true
			out = append(out, Var{Name: name, T: t})
		}
		block := func(stmts []Stmt) {
			scope := map[string]bool{}
			for k := range inner {
				scope[k] = true
			}
			for _, s := range stmts {
				switch d := s.(type) {
				case Declare:
					scope[d.Name] = true
				case Sample:
					scope[d.Dom], scope[d.Rng] = true, true
				}
				walk(s, scope)
			}
		}
		switch s := s.(type) {
		case Declare:
			store(s.Name, s.T)
		case Sample:
			store(s.Dom, s.DomType())
			store(s.Rng, s.RngType())
		case Assign:
			store(s.Name, s.Value.Type())
		case If:
			block(s.Then)
			block(s.Else)
		case While:
			block(s.Body)
		case ForEach:
			block(s.Body)
		}
	}
	walk(s, map[string]bool{})
	return out
}

func (e *cEmitter) stmt(s Stmt) {
	switch s := s.(type) {
	case Declare:
		e.linef("%s %s = %s;", cType(s.T), cLocal(s.Name), cNarrowed(e.declInit(s), s.T, s.Range, false))
	case Assign:
		e.linef(cAssign, cLocal(s.Name), cNarrowed(e.expr(s.Value), s.Value.Type(), s.Range, true))
	case If:
		e.linef("if (%s) {", e.expr(s.Cond))
		e.indent++
		e.block(s.Then)
		e.indent--
		if len(s.Else) > 0 {
			e.linef("} else {")
			e.indent++
			e.block(s.Else)
			e.indent--
		}
		e.linef("}")
	case While:
		mark := e.arenaMark()
		e.linef("while ((sysml_step(1), %s)) {", e.expr(s.Cond))
		e.indent++
		until := ""
		if s.Until != nil {
			// The body is a block of its own: what it declares does not hide a name the condition reads.
			e.linef("{")
			e.indent++
			e.block(s.Body)
			e.indent--
			e.linef("}")
			e.linef("bool sysml_until = %s;", e.expr(s.Until))
			until = "sysml_until"
		} else {
			e.block(s.Body)
		}
		e.compact(escapingSeqs(s), mark)
		if until != "" {
			e.linef("if (%s) break;", until)
		}
		e.indent--
		e.linef("}")
	case ForEach:
		e.forEach(s)
	case Sample:
		e.linef("%s", e.sample(s))
	case Return:
		e.linef("{ %s sysml_r = %s; sysml_leave(); return sysml_r; }", e.result, cNarrowed(e.expr(s.Value), s.Value.Type(), e.resultRange, true))
	default:
		e.err = fmt.Errorf("codegen: C emitter has no case for %T", s)
	}
}

// declInit is a declaration's initial value, null where it states none.
func (e *cEmitter) declInit(d Declare) string {
	if d.Init == nil {
		return fmt.Sprintf("sysml_null_%s()", cSeqSuffix(d.T))
	}
	return e.expr(d.Init)
}

func cZero(t Type) string {
	switch {
	case t.MayUnset():
		return fmt.Sprintf("((%s){0})", cType(t))
	case t == TypeBool:
		return "false"
	case t.Many():
		return fmt.Sprintf("sysml_null_%s()", cSeqSuffix(t))
	case t == TypeNum:
		return "sysml_ni(SYSML_I(0))"
	case t == TypeString:
		return "sysml_str_empty()"
	case t.IsFn():
		return "((sysml_fn){0})"
	case t == TypeInt:
		return "SYSML_I(0)"
	}
	return "0"
}

func (e *cEmitter) expr(x Expr) string {
	switch x := x.(type) {
	case IntLit:
		return e.intLit(x)
	case RealLit:
		return cReal(x.Value)
	case BoolLit:
		if x.Value {
			return "true"
		}
		return "false"
	case StrLit:
		return cStrLit(x.Value)
	case EnumLit:
		return fmt.Sprintf("((sysml_enum)%d)", x.T.Enum.Base+x.I)
	case EnumText:
		return "sysml_literal_str(" + e.expr(x.X) + ")"
	case Refusal:
		return e.refusal(x)
	case Var:
		return cLocal(x.Name)
	case ToReal:
		if x.X.Type().Many() {
			return fmt.Sprintf("sysml_widen_%s(%s)", cSeqSuffix(x.X.Type()), e.expr(x.X))
		}
		if x.X.Type() == TypeNum {
			return "sysml_num_real(" + e.expr(x.X) + ")"
		}
		if x.Exact {
			return "sysml_to_real_exact(" + e.expr(x.X) + ")"
		}
		return "sysml_to_real(" + e.expr(x.X) + ")"
	case ToNum:
		switch x.X.Type() {
		case TypeInt:
			return "sysml_ni(" + e.expr(x.X) + ")"
		case TypeReal:
			return "sysml_nr(" + e.expr(x.X) + ")"
		}
		return fmt.Sprintf("sysml_nums_%s(%s)", cSeqSuffix(x.X.Type()), e.expr(x.X))
	case AsInt:
		return "(" + e.expr(x.X) + ").i"
	case NumSplit:
		ints := make([]string, len(x.Nums))
		for i, v := range x.Nums {
			ints[i] = "!" + cLocal(v.Name) + ".real"
		}
		return fmt.Sprintf("(%s ? %s : %s)", strings.Join(ints, " && "), e.expr(x.Int), e.expr(x.Real))
	case Unary:
		return e.unary(x)
	case Binary:
		return e.binary(x)
	case Cond:
		return fmt.Sprintf("(%s ? %s : %s)", e.expr(x.C), e.expr(x.Then), e.expr(x.Else))
	case Call:
		return e.sequenced(argValues(x.Args), func(names []string) string {
			return fmt.Sprintf("%s(%s)", x.Fn.Ident, strings.Join(callOperands(x.Args, len(x.Fn.Params), names), ", "))
		})
	case LibCall:
		return e.sequenced(argValues(x.Args), func(names []string) string {
			return x.Op.cExpr(callOperands(x.Args, len(x.Op.Operands()), names))
		})
	case Steps:
		return fmt.Sprintf("(sysml_step(%d), %s)", x.N, e.expr(x.X))
	}
	if s, ok := e.seqExpr(x); ok {
		return s
	}
	if s, ok := e.fnExpr(x); ok {
		return s
	}
	e.err = fmt.Errorf("codegen: C emitter has no case for %T", x)
	return "0"
}

// argValues is the arguments' expressions, in source order.
func argValues(args []Arg) []Expr {
	values := make([]Expr, len(args))
	for i, a := range args {
		values[i] = a.Value
	}
	return values
}

// callOperands picks, per parameter, the name of the last argument bound to it.
func callOperands(args []Arg, nParams int, names []string) []string {
	operands := make([]string, nParams)
	for i, a := range args {
		operands[a.Param] = names[i]
	}
	return operands
}

// inParamOrder reports whether a call's arguments are its parameters once each,
// in order, so evaluating the call's operands in place is source order.
func inParamOrder(args []Arg, nParams int) bool {
	if len(args) != nParams {
		return false
	}
	for i, a := range args {
		if a.Param != i {
			return false
		}
	}
	return true
}

// cCmpQ names the comparison of an exact quotient with the whole number x: an
// Integer where x widens one, a Real otherwise.
func cCmpQ(x Expr) string {
	if isWidenedInt(x) {
		return "sysml_cmp_qi"
	}
	return "sysml_cmp_qr"
}

// intLit is an Integer literal; one beyond int64 is a constant of the program.
func (e *cEmitter) intLit(x IntLit) string {
	if x.Big == nil {
		if x.Value == math.MinInt64 {
			return "SYSML_I(INT64_MIN)"
		}
		return fmt.Sprintf("SYSML_I(INT64_C(%d))", x.Value)
	}
	// Least significant limb first, as sysml_big holds them.
	bytes := new(big.Int).Abs(x.Big).Bytes()
	var limbs []string
	for end := len(bytes); end > 0; end -= 4 {
		var limb uint32
		for _, b := range bytes[max(end-4, 0):end] {
			limb = limb<<8 | uint32(b)
		}
		limbs = append(limbs, fmt.Sprintf("0x%xu", limb))
	}
	return fmt.Sprintf("({ static const struct { bool neg; int32_t n; uint32_t d[%d]; } sysml_l = {%t, %d, {%s}}; (sysml_integer){0, (const sysml_big *)&sysml_l}; })", len(limbs), x.Big.Sign() < 0, len(limbs), strings.Join(limbs, ", "))
}

// cReal renders a float64 so the C compiler reads back the identical value.
func cReal(v float64) string {
	s := strconv.FormatFloat(v, 'g', -1, 64)
	if !strings.ContainsAny(s, ".eE") {
		s += ".0"
	}
	return s
}

func (e *cEmitter) unary(x Unary) string {
	operand := e.expr(x.X)
	switch x.Op {
	case ast.OpNot:
		return "(!" + operand + ")"
	case ast.OpPos:
		return operand
	case ast.OpNeg:
		if x.T == TypeInt {
			return "sysml_neg(" + operand + ")"
		}
		if x.Exact {
			return "(0.0 - " + operand + ")"
		}
		return "(-" + operand + ")"
	}
	e.err = fmt.Errorf("codegen: C emitter has no unary case for %s", x.Op)
	return "0"
}

func (e *cEmitter) binary(x Binary) string {
	if q, ok := exactQuotient(x.L); ok && isComparison(x.Op) {
		return e.sequenced([]Expr{q.L, q.R}, func(v []string) string {
			return fmt.Sprintf("({ sysml_nonzero(%s); %s; })", v[1], e.sequenced([]Expr{wholeOperand(x.R)}, func(c []string) string {
				return fmt.Sprintf("(%s(%s, %s, %s) %s 0)", cCmpQ(x.R), v[0], v[1], c[0], cOperator(x.Op))
			}))
		})
	}
	if q, ok := exactQuotient(x.R); ok && isComparison(x.Op) {
		return e.sequenced([]Expr{wholeOperand(x.L), q.L, q.R}, func(v []string) string {
			return fmt.Sprintf("(-%s(%s, sysml_nonzero(%s), %s) %s 0)", cCmpQ(x.L), v[1], v[2], v[0], cOperator(x.Op))
		})
	}
	if i, ok := widenedInt(x.L); ok && isComparison(x.Op) && !isWidenedInt(x.R) {
		return e.sequenced([]Expr{i, x.R}, func(v []string) string {
			return fmt.Sprintf("(sysml_cmp_ir(%s, %s) %s 0)", v[0], v[1], cOperator(x.Op))
		})
	}
	if i, ok := widenedInt(x.R); ok && isComparison(x.Op) && !isWidenedInt(x.L) {
		return e.sequenced([]Expr{x.L, i}, func(v []string) string {
			return fmt.Sprintf("(-sysml_cmp_ir(%s, %s) %s 0)", v[1], v[0], cOperator(x.Op))
		})
	}
	if x.L.Type().IsFn() {
		return e.sequenced([]Expr{x.L, x.R}, func(v []string) string {
			if x.Op == ast.OpNeq {
				return fmt.Sprintf("(!sysml_fn_eq(%s, %s))", v[0], v[1])
			}
			return fmt.Sprintf("sysml_fn_eq(%s, %s)", v[0], v[1])
		})
	}
	switch x.Op {
	case ast.OpAnd, ast.OpConditionalAnd:
		return fmt.Sprintf("(%s && %s)", e.expr(x.L), e.expr(x.R))
	case ast.OpOr, ast.OpConditionalOr:
		return fmt.Sprintf("(%s || %s)", e.expr(x.L), e.expr(x.R))
	case ast.OpImplies:
		return fmt.Sprintf("(!%s || %s)", e.expr(x.L), e.expr(x.R))
	}
	return e.sequenced([]Expr{x.L, x.R}, func(v []string) string { return e.strict(x, v[0], v[1]) })
}

// strict is a binary operator whose operands l and r are already evaluated.
func (e *cEmitter) strict(x Binary, l, r string) string {
	operands := x.L.Type()
	if operands == TypeString {
		if x.Op == ast.OpAdd {
			return fmt.Sprintf("sysml_str_cat(%s, %s)", l, r)
		}
		return fmt.Sprintf("(sysml_str_cmp(%s, %s) %s 0)", l, r, cOperator(x.Op))
	}
	if operands == TypeNum {
		switch x.Op {
		case ast.OpEqEqEq:
			return fmt.Sprintf("sysml_nsame(%s, %s)", l, r)
		case ast.OpNeqEqEq:
			return fmt.Sprintf("(!sysml_nsame(%s, %s))", l, r)
		}
		return fmt.Sprintf("(sysml_ncmp(%s, %s) %s 0)", l, r, cOperator(x.Op))
	}
	switch x.Op {
	case ast.OpAdd, ast.OpSub:
		if operands == TypeInt {
			return fmt.Sprintf("sysml_%s(%s, %s)", map[ast.OperatorKind]string{ast.OpAdd: "add", ast.OpSub: "sub"}[x.Op], l, r)
		}
		return cUnsignedZero(x, fmt.Sprintf("sysml_finite(%s %s %s)", l, cOperator(x.Op), r))
	case ast.OpMul:
		if operands == TypeInt {
			return fmt.Sprintf("sysml_mul(%s, %s)", l, r)
		}
		return cUnsignedZero(x, fmt.Sprintf("sysml_finite(%s * %s)", l, r))
	case ast.OpDiv:
		if operands == TypeInt {
			return fmt.Sprintf("sysml_quot(%s, %s)", l, r)
		}
		return cUnsignedZero(x, fmt.Sprintf("sysml_rdiv(%s, %s)", l, r))
	case ast.OpMod:
		if operands == TypeInt {
			return fmt.Sprintf("sysml_mod(%s, %s)", l, r)
		}
		return cUnsignedZero(x, fmt.Sprintf("sysml_rmod(%s, %s)", l, r))
	case ast.OpPow:
		return e.pow(x, l, r)
	case ast.OpLt, ast.OpLe, ast.OpGt, ast.OpGe, ast.OpEq, ast.OpNeq:
		if operands == TypeInt {
			return fmt.Sprintf("(sysml_icmp(%s, %s) %s 0)", l, r, cOperator(x.Op))
		}
		return fmt.Sprintf("(%s %s %s)", l, cOperator(x.Op), r)
	case ast.OpXor:
		return fmt.Sprintf("(%s != %s)", l, r)
	}
	e.err = fmt.Errorf("codegen: C emitter has no binary case for %s", x.Op)
	return "0"
}

// cUnsignedZero gives an exact Rational operation's zero result no sign.
func cUnsignedZero(x Binary, r string) string {
	if x.Whole && x.Guard {
		return "sysml_whole(" + r + ")"
	}
	if x.Exact {
		return "sysml_unsigned_zero(" + r + ")"
	}
	return r
}

// pow follows semantics.Pow: Integer ** Integer is an Integer, anything else a
// Real. The compiler has already widened the operands of a Real power.
func (e *cEmitter) pow(x Binary, l, r string) string {
	if x.T == TypeInt {
		return fmt.Sprintf("sysml_ipow(%s, %s)", l, r)
	}
	return fmt.Sprintf("sysml_rpow(%s, %s)", l, r)
}

func cOperator(op ast.OperatorKind) string {
	switch op {
	case ast.OpAdd:
		return "+"
	case ast.OpSub:
		return "-"
	case ast.OpMul:
		return "*"
	case ast.OpLt:
		return "<"
	case ast.OpLe:
		return "<="
	case ast.OpGt:
		return ">"
	case ast.OpGe:
		return ">="
	case ast.OpEq:
		return "=="
	case ast.OpNeq:
		return "!="
	}
	return "?"
}

// entry emits the public wrapper and, optionally, main.
func (e *cEmitter) entry(fn *Func, withMain bool) {
	e.linef("/* %s: returns 0 and stores the result, or 1 with sysml_error set. */", fn.Name)
	e.linef("const char *sysml_error(void) { return sysml_error_message; }")
	e.raw("\n")
	params := cParams(fn)
	if params == "void" {
		params = ""
	} else {
		params += ", "
	}
	e.linef("int sysml_run(%s%s *result) {", params, cType(fn.Result))
	e.indent++
	e.linef("sysml_depth = 0;")
	e.linef("sysml_steps = 0;")
	e.linef("sysml_run_begin();")
	if e.collections {
		e.linef("sysml_elements = 0;")
	}
	e.linef("sysml_keep_release();")
	if e.records {
		e.linef("sysml_recs_release();")
	}
	e.linef("if (setjmp(sysml_escape)) return 1;")
	args := make([]string, len(fn.Params))
	for i, p := range fn.Params {
		args[i] = cLocal(p.Name)
		if p.Type.Many() {
			// The run holds its sequence arguments throughout, as the interpreter
			// holds the sequence literals it evaluated them from.
			e.linef("if (%s.shape == SYSML_MANY) sysml_charge(%s.len);", args[i], args[i])
		}
	}
	e.linef("*result = %s(%s);", fn.Ident, strings.Join(args, ", "))
	e.linef("return 0;")
	e.indent--
	e.linef("}")
	if !withMain {
		return
	}
	e.raw("\n")
	e.linef("int main(int argc, char **argv) {")
	e.indent++
	e.linef("long long repeat = 1;")
	e.linef("bool bad_repeat = false;")
	e.linef("if (argc >= 3 && strcmp(argv[1], \"--repeat\") == 0) {")
	e.linef("\tchar *end; errno = 0; repeat = strtoll(argv[2], &end, 10);")
	e.linef("\tbad_repeat = *argv[2] == '\\0' || *end != '\\0' || errno != 0 || repeat < 1;")
	e.linef("\targv += 2; argc -= 2;")
	e.linef("}")
	e.linef("if (bad_repeat || argc != %d) {", len(fn.Params)+1)
	e.indent++
	e.linef("fprintf(stderr, \"usage: %%s [--repeat N]%s\\n\", argv[0]);", cUsage(fn))
	e.linef("return 2;")
	e.indent--
	e.linef("}")
	e.linef("sysml_max_steps = sysml_read_budget(\"OPENSYSML_MAX_STEPS\", \"evaluation steps\", SYSML_DEFAULT_MAX_STEPS);")
	e.linef("sysml_read_max_integer_bits();")
	if e.collections {
		e.linef("sysml_max_elements = sysml_read_budget(\"OPENSYSML_MAX_ELEMENTS\", \"collection elements\", SYSML_DEFAULT_MAX_ELEMENTS);")
	}
	for i, p := range fn.Params {
		parser := cType(p.Type)[len("sysml_"):]
		if p.Type.IsEnum() && !p.Type.Many() {
			parser = cSeqSuffix(p.Type)
		}
		if p.Type == TypeInt {
			parser = "int"
		}
		e.linef("%s %s = sysml_parse_%s(argv[%d], \"%s\");", cType(p.Type), cLocal(p.Name), parser, i+1, p.Name)
	}
	e.linef("%s result = %s;", cType(fn.Result), cZero(fn.Result))
	e.linef("for (long long i = 0; i < repeat; i++) {")
	e.indent++
	e.linef("if (sysml_run(%s)) {", strings.Join(append(args, "&result"), ", "))
	e.indent++
	e.linef("fprintf(stderr, \"%s: %%s\\n\", sysml_error());", fn.Name)
	e.linef("return 1;")
	e.indent--
	e.linef("}")
	e.indent--
	e.linef("}")
	result, res := "result", fn.Result
	if res.MayUnset() {
		unset := runtime.UnsetText
		if fn.HeldResult {
			unset = "[" + unset + "]"
		}
		e.linef("if (result.u) { puts(\"%s\"); return 0; }", unset)
		result, res = "result.v", res.Concrete()
	}
	// A held result is printed as the one-element sequence it is shaped into.
	held := fn.HeldResult && !res.Many()
	if held {
		e.linef("fputc('[', stdout);")
	}
	switch {
	case res.IsEnum() && !res.Many():
		e.linef("sysml_print_enum_value(%s);", result)
	case res.IsFn() && !res.Many():
		e.linef("sysml_print_fn_value(%s);", result)
	case res == TypeInt:
		e.linef("sysml_print_int_value(%s);", result)
	case res == TypeReal:
		e.linef("sysml_print_real_value(%s);", result)
	case res == TypeBool:
		e.linef("fputs(%s ? \"true\" : \"false\", stdout);", result)
	case res == TypeNum:
		e.linef("sysml_print_num_value(%s);", result)
	case res == TypeString:
		e.linef("sysml_print_str_value(%s);", result)
	default:
		e.linef("sysml_print_seq_%s(%s);", cSeqSuffix(res), result)
	}
	if held {
		e.linef("puts(\"]\");")
	} else if !res.Many() {
		e.linef("fputc('\\n', stdout);")
	}
	e.linef("return 0;")
	e.indent--
	e.linef("}")
}

func cUsage(fn *Func) string {
	var b strings.Builder
	for _, p := range fn.Params {
		fmt.Fprintf(&b, " <%s:%s>", p.Name, p.Type)
	}
	return b.String()
}
