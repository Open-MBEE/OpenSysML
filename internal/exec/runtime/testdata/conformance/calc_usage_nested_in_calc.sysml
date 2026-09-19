package test {
	private import ScalarValues::*;
	private import RealFunctions::*;

	// A steering law as a multi-output calc, reused by the right-hand side of the
	// equations of motion of a lander: the usage nested in `Rhs` binds its inputs
	// from the parameters of the invocation reading it, so one steering law serves
	// every state the right-hand side is evaluated at.
	calc def Steering {
		in svx : Real;
		in svh : Real;
		out speed = sqrt(svx * svx + svh * svh);
		out ux = 0.0 - svx / speed;
		out uh = 0.0 - svh / speed;
	}

	calc def Rhs {
		in vx : Real;
		in vh : Real;
		in thrust : Real;
		calc st : Steering {
			in svx = vx;
			in svh = vh;
		}
		out dvx = thrust * st.ux;
		out dvh = thrust * st.uh - 1.62;
	}

	// Descending downrange: speed 151.327…, thrust opposing the velocity.
	calc rhsAt : Rhs {
		in vx = 150.0;
		in vh = -20.0;
		in thrust = 4.0;
	}

	// The same right-hand side at another state, with its own run of the nested
	// steering law: straight descent, so all the thrust goes into the vertical.
	calc rhsAtDescent : Rhs {
		in vx = 0.0;
		in vh = -30.0;
		in thrust = 2.0;
	}

	attribute descentDvx = rhsAtDescent.dvx;
	attribute descentDvh = rhsAtDescent.dvh;
}
