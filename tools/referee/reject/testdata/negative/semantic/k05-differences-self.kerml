// Invalid: a type cannot difference with itself (KerML 8.3.3.1 validateTypeDifferencingTypesNotSelf; pilot validateTypeDifferencingTypesNotSelf).
package K05DifferencesSelf {
    class A;
    class C differences C, A;
}
