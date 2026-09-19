// Invalid: Owned usages of a port definition (other than ports) must be referential (SysML 7.13 validatePortDefinitionOwnedUsagesNotComposite; pilot PortUsage_Invalid.sysml.xt:33).
package P26PortDefNonReferentialUsage {
    part def B;
    port def pd1 {
        part b2 : B;
    }
}
