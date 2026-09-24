package test {
	private import ScalarValues::*;

	// A terminate action usage's body may declare the node's pins, which a flow into it
	// targets as into any node: the flow into `stop.reason` is bound as the token reaches
	// stop, and stop then ends the action with x assigned and y not.
	action stopper {
		out attribute x : Integer = 0;
		out attribute y : Integer = 0;

		first start;
		then action a1 { out code : Integer; assign x := 1; assign code := 7; }
		then stop;
		action stop terminate { in reason : Integer; }
		then action a2 { assign y := 2; }
		then done;

		flow a1.code to stop.reason;
	}
}
