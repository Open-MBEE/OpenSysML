// Invalid: A satisfaction subject is a feature reference or feature chain, not an expression (SysML.xtext:2139 SatisfactionReferenceExpression).
package G86SatisfyByExpression {
    requirement def R { subject s[1]; }
    part def Sub;
    part p {
        part sub : Sub[2];
        satisfy requirement sr : R by sub#(1);
    }
}
