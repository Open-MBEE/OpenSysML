// Invalid: A requirement verification must be in the objective of a verification case (SysML 7.21 validateRequirementVerificationMembershipOwningType; pilot Verification_invalid.sysml.xt:30).
package P29VerifyOutsideObjective {
    requirement def R;
    requirement r : R;
    requirement def R2 {
        verify r;
    }
}
