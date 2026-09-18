// Invalid: a behavior cannot specialize a structure (KerML 8.3.4.2 validateBehaviorSpecialization; pilot validateBehaviorSpecialization).
package K23BehaviorSpecializesStruct {
    struct S;
    behavior B specializes S;
}
