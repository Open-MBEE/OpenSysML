package BodyLocals {
    private import ScalarValues::*;

    enum def Level :> Integer { low = 1; high = 3; }

    // A body-local declaration answers to its declared type, multiplicity and
    // uniqueness when its initial value is bound (KerML 1.0 §7.3.4).
    calc def BodyOnly { in n : Integer; attribute l : Level = n; return : Integer = l + 0; }
    calc def TwoOf { in n : Integer; attribute xs : Integer[2] = (n, n + 1, n + 2); return : Integer = SequenceFunctions::size(xs); }
    calc def Distinct { in n : Integer; attribute xs : Integer[*] = (n, n + 1, n); return : Integer = SequenceFunctions::size(xs); }

    attribute acceptedLevel : Integer = BodyOnly(3);
    attribute refusedLevel : Integer = BodyOnly(2);
    attribute refusedCount : Integer = TwoOf(1);
    attribute refusedRepeat : Integer = Distinct(1);
}
