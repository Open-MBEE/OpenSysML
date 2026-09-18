// Invalid: a type cannot union with itself (KerML 8.3.3.1 validateTypeUnioningTypesNotSelf; pilot validateTypeUnioningTypesNotSelf).
package K07UnionsSelf {
    class A;
    class C unions C, A;
}
