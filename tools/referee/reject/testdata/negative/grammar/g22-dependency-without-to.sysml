// Invalid: Dependency requires a to clause naming its supplier (SysML.xtext:55 Dependency).
package G22DependencyWithoutTo {
    part def A;
    part def B;
    dependency d from A;
}
