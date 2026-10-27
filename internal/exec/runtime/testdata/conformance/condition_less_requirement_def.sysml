// A requirement definition with no require or assume constraint holds vacuously:
// its result is allTrue(assumptions()) implies allTrue(constraints()) (SysML v2
// §9.2.14 Requirements::RequirementConstraintCheck), true over no constraint —
// unlike a constraint declaring no condition, which binds no result at all.
package test {
	requirement def Bare;
}
