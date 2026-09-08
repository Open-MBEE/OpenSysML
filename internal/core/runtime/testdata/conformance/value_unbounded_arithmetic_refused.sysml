// `*` is no number, so arithmetic over it is refused with an error naming the
// operation rather than answered with a finite result.
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
        attribute positive = +*;
    }
}
