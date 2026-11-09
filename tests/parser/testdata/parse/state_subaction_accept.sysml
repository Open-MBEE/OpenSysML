package StateSubactionAccept {

	attribute def Sig;
	attribute def Stop;
	port def SigPort;

	state def Monitor {
		port p : SigPort;
		attribute count : ScalarValues::Integer;

		state idle {
			entry accept a : Sig via p;
			do accept Sig;
			exit accept :> stop { assign count := count + 1; }
		}

		state busy {
			entry accept a : Sig;
			then idle;
		}

		state done {
			do accept after 2 [SI::s];
			accept Stop then idle;
		}

		event occurrence stop : Stop;
	}
}
