// Invalid: a data type cannot specialize a class (KerML 8.3.3.4.1 validateDataTypeSpecialization; pilot validateDataTypeSpecialization).
package K20DatatypeSpecializesClass {
    class A;
    datatype D specializes A;
}
