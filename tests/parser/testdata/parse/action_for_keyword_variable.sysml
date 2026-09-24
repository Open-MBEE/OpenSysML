// A for loop's variable is a usage declaration, so a keyword may name it
// (SysML.xtext ForVariableDeclaration).
package ForKeywordVariables {
    part def Collection;
    action def Iterate {
        in items : Collection;
        for step in items {
            action inner;
        }
        for item in items {
            action inner2;
        }
        for state in items {
            action inner3;
        }
        for part in items {
            action inner4;
        }
        for end in items {
            action inner5;
        }
        for i in items {
            action inner6;
        }
    }
}
