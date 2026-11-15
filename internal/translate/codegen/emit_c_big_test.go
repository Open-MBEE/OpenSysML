package codegen

import (
	"bufio"
	"bytes"
	"fmt"
	"math"
	"math/big"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// cIntegerOperand draws Integers around the int64 fast path's edges, powers of
// two and their neighbours, and magnitudes of up to several hundred bits.
func cIntegerOperand(r *rand.Rand) *big.Int {
	var b *big.Int
	switch r.Intn(8) {
	case 0:
		b = big.NewInt(r.Int63n(1000) - 500)
	case 1:
		b = big.NewInt(r.Int63())
	case 2:
		b = new(big.Int).Lsh(big.NewInt(1), uint(r.Intn(200)))
		b.Add(b, big.NewInt(r.Int63n(3)-1))
	case 3:
		b = new(big.Int).Add(big.NewInt(math.MaxInt64), big.NewInt(r.Int63n(5)-2))
	case 4:
		// Limbs of all ones and all zeros, where a division's estimate is corrected.
		b = new(big.Int)
		for i, n := 0, 1+r.Intn(8); i < n; i++ {
			b.Lsh(b, 32)
			if r.Intn(2) == 0 {
				b.Or(b, big.NewInt(math.MaxUint32))
			}
		}
		b.Add(b, big.NewInt(r.Int63n(3)))
	default:
		b = new(big.Int).Rand(r, new(big.Int).Lsh(big.NewInt(1), uint(1+r.Intn(400))))
	}
	if r.Intn(2) == 0 {
		b.Neg(b)
	}
	return b
}

// realBits spells a Real by its bits, a zero without its sign.
func realBits(f float64) string {
	if f == 0 {
		f = 0
	}
	return fmt.Sprintf("%x", math.Float64bits(f))
}

// TestCIntegerMatchesBigInt compares the C target's Integer notation, arithmetic,
// ordering and Real conversions with math/big over operands on both sides of int64.
func TestCIntegerMatchesBigInt(t *testing.T) {
	cc := os.Getenv(CCompilerEnvVar)
	if cc == "" {
		cc = "cc"
	}
	if _, err := exec.LookPath(cc); err != nil {
		t.Skip("no C compiler on PATH")
	}
	dir := t.TempDir()
	src := filepath.Join(dir, "integer.c")
	program := cScalarRuntime() + `
static void bits(sysml_real f) { uint64_t u; if (f == 0) f = 0; memcpy(&u, &f, sizeof u); printf(" %" PRIx64, u); }
static void show(sysml_integer a) { putchar(' '); sysml_print_int_value(a); }
int main(void) {
	static char la[4096], lb[4096], lr[64];
	int e;
	while (scanf("%4000s %4000s %60s %d", la, lb, lr, &e) == 4) {
		sysml_mark m = sysml_arena_mark();
		bool ok;
		sysml_integer a = sysml_int_of_digits(la, &ok), b = sysml_int_of_digits(lb, &ok);
		uint64_t u = strtoull(lr, NULL, 16);
		sysml_real r;
		memcpy(&r, &u, sizeof r);
		show(a); show(sysml_add(a, b)); show(sysml_sub(a, b)); show(sysml_mul(a, b)); show(sysml_neg(a));
		show(sysml_ipow(a, SYSML_I(e)));
		printf(" %d %d %d", sysml_icmp(a, b), (int)sysml_ieq(a, b), sysml_cmp_ir(a, r));
		bits(sysml_to_real(a));
		if (sysml_isign(b)) { show(sysml_mod(a, b)); bits(sysml_quot(a, b)); }
		putchar('\n');
		sysml_arena_release(m);
	}
	return 0;
}
`
	if err := os.WriteFile(src, []byte(program), 0o600); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(dir, "integer")
	build := exec.Command(cc, append(append([]string{}, CFlags...), "-o", bin, src, "-lm")...) // #nosec G204 -- test-owned arguments
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("cc: %v\n%s", err, out)
	}

	rng := rand.New(rand.NewSource(20261115))
	var in bytes.Buffer
	var want []string
	for i := 0; i < 20000; i++ {
		a, b := cIntegerOperand(rng), cIntegerOperand(rng)
		if i%7 == 0 {
			// A divisor near the dividend, and one of a single limb.
			b = new(big.Int).Add(a, big.NewInt(rng.Int63n(5)-2))
		} else if i%7 == 1 {
			b = big.NewInt(rng.Int63n(1<<32) + 1)
		}
		// A Real beside b: itself, a neighbour, a fraction, and one far off.
		r, _ := new(big.Float).SetInt(b).Float64()
		switch i % 5 {
		case 1:
			r = math.Nextafter(r, math.Inf(1))
		case 2:
			r = math.Nextafter(r, math.Inf(-1))
		case 3:
			r /= 2
		case 4:
			r *= 3
		}
		e := rng.Intn(12)
		fmt.Fprintf(&in, "%s %s %x %d\n", a, b, math.Float64bits(r), e)

		f, _ := new(big.Float).SetInt(a).Float64()
		line := fmt.Sprintf(" %s %s %s %s %s %s %d %d %d %s", a,
			new(big.Int).Add(a, b), new(big.Int).Sub(a, b), new(big.Int).Mul(a, b), new(big.Int).Neg(a),
			new(big.Int).Exp(a, big.NewInt(int64(e)), nil),
			a.Cmp(b), map[bool]int{true: 1}[a.Cmp(b) == 0], new(big.Float).SetInt(a).Cmp(big.NewFloat(r)), realBits(f))
		if b.Sign() != 0 {
			q, _ := new(big.Rat).SetFrac(a, b).Float64()
			line += fmt.Sprintf(" %s %s", new(big.Int).Rem(a, b), realBits(q))
		}
		want = append(want, line)
	}
	inputs := strings.Split(in.String(), "\n")
	run := exec.Command(bin)
	run.Stdin = &in
	out, err := run.Output()
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	sc := bufio.NewScanner(bytes.NewReader(out))
	sc.Buffer(nil, 1<<20)
	n, bad := 0, 0
	for ; sc.Scan(); n++ {
		if n < len(want) && sc.Text() != want[n] {
			if bad++; bad <= 5 {
				t.Errorf("operands %s:\n got %s\nwant %s", inputs[n], sc.Text(), want[n])
			}
		}
	}
	if n != len(want) {
		t.Fatalf("%d result lines, want %d", n, len(want))
	}
}
