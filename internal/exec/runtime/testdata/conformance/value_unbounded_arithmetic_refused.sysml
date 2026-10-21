// The Kernel Function Library defines no arithmetic result over `*` (KerML 1.0
// §9.3.2 extends Natural with it, §9.4 says nothing of it), so arithmetic over
// it is refused with an error naming the operation rather than answered with a
// finite result, as a division by zero is; the function forms and the range
// operator refuse it the same way.
package test {
    private import ScalarValues::*;

    part def Arithmetic {
        attribute sum = * + 1;
        attribute addedTo = 1 + *;
        attribute difference = * - 1;
        attribute product = 2 * *;
        attribute quotient = * / 2;
        attribute remainder = * % 2;
        attribute power = * ** 2;
        attribute negated = -*;
        attribute functionForm = NaturalFunctions::'+'(*, 1);
        attribute naturalQuotient = NaturalFunctions::'/'(*, 2);
        attribute range = 1..*;
        attribute total = IntegerFunctions::sum((1, *));
    }
}
