// Invalid: An unnamed assertion derives no member name from the feature it references, so a qualified reference through its owner still reaches the referenced non-constraint (KerML 7.3.4.5 Feature names; SysML 7.19 Constraints; pilot validateAssertConstraintUsageReference).
package S93AssertThroughOwnerOfUnnamedAssertion {
    part def H {
        part q;
        constraint c;
    }
    part h : H {
        assert q;
    }
    part ctx {
        assert h.q;
        assert h.c;
    }
}
