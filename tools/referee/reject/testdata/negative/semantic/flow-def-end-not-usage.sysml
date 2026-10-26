// Invalid: a flow definition end must be a usage, inherited ends included (SysML v2.1 Beta 2 8.3.16.2 flowEnd : Usage redefines associationEnd; the pilot checks it as validateFlowDefinitionEndsAreUsages since release 2026-09, so both reject).
package FlowDefEndNotUsage {
    private import Transfers::Transfer;
    abstract flow def F :> Transfer;
}
