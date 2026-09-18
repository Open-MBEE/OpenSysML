// Invalid: a type cannot union exactly one type (KerML 8.3.3.1 validateTypeOwnedUnioningNotOne; pilot validateOwnedUnioningNotOne).
package K04UnionsOneType {
    class A;
    class C unions A;
}
