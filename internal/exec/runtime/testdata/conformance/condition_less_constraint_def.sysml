// A constraint is a BooleanEvaluation whose result only its body expression binds
// (KerML 1.1 §7.4.8.2, SysML v2.1 §7.20.2): a definition declaring no condition
// binds no result, so evaluating it reports no value, not a verdict.
package test {
	constraint def Empty;
}
