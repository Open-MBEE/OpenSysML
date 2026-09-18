state def VehicleLifecycle {
	entry action initSystem;
	do action monitorHealth;
	exit action shutdown;
	
	state Off {
		entry action logOff;
	}
	
	state Idle {
		entry action logIdle;
		exit action logIdleExit;
	}
	
	state Active {
		entry action startEngine;
		do action runEngine;
		exit action stopEngine;
		
		state Accelerating;
		state Cruising;
		state Braking;
		
		transition first Accelerating when stable then Cruising;
	}
	
	transition first Off when powerOn then Idle;
	transition first Idle when start then Active;
	transition first Active when shutdown then Off;
}
