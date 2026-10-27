// Invalid: a connection definition end must be a usage, inherited ends included (SysML v2.1 Beta 2 8.3.13.3 connectionEnd : Usage redefines associationEnd; pilot validateConnectionDefinitionEndsAreUsages since release 2026-09, so both reject).
package ConnectionDefEndNotUsage {
    private import Links::BinaryLink;
    abstract connection def B :> BinaryLink;
}
