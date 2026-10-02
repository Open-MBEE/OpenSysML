//! Exact KerML Rationals, as the wire's `rational_value` carries them.

use std::cmp::Ordering;
use std::fmt;

use crate::error::Error;

/// An exact Rational no `f64` holds, such as `1/3` or `1/10`: a numerator over a
/// positive denominator in lowest terms, each held as its decimal digits. One an
/// `f64` holds exactly is always a `Real`, never this, so two numbers are equal
/// exactly when their arms and terms are.
#[derive(Clone, Debug, PartialEq, Eq, Hash)]
pub struct Rational {
    numerator: String,
    denominator: String,
}

impl Rational {
    /// The rational `numerator/denominator` spells: decimal digits with no leading
    /// zero, a `-` only on the numerator, in lowest terms, and not a number an
    /// `f64` holds, which a `Real` carries.
    pub fn parse(numerator: &str, denominator: &str) -> Result<Self, Error> {
        let malformed = |why: &str| {
            Error::Decode(format!(
                "not a canonical rational: {numerator}/{denominator} {why}"
            ))
        };
        let negative = numerator.starts_with('-');
        let (Some(n), Some(d)) = (
            Natural::parse(numerator.strip_prefix('-').unwrap_or(numerator)),
            Natural::parse(denominator),
        ) else {
            return Err(malformed("is not decimal terms"));
        };
        if d.is_zero() || (negative && n.is_zero()) {
            return Err(malformed("has no value"));
        }
        if !Natural::gcd(&n, &d).is_one() {
            return Err(malformed("is not in lowest terms"));
        }
        if binary64(&n, &d) {
            return Err(malformed("is an f64, which real_value carries"));
        }
        Ok(Self {
            numerator: numerator.to_owned(),
            denominator: denominator.to_owned(),
        })
    }

    /// The numerator's decimal digits, `-` signed.
    pub fn numerator(&self) -> &str {
        &self.numerator
    }

    /// The denominator's decimal digits, positive.
    pub fn denominator(&self) -> &str {
        &self.denominator
    }

    /// The nearest `f64`, ties to even; infinite only beyond the finite range.
    pub fn to_f64(&self) -> f64 {
        let digits = self.numerator.strip_prefix('-').unwrap_or(&self.numerator);
        let magnitude = match (Natural::parse(digits), Natural::parse(&self.denominator)) {
            (Some(n), Some(d)) => nearest(&n, &d),
            _ => f64::NAN,
        };
        if self.numerator.starts_with('-') {
            -magnitude
        } else {
            magnitude
        }
    }
}

impl fmt::Display for Rational {
    /// Writes `numerator/denominator`, as the evaluator spells a non-terminating Rational.
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "{}/{}", self.numerator, self.denominator)
    }
}

// Whether the positive n/d, in lowest terms, is a finite f64: a dyadic fraction of
// at most 53 significant bits whose least bit is no finer than 2^-1074.
fn binary64(n: &Natural, d: &Natural) -> bool {
    let shift = d.trailing_zeros();
    if d.bit_len() != shift + 1 {
        return false;
    }
    let zeros = n.trailing_zeros();
    let significant = n.bit_len() - zeros.min(n.bit_len());
    let least = zeros as i64 - shift as i64;
    let top = n.bit_len() as i64 - 1 - shift as i64;
    n.is_zero() || (significant <= 53 && least >= -1074 && top <= 1023)
}

// n/d, both positive, rounded once to the nearest f64.
fn nearest(n: &Natural, d: &Natural) -> f64 {
    if n.is_zero() {
        return 0.0;
    }
    // A quotient of 63 or 64 bits and a sticky remainder hold enough to round once.
    let scale = 63 - (n.bit_len() as i64 - d.bit_len() as i64);
    let (q, exact) = if scale >= 0 {
        n.shl(scale as usize).div_u64(d)
    } else {
        n.div_u64(&d.shl((-scale) as usize))
    };
    let q = u128::from(q);
    let width = 128 - q.leading_zeros() as i64;
    let top = width - 1 - scale;
    if top > 1023 {
        return f64::INFINITY;
    }
    let precision = if top >= -1022 { 53 } else { 53 - (-1022 - top) };
    let drop = width - precision;
    if drop > 127 {
        return 0.0;
    }
    let drop = drop as u32;
    let mut kept = q >> drop;
    let rest = q & ((1u128 << drop) - 1);
    let half = 1u128 << (drop - 1);
    if rest > half || (rest == half && (!exact || kept & 1 == 1)) {
        kept += 1;
    }
    let least = drop as i32 - scale as i32;
    let mut x = kept as f64;
    if least < -1022 {
        x *= pow2(least + 1022);
        x * pow2(-1022)
    } else {
        x * pow2(least)
    }
}

