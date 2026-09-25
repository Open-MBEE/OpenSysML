// Invalid: a type cannot intersect exactly one type (KerML 8.3.3.1 validateTypeOwnedIntersectingNotOne; pilot validateOwnedIntersectingNotOne).
package K03IntersectsOneType {
    class A;
    class C intersects A;
}
