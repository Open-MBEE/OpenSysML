package codegen

// cBigRuntime is the Integer of a C program: an int64 while a value fits and
// a sign and magnitude in the arena beyond it, as the interpreter's is.
const cBigRuntime = `
/* An Integer beyond int64: its magnitude, least significant limb first. */
typedef struct sysml_big { bool neg; int32_t n; uint32_t d[]; } sysml_big;
/* A view of any Integer's magnitude; n is 0 for zero. */
typedef struct { bool neg; int32_t n; const uint32_t *d; uint32_t buf[2]; } sysml_mag;

static int64_t sysml_max_integer_bits = SYSML_DEFAULT_MAX_INTEGER_BITS;

static void sysml_view(sysml_integer a, sysml_mag *m) {
	if (a.big) {
		m->neg = a.big->neg; m->n = a.big->n; m->d = a.big->d;
		return;
	}
	uint64_t u = a.small < 0 ? 0 - (uint64_t)a.small : (uint64_t)a.small;
	m->neg = a.small < 0;
	m->buf[0] = (uint32_t)u; m->buf[1] = (uint32_t)(u >> 32);
	m->n = m->buf[1] ? 2 : m->buf[0] ? 1 : 0;
	m->d = m->buf;
}

static inline uint32_t *sysml_limbs(int64_t n) { return (uint32_t *)sysml_alloc((size_t)(n ? n : 1) * sizeof(uint32_t)); }

static inline int32_t sysml_mag_trim(const uint32_t *d, int32_t n) {
	while (n > 0 && d[n - 1] == 0) n--;
	return n;
}

static inline int64_t sysml_mag_bits(const uint32_t *d, int32_t n) {
	return n ? 32 * (int64_t)n - __builtin_clz(d[n - 1]) : 0;
}

static int sysml_mag_cmp(const uint32_t *a, int32_t an, const uint32_t *b, int32_t bn) {
	if (an != bn) return an < bn ? -1 : 1;
	for (int32_t i = an; i-- > 0;)
		if (a[i] != b[i]) return a[i] < b[i] ? -1 : 1;
	return 0;
}

/* r, of an+1 limbs with an >= bn, is a + b. */
static int32_t sysml_mag_add(uint32_t *r, const uint32_t *a, int32_t an, const uint32_t *b, int32_t bn) {
	uint64_t carry = 0;
	for (int32_t i = 0; i < an; i++) {
		carry += (uint64_t)a[i] + (i < bn ? b[i] : 0);
		r[i] = (uint32_t)carry;
		carry >>= 32;
	}
	r[an] = (uint32_t)carry;
	return sysml_mag_trim(r, an + 1);
}

/* r, of an limbs, is a - b for a >= b. */
static int32_t sysml_mag_sub(uint32_t *r, const uint32_t *a, int32_t an, const uint32_t *b, int32_t bn) {
	int64_t borrow = 0;
	for (int32_t i = 0; i < an; i++) {
		int64_t v = (int64_t)a[i] - (i < bn ? b[i] : 0) - borrow;
		borrow = v < 0;
		r[i] = (uint32_t)v;
	}
	return sysml_mag_trim(r, an);
}

/* r, of an+bn limbs, is a * b. */
static int32_t sysml_mag_mul(uint32_t *r, const uint32_t *a, int32_t an, const uint32_t *b, int32_t bn) {
	memset(r, 0, (size_t)(an + bn) * sizeof(uint32_t));
	for (int32_t i = 0; i < an; i++) {
		uint64_t carry = 0;
		for (int32_t j = 0; j < bn; j++) {
			carry += (uint64_t)a[i] * b[j] + r[i + j];
			r[i + j] = (uint32_t)carry;
			carry >>= 32;
		}
		r[i + bn] = (uint32_t)carry;
	}
	return sysml_mag_trim(r, an + bn);
}

/* r, of an+s/32+1 limbs, is a shifted left by s bits. */
static int32_t sysml_mag_shl(uint32_t *r, const uint32_t *a, int32_t an, int64_t s) {
	int32_t limbs = (int32_t)(s / 32), bits = (int32_t)(s % 32);
	memset(r, 0, (size_t)(an + limbs + 1) * sizeof(uint32_t));
	for (int32_t i = 0; i < an; i++) {
		uint64_t v = (uint64_t)a[i] << bits;
		r[i + limbs] |= (uint32_t)v;
		r[i + limbs + 1] |= (uint32_t)(v >> 32);
	}
	return sysml_mag_trim(r, an + limbs + 1);
}

/* a, of an limbs, becomes its quotient by the limb b; the remainder is returned. */
static uint32_t sysml_mag_div1(uint32_t *a, int32_t an, uint32_t b) {
	uint64_t rem = 0;
	for (int32_t i = an; i-- > 0;) {
		rem = rem << 32 | a[i];
		a[i] = (uint32_t)(rem / b);
		rem %= b;
	}
	return (uint32_t)rem;
}

/* q, of an-bn+1 limbs, and r, of bn limbs, are the quotient and remainder of
   a by b, for an >= bn > 0 (Knuth's Algorithm D); their trimmed lengths are set. */
static void sysml_mag_divmod(uint32_t *q, int32_t *qn, uint32_t *r, int32_t *rn, const uint32_t *a, int32_t an, const uint32_t *b, int32_t bn) {
	if (bn == 1) {
		memcpy(q, a, (size_t)an * sizeof(uint32_t));
		r[0] = sysml_mag_div1(q, an, b[0]);
		*qn = sysml_mag_trim(q, an);
		*rn = sysml_mag_trim(r, 1);
		return;
	}
	int s = __builtin_clz(b[bn - 1]);
	uint32_t *u = sysml_limbs((int64_t)an + 1), *v = sysml_limbs(bn);
	for (int32_t i = bn; i-- > 0;)
		v[i] = s ? b[i] << s | (i ? b[i - 1] >> (32 - s) : 0) : b[i];
	u[an] = s ? a[an - 1] >> (32 - s) : 0;
	for (int32_t i = an; i-- > 0;)
		u[i] = s ? a[i] << s | (i ? a[i - 1] >> (32 - s) : 0) : a[i];
	for (int32_t j = an - bn; j >= 0; j--) {
		uint64_t num = (uint64_t)u[j + bn] << 32 | u[j + bn - 1];
		uint64_t qh = num / v[bn - 1], rh = num % v[bn - 1];
		while (qh >> 32 || qh * v[bn - 2] > (rh << 32 | u[j + bn - 2])) {
			qh--;
			rh += v[bn - 1];
			if (rh >> 32) break;
		}
		int64_t borrow = 0;
		uint64_t carry = 0;
		for (int32_t i = 0; i < bn; i++) {
			carry += qh * v[i];
			int64_t t = (int64_t)u[i + j] - (int64_t)(uint32_t)carry - borrow;
			borrow = t < 0;
			u[i + j] = (uint32_t)t;
			carry >>= 32;
		}
		int64_t t = (int64_t)u[j + bn] - (int64_t)carry - borrow;
		u[j + bn] = (uint32_t)t;
		if (t < 0) {
			qh--;
			carry = 0;
			for (int32_t i = 0; i < bn; i++) {
				carry += (uint64_t)u[i + j] + v[i];
				u[i + j] = (uint32_t)carry;
				carry >>= 32;
			}
			u[j + bn] += (uint32_t)carry;
		}
		q[j] = (uint32_t)qh;
	}
	for (int32_t i = 0; i < bn; i++)
		r[i] = s ? u[i] >> s | u[i + 1] << (32 - s) : u[i];
	*qn = sysml_mag_trim(q, an - bn + 1);
	*rn = sysml_mag_trim(r, bn);
}

static void sysml_size_fail(int64_t bits) {
	static char msg[200];
	snprintf(msg, sizeof msg, "integer size limit exceeded: the result needs at least %" PRId64 " bits, beyond the %" PRId64 "-bit Integer size budget (raise OPENSYSML_MAX_INTEGER_BITS to allow more)", bits, sysml_max_integer_bits);
	sysml_fail(msg);
}

/* The Integer of a sign and a magnitude: an int64 when it fits. sized checks the size budget. */
static sysml_integer sysml_wrap(bool neg, const uint32_t *d, int32_t n, bool sized) {
	n = sysml_mag_trim(d, n);
	if (n <= 2) {
		uint64_t u = n ? d[0] | (n == 2 ? (uint64_t)d[1] << 32 : 0) : 0;
		if (!neg && u <= (uint64_t)INT64_MAX) return (sysml_integer){(int64_t)u, NULL};
		if (neg && u <= (uint64_t)1 << 63) return (sysml_integer){(int64_t)(0 - u), NULL};
	}
	if (sized && sysml_mag_bits(d, n) > sysml_max_integer_bits) sysml_size_fail(sysml_mag_bits(d, n));
	sysml_big *b = (sysml_big *)sysml_alloc(sizeof(sysml_big) + (size_t)n * sizeof(uint32_t));
	b->neg = neg;
	b->n = n;
	memcpy(b->d, d, (size_t)n * sizeof(uint32_t));
	return (sysml_integer){0, b};
}

static inline int sysml_isign(sysml_integer a) {
	if (a.big) return a.big->neg ? -1 : 1;
	return (a.small > 0) - (a.small < 0);
}

/* a + b, or a - b when minus is set, where an int64 does not hold it. */
static sysml_integer sysml_big_sum(sysml_integer a, sysml_integer b, bool minus) {
	sysml_mag x, y;
	sysml_view(a, &x);
	sysml_view(b, &y);
	bool yneg = y.neg != minus;
	if (x.neg == yneg) {
		const sysml_mag *hi = x.n >= y.n ? &x : &y, *lo = x.n >= y.n ? &y : &x;
		uint32_t *r = sysml_limbs((int64_t)hi->n + 1);
		return sysml_wrap(x.neg, r, sysml_mag_add(r, hi->d, hi->n, lo->d, lo->n), true);
	}
	int c = sysml_mag_cmp(x.d, x.n, y.d, y.n);
	if (c == 0) return (sysml_integer){0, NULL};
	const sysml_mag *hi = c > 0 ? &x : &y, *lo = c > 0 ? &y : &x;
	uint32_t *r = sysml_limbs(hi->n);
	return sysml_wrap(c > 0 ? x.neg : yneg, r, sysml_mag_sub(r, hi->d, hi->n, lo->d, lo->n), true);
}

static inline sysml_integer sysml_add(sysml_integer a, sysml_integer b) {
	int64_t r;
	if (__builtin_expect(!a.big && !b.big && !__builtin_add_overflow(a.small, b.small, &r), 1)) return (sysml_integer){r, NULL};
	return sysml_big_sum(a, b, false);
}

static inline sysml_integer sysml_sub(sysml_integer a, sysml_integer b) {
	int64_t r;
	if (__builtin_expect(!a.big && !b.big && !__builtin_sub_overflow(a.small, b.small, &r), 1)) return (sysml_integer){r, NULL};
	return sysml_big_sum(a, b, true);
}

static sysml_integer sysml_big_mul(sysml_integer a, sysml_integer b, bool sized) {
	sysml_mag x, y;
	sysml_view(a, &x);
	sysml_view(b, &y);
	if (x.n == 0 || y.n == 0) return (sysml_integer){0, NULL};
	/* The product has at least bitlen(x)+bitlen(y)-1 bits. */
	int64_t bits = sysml_mag_bits(x.d, x.n) + sysml_mag_bits(y.d, y.n) - 1;
	if (sized && bits > sysml_max_integer_bits) sysml_size_fail(bits);
	uint32_t *r = sysml_limbs((int64_t)x.n + y.n);
	return sysml_wrap(x.neg != y.neg, r, sysml_mag_mul(r, x.d, x.n, y.d, y.n), sized);
}

/* a * b whatever its size: a step of a comparison or of a checked power. */
static inline sysml_integer sysml_mul_exact(sysml_integer a, sysml_integer b) {
	int64_t r;
	if (!a.big && !b.big && !__builtin_mul_overflow(a.small, b.small, &r)) return (sysml_integer){r, NULL};
	return sysml_big_mul(a, b, false);
}

/* a, failing when it is beyond the size budget. */
static inline sysml_integer sysml_sized(sysml_integer a) {
	if (a.big && sysml_mag_bits(a.big->d, a.big->n) > sysml_max_integer_bits) sysml_size_fail(sysml_mag_bits(a.big->d, a.big->n));
	return a;
}

/* a * 2^s. */
static sysml_integer sysml_shift(sysml_integer a, int64_t s) {
	sysml_mag x;
	sysml_view(a, &x);
	uint32_t *r = sysml_limbs((int64_t)x.n + s / 32 + 1);
	return sysml_wrap(x.neg, r, sysml_mag_shl(r, x.d, x.n, s), false);
}

static inline bool sysml_low_bit(sysml_integer a) { return a.big ? a.big->d[0] & 1 : a.small & 1; }

static inline sysml_integer sysml_mul(sysml_integer a, sysml_integer b) {
	int64_t r;
	if (__builtin_expect(!a.big && !b.big && !__builtin_mul_overflow(a.small, b.small, &r), 1)) return (sysml_integer){r, NULL};
	return sysml_big_mul(a, b, true);
}

static inline sysml_integer sysml_neg(sysml_integer a) {
	if (__builtin_expect(!a.big && a.small != INT64_MIN, 1)) return (sysml_integer){-a.small, NULL};
	sysml_mag x;
	sysml_view(a, &x);
	return sysml_wrap(!x.neg, x.d, x.n, false);
}

/* The remainder of a by b, with the sign of a. */
static sysml_integer sysml_mod(sysml_integer a, sysml_integer b) {
	if (sysml_isign(b) == 0) sysml_fail("division by zero");
	if (!a.big && !b.big) return (sysml_integer){b.small == -1 ? 0 : a.small % b.small, NULL};
	sysml_mag x, y;
	sysml_view(a, &x);
	sysml_view(b, &y);
	if (sysml_mag_cmp(x.d, x.n, y.d, y.n) < 0) return a;
	uint32_t *q = sysml_limbs((int64_t)x.n - y.n + 1), *r = sysml_limbs(y.n);
	int32_t qn, rn;
	sysml_mag_divmod(q, &qn, r, &rn, x.d, x.n, y.d, y.n);
	return sysml_wrap(x.neg, r, rn, false);
}

static inline int sysml_icmp(sysml_integer a, sysml_integer b) {
	if (__builtin_expect(!a.big && !b.big, 1)) return (a.small > b.small) - (a.small < b.small);
	sysml_mag x, y;
	sysml_view(a, &x);
	sysml_view(b, &y);
	if (x.neg != y.neg) return x.neg ? -1 : 1;
	int c = sysml_mag_cmp(x.d, x.n, y.d, y.n);
	return x.neg ? -c : c;
}

static inline bool sysml_ieq(sysml_integer a, sysml_integer b) { return sysml_icmp(a, b) == 0; }

/* The binary64 nearest a / b, ties to even, for magnitudes a and b > 0. */
static double sysml_mag_ratio(const uint32_t *a, int32_t an, const uint32_t *b, int32_t bn) {
	if (an == 0) return 0;
	/* The quotient of a * 2^k by b has 55 or 56 bits: two or more beyond a binary64's 53. */
	int64_t k = 55 - (sysml_mag_bits(a, an) - sysml_mag_bits(b, bn));
	uint32_t *n = sysml_limbs((int64_t)an + (k > 0 ? k / 32 : 0) + 1), *d = sysml_limbs((int64_t)bn + (k < 0 ? -k / 32 : 0) + 1);
	int32_t nn = sysml_mag_shl(n, a, an, k > 0 ? k : 0), dn = sysml_mag_shl(d, b, bn, k < 0 ? -k : 0);
	uint32_t *q = sysml_limbs((int64_t)nn - dn + 1), *r = sysml_limbs(dn);
	int32_t qn, rn;
	sysml_mag_divmod(q, &qn, r, &rn, n, nn, d, dn);
	uint64_t v = q[0] | (qn > 1 ? (uint64_t)q[1] << 32 : 0);
	/* Drop what a binary64 does not hold: bits past 53, and those below the least subnormal. */
	int64_t drop = 64 - __builtin_clzll(v) - 53;
	if (drop < k - 1074) drop = k - 1074;
	if (drop >= 64) return 0;
	uint64_t rest = v & (((uint64_t)1 << drop) - 1), half = (uint64_t)1 << (drop - 1);
	v >>= drop;
	if (rest > half || (rest == half && (rn > 0 || (v & 1)))) v++;
	return ldexp((double)v, (int)(drop - k > 2000 ? 2000 : drop - k));
}

/* The Real nearest an Integer. */
static inline sysml_real sysml_to_real(sysml_integer a) {
	if (__builtin_expect(!a.big, 1)) return (sysml_real)a.small;
	static const uint32_t one[1] = {1};
	double f = sysml_mag_ratio(a.big->d, a.big->n, one, 1);
	return a.big->neg ? -f : f;
}

/* The order of an Integer and a Real, compared exactly. */
static int sysml_cmp_ir(sysml_integer a, sysml_real r) {
	if (!a.big && a.small > -((int64_t)1 << 53) && a.small < (int64_t)1 << 53) {
		sysml_real f = (sysml_real)a.small;
		return (f > r) - (f < r);
	}
	sysml_mag x;
	sysml_view(a, &x);
	if (isnan(r)) return 0;
	if (x.neg != (r < 0) || r == 0) return x.neg ? -1 : 1;
	if (isinf(r)) return r > 0 ? -1 : 1;
	int e;
	/* |r| is m * 2^e for the 53-bit whole number m; below 2^53 it is under |a|. */
	uint64_t m = (uint64_t)ldexp(frexp(fabs(r), &e), 53);
	e -= 53;
	int c = 1;
	if (e >= 0) {
		uint32_t w[2] = {(uint32_t)m, (uint32_t)(m >> 32)};
		uint32_t *s = sysml_limbs(3 + e / 32);
		c = sysml_mag_cmp(x.d, x.n, s, sysml_mag_shl(s, w, 2, e));
	}
	return x.neg ? -c : c;
}

/* The whole Real x as an Integer. */
static sysml_integer sysml_int_of_real(sysml_real x) {
	if (x >= -9223372036854775808.0 && x < 9223372036854775808.0) return (sysml_integer){(int64_t)x, NULL};
	int e;
	uint64_t m = (uint64_t)ldexp(frexp(fabs(x), &e), 53);
	uint32_t w[2] = {(uint32_t)m, (uint32_t)(m >> 32)};
	uint32_t *s = sysml_limbs(3 + (e - 53) / 32);
	return sysml_wrap(x < 0, s, sysml_mag_shl(s, w, 2, e - 53), false);
}

/* The decimal digits of an Integer, in the arena; *len is their count. */
static const char *sysml_int_digits(sysml_integer a, int64_t *len) {
	if (!a.big) {
		char *out = (char *)sysml_alloc(24);
		*len = snprintf(out, 24, "%" PRId64, a.small);
		return out;
	}
	int32_t n = a.big->n;
	uint32_t *w = sysml_limbs(n);
	memcpy(w, a.big->d, (size_t)n * sizeof(uint32_t));
	/* Nine digits a pass, least significant first; a limb holds under ten digits. */
	int64_t cap = (int64_t)n * 10 + 2, at = cap;
	char *out = (char *)sysml_alloc((size_t)cap);
	while (n > 0) {
		uint32_t rem = sysml_mag_div1(w, n, 1000000000u);
		n = sysml_mag_trim(w, n);
		for (int i = 0; i < 9 && (n > 0 || rem > 0); i++) {
			out[--at] = (char)('0' + rem % 10);
			rem /= 10;
		}
	}
	if (a.big->neg) out[--at] = '-';
	*len = cap - at;
	return out + at;
}

static void sysml_print_int_value(sysml_integer a) {
	int64_t len;
	const char *s = sysml_int_digits(a, &len);
	fwrite(s, 1, (size_t)len, stdout);
}

/* An Integer as the interpreter prints it, in the arena. */
static const char *sysml_show_int(sysml_integer a) {
	int64_t len;
	const char *s = sysml_int_digits(a, &len);
	char *text = sysml_alloc((size_t)len + 1);
	memcpy(text, s, (size_t)len);
	text[len] = 0;
	return text;
}

/* The exact ratio a / b rounded once to binary64 (round to nearest, ties to even). */
static sysml_real sysml_quot(sysml_integer a, sysml_integer b) {
	if (__builtin_expect(sysml_isign(b) == 0, 0)) sysml_fail("division by zero");
	/* The Rational zero has no sign, whatever the divisor's. */
	if (sysml_isign(a) == 0) return 0.0;
	if (!a.big && !b.big && a.small > -(1LL << 53) && a.small < (1LL << 53) && b.small > -(1LL << 53) && b.small < (1LL << 53))
		return (sysml_real)a.small / (sysml_real)b.small;
	sysml_mag x, y;
	sysml_view(a, &x);
	sysml_view(b, &y);
	double q = sysml_mag_ratio(x.d, x.n, y.d, y.n);
	if (q == 0 && x.n) sysml_fail("arithmetic overflow: a nonzero Rational below the least Real");
	if (isinf(q)) sysml_fail("arithmetic overflow: result is not a finite Real");
	return x.neg != y.neg ? -q : q;
}

/* A hash of an Integer, equal for equal ones. */
static uint64_t sysml_int_key(sysml_integer a) {
	if (!a.big) return (uint64_t)a.small;
	uint64_t h = a.big->neg ? 0x9E3779B97F4A7C15ull : 0;
	for (int32_t i = 0; i < a.big->n; i++) h = (h ^ a.big->d[i]) * 0x100000001B3ull;
	return h;
}

/* A big Integer copied out of the arena when a release to m would reclaim it; NULL when it would not. */
static sysml_big *sysml_save_big(sysml_integer a, sysml_mark m) {
	if (!a.big || !sysml_above_mark(a.big, m)) return NULL;
	size_t size = sizeof(sysml_big) + (size_t)a.big->n * sizeof(uint32_t);
	sysml_big *t = malloc(size);
	if (!t) sysml_fail("out of memory");
	return memcpy(t, a.big, size);
}

/* Moves a saved copy back into the arena after the release. */
static void sysml_restore_big(sysml_integer *a, sysml_big *t) {
	if (!t) return;
	size_t size = sizeof(sysml_big) + (size_t)t->n * sizeof(uint32_t);
	a->big = memcpy(sysml_alloc(size), t, size);
	free(t);
}

/* a held until the run ends, for a record or a closure keeping it. */
static sysml_integer sysml_keep_int(sysml_integer a) {
	if (a.big && sysml_in_arena(a.big)) a.big = sysml_keep(a.big, sizeof(sysml_big) + (size_t)a.big->n * sizeof(uint32_t));
	return a;
}

static void sysml_save_ints(sysml_integer *t, int64_t n) {
	for (int64_t i = 0; i < n; i++) {
		if (!t[i].big) continue;
		size_t size = sizeof(sysml_big) + (size_t)t[i].big->n * sizeof(uint32_t);
		sysml_big *b = malloc(size);
		if (!b) sysml_fail("out of memory");
		t[i].big = memcpy(b, t[i].big, size);
	}
}

static void sysml_restore_ints(sysml_integer *d, int64_t n) {
	for (int64_t i = 0; i < n; i++) {
		if (!d[i].big) continue;
		sysml_big *t = (sysml_big *)d[i].big;
		size_t size = sizeof(sysml_big) + (size_t)t->n * sizeof(uint32_t);
		d[i].big = memcpy(sysml_alloc(size), t, size);
		free(t);
	}
}

static void sysml_keep_ints(sysml_integer *d, int64_t n) {
	for (int64_t i = 0; i < n; i++) d[i] = sysml_keep_int(d[i]);
}

/* The Integer of decimal digits with an optional sign; ok is false for anything else. */
static sysml_integer sysml_int_of_digits(const char *s, bool *ok) {
	bool neg = *s == '-';
	if (*s == '-' || *s == '+') s++;
	size_t len = strlen(s);
	*ok = len > 0 && strspn(s, "0123456789") == len;
	if (!*ok) return (sysml_integer){0, NULL};
	uint32_t *d = sysml_limbs((int64_t)(len / 9) + 2);
	int32_t n = 0;
	for (size_t i = 0; i < len;) {
		uint64_t carry = 0, scale = 1;
		for (size_t end = i + (len - i - 1) % 9 + 1; i < end; i++) {
			carry = carry * 10 + (uint64_t)(s[i] - '0');
			scale *= 10;
		}
		for (int32_t j = 0; j < n; j++) {
			carry += (uint64_t)d[j] * scale;
			d[j] = (uint32_t)carry;
			carry >>= 32;
		}
		if (carry) d[n++] = (uint32_t)carry;
	}
	return sysml_wrap(neg, d, n, false);
}
`
