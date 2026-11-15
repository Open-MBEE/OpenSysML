// Real is the extended reals (KerML 1.1 9.3.2.2.9): `*` is the infinity above
// every number (8.3.4.8.11) and its negation `-*` the one below. Arithmetic over
// them answers the limit where one exists and reports an indeterminate form with
// a typed error, as a division by zero is; the function forms and the
// aggregations answer the same, and the range operator still wants finite bounds.
package test {
    private import ScalarValues::*;

    part def Arithmetic {
        attribute sum = * + 1;
        attribute addedTo = 1 + *;
        attribute difference = * - 1;
        attribute below = 1 - *;
        attribute product = 2 * *;
        attribute negativeProduct = -2 * *;
        attribute quotient = * / 2;
        attribute vanishing : Real = 2.0 / *;
        attribute power = * ** 2;
        attribute oddPower = (-*) ** 3;
        attribute decay = 2 ** -*;
        attribute negated = -*;
        attribute restored = -(-*);
        attribute declared : Real = -*;
        attribute ordered : Boolean = declared < -1.0e308 and declared < *;
        attribute functionForm = NaturalFunctions::'+'(*, 1);
        attribute naturalQuotient = NaturalFunctions::'/'(*, 2);
        attribute total = IntegerFunctions::sum((1, *));
        attribute least = RealFunctions::min(-*, 3.0);
        attribute magnitude = RealFunctions::abs(-*);
        attribute text = RealFunctions::ToString(-*);

        attribute opposite = * - *;
        attribute zeroTimes = 0 * *;
        attribute ratio = * / *;
        attribute remainder = * % 2;
        attribute zerothPower = * ** 0;
        attribute zeroDivisor = -* / 0;
        attribute range = 1..*;
        attribute noNatural = NaturalFunctions::'+'(-*, 1);
    }
}
