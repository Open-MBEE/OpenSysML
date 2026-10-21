// What the ordering of `*` decides is answered in the function notation as in
// the operator notation (KerML 1.0 §8.2.5.8.4: a Positive greater than all the
// integers): comparisons and equality, max, min and abs in every numeric
// library package, ToString, and unary `+`.
package test {
    private import ScalarValues::*;
    private import NaturalFunctions::*;

    part def Ordering {
        attribute positive = +*;
        attribute greater = '>'(*, 1);
        attribute less = IntegerFunctions::'<'(*, -1);
        attribute atLeast = RealFunctions::'>='(*, 1.5);
        attribute equalsOne = '=='(*, 1);
        attribute equalsItself = IntegerFunctions::'=='(*, *);
        attribute largest = max(*, 1);
        attribute least = min(*, 1);
        attribute largestReal = RealFunctions::max(2.5, *);
        attribute leastReal = RealFunctions::min(*, 2);
        attribute leastRational = RationalFunctions::min(*, 2.5);
        attribute leastGeneric = DataFunctions::min(3, *);
        attribute bothUnbounded = IntegerFunctions::min(*, *);
        attribute magnitude = IntegerFunctions::abs(*);
        attribute realMagnitude = RealFunctions::abs(*);
        attribute text = ToString(*);
        attribute realText = RealFunctions::ToString(*);
    }
}
