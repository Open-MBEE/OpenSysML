package codegen

import (
	"fmt"
	"strings"
)

// cStrRuntime is the String runtime of a generated C program: UTF-8 bytes in
// the arena, compared bytewise (which orders code points), counted and cut by
// code point, and quoted as the interpreter prints a String.
const cStrRuntime = `
typedef struct { sysml_int len; const char *p; } sysml_str;

static inline sysml_str sysml_str_empty(void) { return (sysml_str){0, ""}; }

static sysml_str sysml_str_cat(sysml_str a, sysml_str b) {
	if (!a.len) return b;
	if (!b.len) return a;
	char *p = sysml_alloc((size_t)a.len + (size_t)b.len);
	memcpy(p, a.p, (size_t)a.len);
	memcpy(p + a.len, b.p, (size_t)b.len);
	return (sysml_str){a.len + b.len, p};
}

static int sysml_str_cmp(sysml_str a, sysml_str b) {
	sysml_int n = a.len < b.len ? a.len : b.len;
	int c = n ? memcmp(a.p, b.p, (size_t)n) : 0;
	if (c) return c < 0 ? -1 : 1;
	return (a.len > b.len) - (a.len < b.len);
}

static inline sysml_bool sysml_str_eq(sysml_str a, sysml_str b) {
	return a.len == b.len && (!a.len || memcmp(a.p, b.p, (size_t)a.len) == 0);
}

static inline uint64_t sysml_str_key(sysml_str s) {
	uint64_t h = 0xcbf29ce484222325ULL;
	for (sysml_int i = 0; i < s.len; i++) h = (h ^ (unsigned char)s.p[i]) * 0x100000001b3ULL;
	return h;
}

/* The byte offset of code point k (from 0) of s, its length at k == the count. */
static sysml_int sysml_str_offset(sysml_str s, sysml_int k) {
	sysml_int i = 0;
	for (; i < s.len; i++) if (((unsigned char)s.p[i] & 0xC0) != 0x80 && k-- == 0) return i;
	return i;
}

static sysml_int sysml_str_length(sysml_str s) {
	sysml_int n = 0;
	for (sysml_int i = 0; i < s.len; i++) n += ((unsigned char)s.p[i] & 0xC0) != 0x80;
	return n;
}

static sysml_str sysml_substring(sysml_str x, sysml_int lo, sysml_int hi) {
	sysml_int n = sysml_str_length(x);
	if (lo < 1) sysml_failf("index out of range: function StringFunctions::Substring lower character %lld is outside 1..%lld", (long long)lo, (long long)n);
	if (lo > hi) return sysml_str_empty();
	if (hi > n) sysml_failf("index out of range: function StringFunctions::Substring upper character %lld is outside 1..%lld", (long long)hi, (long long)n);
	sysml_int a = sysml_str_offset(x, lo - 1), b = sysml_str_offset(x, hi);
	return (sysml_str){b - a, x.p + a};
}

static sysml_str sysml_str_of(const char *text) {
	size_t n = strlen(text);
	char *p = sysml_alloc(n ? n : 1);
	memcpy(p, text, n);
	return (sysml_str){(sysml_int)n, p};
}

static sysml_str sysml_int_string(sysml_int v) {
	char text[24];
	snprintf(text, sizeof text, "%" PRId64, v);
	return sysml_str_of(text);
}

static sysml_str sysml_natural_string(sysml_int v) {
	if (v < 0) sysml_failf("type mismatch: function NaturalFunctions::ToString parameter \"x\" requires a Natural value, got %" PRId64, v);
	return sysml_int_string(v);
}

static sysml_str sysml_real_string(sysml_real r) {
	char text[64];
	sysml_format_real(r, text, sizeof text);
	return sysml_str_of(text);
}

static inline sysml_str sysml_bool_string(sysml_bool b) { return b ? (sysml_str){4, "true"} : (sysml_str){5, "false"}; }

/* The code point at p, as Go's utf8.DecodeRune reads it: U+FFFD of width 1 for a malformed byte. */
static uint32_t sysml_decode(const unsigned char *p, sysml_int n, int *w) {
	unsigned c = p[0];
	*w = 1;
	if (c < 0x80) return c;
	int need = c >= 0xF0 ? 3 : c >= 0xE0 ? 2 : c >= 0xC2 ? 1 : 0;
	if (!need || c > 0xF4 || n <= need) return 0xFFFD;
	uint32_t r = c & (0x3F >> need);
	for (int i = 1; i <= need; i++) {
		if ((p[i] & 0xC0) != 0x80) return 0xFFFD;
		r = r << 6 | (p[i] & 0x3F);
	}
	if ((need == 2 && r < 0x800) || (need == 3 && (r < 0x10000 || r > 0x10FFFF)) || (r >= 0xD800 && r <= 0xDFFF)) return 0xFFFD;
	*w = need + 1;
	return r;
}

/* s as the String literal that reads back to it: only the quote, the backslash and \b \t \n \f \r are escaped. */
static const char *sysml_quote(sysml_str s) {
	char *out = sysml_alloc((size_t)s.len * 2 + 3);
	size_t k = 0;
	out[k++] = '"';
	for (sysml_int i = 0; i < s.len; i++) {
		char c = s.p[i], esc = 0;
		switch (c) {
		case '"': case '\\': esc = c; break;
		case '\b': esc = 'b'; break;
		case '\t': esc = 't'; break;
		case '\n': esc = 'n'; break;
		case '\f': esc = 'f'; break;
		case '\r': esc = 'r'; break;
		}
		if (esc) { out[k++] = '\\'; out[k++] = esc; }
		else out[k++] = c;
	}
	out[k++] = '"';
	out[k] = 0;
	return out;
}

static void sysml_print_str_value(sysml_str s) { fputs(sysml_quote(s), stdout); }
static void sysml_print_str(sysml_str s) { sysml_print_str_value(s); fputc('\n', stdout); }
static void sysml_format_str(sysml_str s, char *out, size_t size) { snprintf(out, size, "%s", sysml_quote(s)); }
static const char *sysml_show_str(sysml_str s) { return sysml_quote(s); }

/* Copies a String's bytes out of the arena when a release to m would reclaim them; NULL when it would not. */
static char *sysml_save_text(sysml_str s, sysml_mark m) {
	if (!s.len || !sysml_above_mark(s.p, m)) return NULL;
	char *t = malloc((size_t)s.len);
	if (!t) sysml_fail("out of memory");
	memcpy(t, s.p, (size_t)s.len);
	return t;
}

/* Moves saved bytes back into the arena after the release. */
static void sysml_restore_text(sysml_str *s, char *t) {
	if (!t) return;
	char *p = sysml_alloc((size_t)s->len);
	memcpy(p, t, (size_t)s->len);
	free(t);
	s->p = p;
}

/* The bytes of saved String elements, copied out with the elements and back in after. */
static void sysml_save_texts(sysml_str *t, sysml_int n) {
	for (sysml_int i = 0; i < n; i++) {
		if (!t[i].len) continue;
		char *p = malloc((size_t)t[i].len);
		if (!p) sysml_fail("out of memory");
		memcpy(p, t[i].p, (size_t)t[i].len);
		t[i].p = p;
	}
}

static void sysml_restore_texts(sysml_str *d, sysml_int n) {
	for (sysml_int i = 0; i < n; i++) {
		if (!d[i].len) continue;
		char *p = sysml_alloc((size_t)d[i].len);
		memcpy(p, d[i].p, (size_t)d[i].len);
		free((char *)d[i].p);
		d[i].p = p;
	}
}

/* Reads a String argument written as a KerML string literal. */
static sysml_str sysml_parse_str(const char *s, const char *name) {
	size_t n = strlen(s);
	char *out = sysml_alloc(n ? n : 1);
	sysml_int k = 0;
	bool ok = n >= 2 && s[0] == '"' && s[n - 1] == '"';
	for (size_t i = 0; ok && i < n;) {
		int w;
		sysml_decode((const unsigned char *)s + i, (sysml_int)(n - i), &w);
		if (w == 1 && (unsigned char)s[i] >= 0x80) ok = false;
		i += (size_t)w;
	}
	for (size_t i = 1; ok && i + 1 < n; i++) {
		char c = s[i];
		if (c == '"') { ok = false; break; }
		if (c != '\\') { out[k++] = c; continue; }
		if (++i + 1 >= n) { ok = false; break; }
		switch (s[i]) {
		case 'b': out[k++] = '\b'; break;
		case 't': out[k++] = '\t'; break;
		case 'n': out[k++] = '\n'; break;
		case 'f': out[k++] = '\f'; break;
		case 'r': out[k++] = '\r'; break;
		case '"': case '\'': case '\\': out[k++] = s[i]; break;
		default: ok = false;
		}
	}
	if (!ok) {
		fprintf(stderr, "argument %s: %s is not a String literal\n", name, s);
		exit(2);
	}
	return (sysml_str){k, out};
}
`

// cStrLit is a String literal as a C sysml_str over static bytes.
func cStrLit(s string) string {
	return fmt.Sprintf("((sysml_str){%d, %s})", len(s), cString(s))
}

// cString is a C string literal of s; every byte outside printable ASCII is an
// octal escape, so no source encoding is assumed.
func cString(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		switch c := s[i]; {
		case c == '"' || c == '\\' || c == '?':
			b.WriteByte('\\')
			b.WriteByte(c)
		case c >= 0x20 && c < 0x7F:
			b.WriteByte(c)
		default:
			fmt.Fprintf(&b, "\\%03o", c)
		}
	}
	return "\"" + b.String() + "\""
}
