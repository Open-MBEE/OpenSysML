package Tanks {
    private import ScalarValues::Real;

    // `level` states no multiplicity: a parameter is a reference usage, which
    // SysML v2 §7.6.3 leaves out of the implicit [1..1], so the default 0..*
    // (KerML 1.1 §7.4.12) governs it. The solver reads it as one value and says so.
    constraint def NonNegative {
        in level : Real;
        level >= 0.0
    }
    constraint nonNegative : NonNegative;
}
