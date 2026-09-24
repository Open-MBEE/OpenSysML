package Garage {
	private import DocumentQueries::*;
	private import KerML::Root::Element;
	private import ScalarValues::*;

	part def Wheel {
		attribute pressure : Real default = 32.0;
		assert constraint pressureOk { pressure >= 30.0 }
	}
	part def Engine {
		attribute power : Real = 300.0;
		assert constraint powerLow { power < 200.0 }
	}
	part def Car {
		attribute mass : Real = 1500.0;
		attribute capacity : Real;
		part engine : Engine;
		part wheels : Wheel[2] {
			attribute :>> pressure = 20.0;
		}
		assert constraint massOk { mass < 2000.0 }
		assert constraint fits { mass <= capacity }
	}
	requirement def PowerReq {
		subject e : Engine;
		require constraint { e.power > 100.0 }
	}
	part car : Car;
	requirement strongEngine : PowerReq;
	satisfy strongEngine by car.engine;
	verification def PowerCheck {
		subject e : Engine;
		objective { verify strongEngine; }
		VerificationCases::PassIf(e.power > 100.0)
	}
	verification checkEngine : PowerCheck { subject e = car.engine; }

	calc def Checks :> Query {
		in root : Element;
		Project(
			source = Verdicts(source = root),
			properties = ("path", "name", "verdict", "reason")
		)
	}
	calc def Failing :> Query {
		in root : Element;
		WhereFeature(source = Verdicts(source = root), 'feature' = "verdict", operator = "!=", value = "holds")
	}
}
