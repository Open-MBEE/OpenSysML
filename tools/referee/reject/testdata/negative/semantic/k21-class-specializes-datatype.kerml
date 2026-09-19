// Invalid: a class cannot specialize a data type (KerML 8.3.3.4.2 validateClassSpecialization; pilot validateClassSpecialization).
package K21ClassSpecializesDatatype {
    datatype D;
    class C specializes D;
}
