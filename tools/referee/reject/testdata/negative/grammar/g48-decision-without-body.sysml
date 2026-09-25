// Invalid: DecisionNode requires an action body after its declaration (SysML.xtext:1670 DecisionNode).
package G48DecisionWithoutBody {
    action def A {
        decide d
    }
}
