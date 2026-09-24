// Invalid: AssignmentNode requires an expression after := (SysML.xtext:1535 AssignmentNode, :1539 AssignmentNodeDeclaration).
package G49AssignWithoutValue {
    action def A {
        attribute a;
        assign a := ;
    }
}
