package Test {
	state def TrafficLight parallel {
		// Pedestrian region - controls walk/don't walk signals
		state pedestrian {
			entry; then start;
			state start;
			state Walk;
			state DontWalk;
			
			succession first start then Walk;
			transition first Walk when pedestrianTimer then DontWalk;
			transition first DontWalk when pedestrianTimer then Walk;
		}
		
		// Vehicle region - controls traffic light colors
		state vehicle {
			entry; then start;
			state start;
			state Green;
			state Yellow;
			state Red;
			
			succession first start then Green;
			transition first Green when vehicleTimer then Yellow;
			transition first Yellow when vehicleTimer then Red;
			transition first Red when vehicleTimer then Green;
		}
	}
}
