// A RightTriangle's base and e2 read the enclosing triangle's length and width
// through `Triangle::length` / `Triangle::width`; the library states the
// hypotenuse as `length^2 + width^2`, a squared quantity that cannot be written
// to a LengthValue, which is reported as the typed mismatch it is.
package test {
    private import ShapeItems::*;
    private import SI::*;

    part def Gusset {
        part tri : RightTriangle {
            :>> length = 4 [m];
            :>> width = 3 [m];
        }
    }
}
