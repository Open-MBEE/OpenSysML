package Observatory {
	private import ScalarValues::*;

	part def Subsystem {
		attribute mass : Real;
	}
	part def MirrorAssembly :> Subsystem;

	port def OpticalPort;
	port def DataPort;

	part telescope {
		part primaryMirror : MirrorAssembly {
			attribute :>> mass = 10.0;
			port opticalOut : OpticalPort;
		}
		part instrumentCluster : Subsystem {
			attribute :>> mass = 4.5;
			port opticalIn : OpticalPort;
			port dataOut : DataPort;
		}
		part mountControl : Subsystem {
			attribute :>> mass = 15.0;
			port dataIn : DataPort;
		}
		connection opticalPath connect primaryMirror.opticalOut to instrumentCluster.opticalIn;
		connection dataPath connect instrumentCluster.dataOut to mountControl.dataIn;
	}

	requirement massRequirement;
	requirement pointingRequirement {
		requirement slewRequirement;
		requirement trackingRequirement;
	}
	part observatory {
		satisfy massRequirement by telescope;
	}

	package Operations {
		private import ScalarValues::*;

		enum def ObservingMode { imaging; spectroscopy; }

		item def Photon;
		item def Image;

		calc def ExposureTime { in flux : Real; return : Real = 1.0 / flux; }

		action def Observe {
			in mode : ObservingMode;
			out image : Image;
		}
		action observe : Observe {
			first start;
			then action acquire { out image : Image; }
			then action readout { in image : Image; out processed : Image; }
			then done;
			flow acquire.image to readout.image;
		}

		state def TelescopeState;
		state telescopeState : TelescopeState {
			entry; then idle;
			state idle;
			state slewing;
			state tracking;
			transition idle_to_slewing first idle then slewing;
			transition slewing_to_tracking first slewing then tracking;
		}

		constraint def MassBudget { in total : Real; total <= 30.0 }
		requirement def MassLimit {
			subject sub : Subsystem;
			require constraint { sub.mass <= 20.0 }
		}

		interface def OpticalInterface { end source : OpticalPort; end target : OpticalPort; }
		interface opticalLink : OpticalInterface connect telescope.primaryMirror.opticalOut to telescope.instrumentCluster.opticalIn;

		allocation def FunctionToPart { end function : Observe; end part : Subsystem; }
		allocation observeToCluster : FunctionToPart allocate observe to telescope.instrumentCluster;

		verification def ThroughputTest {
			objective { verify massRequirement; }
		}

		use case def ObserveTarget { subject obs : Subsystem; }

		metadata def Heritage { attribute mission : String; }
		@Heritage about telescope { mission = "Hubble"; }

		occurrence def Night;
		individual def HubbleTelescope :> Subsystem;

		view def MassView;
		viewpoint def EngineerViewpoint;
		rendering def TableRendering;
	}

	package Empty;
}