// 2^e for a normal exponent e.
fn pow2(e: i32) -> f64 {
    f64::from_bits(((e + 1023) as u64) << 52)
}

// An unsigned integer as base-2^32 limbs, least significant first, with no high zero limb.
#[derive(Clone, Debug, PartialEq, Eq)]
struct Natural(Vec<u32>);

impl Natural {
    fn parse(digits: &str) -> Option<Self> {
        if digits.is_empty()
            || !digits.bytes().all(|b| b.is_ascii_digit())
            || (digits.len() > 1 && digits.starts_with('0'))
        {
            return None;
        }
        let mut limbs: Vec<u32> = Vec::new();
        for b in digits.bytes() {
            let mut carry = u64::from(b - b'0');
            for limb in &mut limbs {
                let wide = u64::from(*limb) * 10 + carry;
                *limb = wide as u32;
                carry = wide >> 32;
            }
            if carry != 0 {
                limbs.push(carry as u32);
            }
        }
        Some(Self(limbs))
    }

    fn is_zero(&self) -> bool {
        self.0.is_empty()
    }

    fn is_one(&self) -> bool {
        self.0 == [1]
    }

    fn bit_len(&self) -> usize {
        self.0
            .last()
            .map_or(0, |top| self.0.len() * 32 - top.leading_zeros() as usize)
    }

    fn trailing_zeros(&self) -> usize {
        self.0
            .iter()
            .position(|limb| *limb != 0)
            .map_or(0, |i| i * 32 + self.0[i].trailing_zeros() as usize)
    }

    fn trimmed(mut limbs: Vec<u32>) -> Self {
        while limbs.last() == Some(&0) {
            limbs.pop();
        }
        Self(limbs)
    }

    fn shl(&self, bits: usize) -> Self {
        let (words, bits) = (bits / 32, bits % 32);
        let mut limbs = vec![0u32; words];
        let mut carry = 0u32;
        for limb in &self.0 {
            limbs.push(if bits == 0 {
                *limb
            } else {
                (limb << bits) | carry
            });
            carry = if bits == 0 { 0 } else { limb >> (32 - bits) };
        }
        limbs.push(carry);
        Self::trimmed(limbs)
    }

    fn shr(&self, bits: usize) -> Self {
        let (words, bits) = (bits / 32, bits % 32);
        if words >= self.0.len() {
            return Self(Vec::new());
        }
        let rest = &self.0[words..];
        let limbs = (0..rest.len())
            .map(|i| {
                let high = rest.get(i + 1).copied().unwrap_or(0);
                if bits == 0 {
                    rest[i]
                } else {
                    (rest[i] >> bits) | (high << (32 - bits))
                }
            })
            .collect();
        Self::trimmed(limbs)
    }

    fn cmp(&self, other: &Self) -> Ordering {
        self.0
            .len()
            .cmp(&other.0.len())
            .then_with(|| self.0.iter().rev().cmp(other.0.iter().rev()))
    }

    // self - other, for self >= other.
    fn sub(&self, other: &Self) -> Self {
        let mut borrow = 0i64;
        let limbs = self
            .0
            .iter()
            .enumerate()
            .map(|(i, limb)| {
                let mut wide =
                    i64::from(*limb) - i64::from(other.0.get(i).copied().unwrap_or(0)) - borrow;
                borrow = i64::from(wide < 0);
                if wide < 0 {
                    wide += 1 << 32;
                }
                wide as u32
            })
            .collect();
        Self::trimmed(limbs)
    }

