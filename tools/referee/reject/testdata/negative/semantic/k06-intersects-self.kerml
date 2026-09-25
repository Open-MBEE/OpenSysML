// Invalid: a type cannot intersect with itself (KerML 8.3.3.1 validateTypeIntersectingTypesNotSelf; pilot validateTypeIntersectingTypesNotSelf).
package K06IntersectsSelf {
    class A;
    class C intersects C, A;
}
