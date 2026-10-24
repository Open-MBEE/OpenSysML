// Invalid: a flow definition end must be a usage, inherited ends included (SysML v2.1 Beta 2 8.3.16.2 flowEnd : Usage redefines associationEnd; the pilot's master branch adds the check as ST6RI-974, after the 2026-08 pin, so this case lands in ours-only-rejects).
package FlowDefEndNotUsage {
    private import Transfers::Transfer;
    abstract flow def F :> Transfer;
}