    // The quotient self/d, known to fit 64 bits, and whether it is exact.
    fn div_u64(&self, d: &Self) -> (u64, bool) {
        let mut rest = self.clone();
        let mut q = 0u64;
        for bit in (0..64).rev() {
            let shifted = d.shl(bit);
            if rest.cmp(&shifted) != Ordering::Less {
                rest = rest.sub(&shifted);
                q |= 1 << bit;
            }
        }
        (q, rest.is_zero())
    }

    // Stein's binary gcd, which needs only shifts and subtraction.
    fn gcd(a: &Self, b: &Self) -> Self {
        if a.is_zero() {
            return b.clone();
        }
        if b.is_zero() {
            return a.clone();
        }
        let common = a.trailing_zeros().min(b.trailing_zeros());
        let mut a = a.shr(a.trailing_zeros());
        let mut b = b.shr(b.trailing_zeros());
        while !b.is_zero() {
            if a.cmp(&b) == Ordering::Greater {
                std::mem::swap(&mut a, &mut b);
            }
            b = b.sub(&a);
            b = b.shr(b.trailing_zeros());
        }
        a.shl(common)
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn q(n: &str, d: &str) -> Rational {
        Rational::parse(n, d).unwrap()
    }

    #[test]
    fn a_rational_rounds_once_to_the_nearest_f64() {
        assert_eq!(q("1", "3").to_f64(), 1.0 / 3.0);
        assert_eq!(q("-1", "10").to_f64(), -0.1);
        assert_eq!(q("3", "10").to_f64(), 0.3);
        assert_eq!(q("2", "3").to_f64(), 2.0 / 3.0);
        let googol = format!("1{}", "0".repeat(100));
        assert_eq!(q(&googol, "3").to_f64(), 1e100 / 3.0);
        assert_eq!(q("1", &googol).to_f64(), 1e-100);
        assert_eq!(
            q(&format!("1{}", "0".repeat(400)), "3").to_f64(),
            f64::INFINITY
        );
        // Below the least subnormal's half, and just above it.
        let tiny = format!("1{}", "0".repeat(330));
        assert_eq!(q("1", &tiny).to_f64(), 0.0);
        assert_eq!(
            q("3", &format!("1{}", "0".repeat(324))).to_f64(),
            f64::from_bits(1)
        );
        // 2^53 + 1 is halfway between two f64s and rounds to even.
        assert_eq!(q("9007199254740993", "1").to_f64(), 9007199254740992.0);
        assert_eq!(q("9007199254740995", "1").to_f64(), 9007199254740996.0);
    }

    #[test]
    fn only_a_canonical_rational_no_f64_holds_parses() {
        for (n, d) in [
            ("2", "6"),
            ("1", "0"),
            ("1", "-3"),
            ("-0", "3"),
            ("01", "3"),
            ("+1", "3"),
            ("1.5", "7"),
            ("", "3"),
            ("1", "2"),
            ("3", "1"),
            ("0", "1"),
            ("-5", "4"),
            ("1", "4503599627370496"),
        ] {
            assert!(Rational::parse(n, d).is_err(), "{n}/{d}");
        }
        let r = q("-1", "3");
        assert_eq!((r.numerator(), r.denominator()), ("-1", "3"));
        assert_eq!(r.to_string(), "-1/3");
        assert!(Rational::parse("9007199254740993", "1").is_ok());
        assert!(Rational::parse("9007199254740993", "2").is_ok());
        assert!(Rational::parse(&format!("1{}", "0".repeat(400)), "1").is_ok());
    }

    #[test]
    fn natural_arithmetic_is_exact() {
        let n = Natural::parse("340282366920938463463374607431768211456").unwrap();
        assert_eq!(n.bit_len(), 129);
        assert_eq!(n.trailing_zeros(), 128);
        let a = Natural::parse("123456789012345678901234567890").unwrap();
        let b = Natural::parse("987654321098765432109876543210").unwrap();
        assert_eq!(
            Natural::gcd(&a, &b),
            Natural::parse("9000000000900000000090").unwrap()
        );
        assert_eq!(a.shl(37).shr(37), a);
    }
}
