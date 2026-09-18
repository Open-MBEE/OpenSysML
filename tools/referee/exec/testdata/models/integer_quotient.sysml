package Quotient {
    private import ScalarValues::*;

    // IntegerFunctions::'/' returns a Rational, so the checker refuses to bind
    // the quotient to an Integer result whatever the operands (KerML 9.4.11.1).
    calc def IntDiv { return : Integer = 7 / 2; }
}
