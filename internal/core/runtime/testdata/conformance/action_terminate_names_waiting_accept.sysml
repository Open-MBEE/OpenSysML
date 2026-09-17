// A terminate naming an accept node parked for a signal ends it waiting: the payload
// never arrives, the node after it runs, and the join meets both branches.
package test {
	private import ScalarValues::*;
	private import ISQ::*;
	private import SI::*;

	attribute def TimeOut;

	action host {
		out attribute steps : Integer = 0;

		first start;
		then fork;
			then listen;
			then quit;
		action listen accept TimeOut;
		then action later assign steps := steps + 10;
		then meet;
		action quit assign steps := steps + 1;
		then action pause accept after 2 [s];
		then action kill { terminate listen; }
		then meet;
		join meet;
		then done;
	}
}
