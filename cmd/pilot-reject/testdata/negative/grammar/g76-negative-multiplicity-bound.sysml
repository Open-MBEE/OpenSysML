// Invalid: A multiplicity bound is a literal or a feature reference; a leading '-' is not admitted (KerML.xtext:780 MultiplicityExpressionMember).
package G76NegativeMultiplicityBound {
    part def D;
    part many [-1] : D;
}
