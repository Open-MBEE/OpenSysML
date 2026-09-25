// Invalid: ForLoopNode requires an in clause naming the sequence (SysML.xtext:1624 ForLoopNode).
package G34ForWithoutIn {
    action def A {
        for x { }
    }
}
