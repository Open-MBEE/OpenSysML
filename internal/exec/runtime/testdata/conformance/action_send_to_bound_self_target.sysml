package test {
	private import ScalarValues::*;

	item def Ping;

	part def Host {
		attribute received : Integer = 0;
		item lastPing : Ping;

		perform action notify {
			first start;
			then action send {
				in target : Host[1] = this;
				send new Ping() to target;
			}
			then action wait accept ping : Ping;
			then action mark {
				assign received := received + 1;
				assign lastPing := wait.ping;
			}
			then done;
		}
	}

	part host : Host;
}
