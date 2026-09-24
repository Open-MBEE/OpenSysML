package test {
	private import ScalarValues::*;
	
	action communicator {
		attribute receivedValue : Integer = 0;
		first start;
		
		// Send message
		action sender {
			send 42 to receiver;
		}
		
		// Accept message (binds to msg parameter)
		action receiver accept msg : Integer;
		
		// Process accepted message
		action processor {
			assign receivedValue := msg;
		}
		
		done;
		
		succession first start then sender;
		succession first sender then receiver;
		succession first receiver then processor;
		succession first processor then done;
	}
}
