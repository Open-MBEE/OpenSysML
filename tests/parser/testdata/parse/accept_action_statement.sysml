// An accept node is an action node (SysML.xtext ActionNode), so it stands wherever
// a statement does; its action name and received payload name stay distinct.
package Accepting {
	attribute def EngineStart;
	attribute def EngineOff;
	port def Signals;
	action def Drive {
		port signals : Signals;

		loop {
			accept engineStart : EngineStart;
			then action engineStopped accept engineOff : EngineOff;
			then accept restart : EngineStart;
			then action watched accept engineStart : EngineStart via signals {
				doc /* an accept node carries a body */
			}
		}
	}
}
