// The `;` that terminates a transition also terminates the statement written as
// its effect (SysML.xtext EffectBehaviorUsage carries no `;` of its own), for
// every statement an effect may be written as.
package TransitionEffects {
	action def Bump;
	attribute def Alert;
	part operator;

	state def S {
		attribute c : ScalarValues::Integer := 0;
		state idle;
		state alerting;

		transition first idle do perform Bump then alerting;
		transition first alerting do assign c := c + 1 then idle;
		transition first idle do { assign c := c + 1; } then alerting;
		transition first alerting do action nudge : Bump then idle;
		transition first idle do send new Alert() to operator then alerting;
	}
}
