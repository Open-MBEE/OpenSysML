// Invalid: 'connector' is a KerML declaration keyword with no SysML production; a SysML package body admits ConnectionUsage, not Connector (KerML.xtext:824 Connector, SysML.xtext:220 PackageMember, :1062 ConnectionUsage).
package X10ConnectorInSysML {
    part def A;
    part a1 : A;
    part b1 : A;
    connector c from a1 to b1;
}
