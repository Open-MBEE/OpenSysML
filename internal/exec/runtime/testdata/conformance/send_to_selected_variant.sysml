// A send addressed by an expression reaches the object a selected variant
// materialized: `engine` is a variation whose `electric` variant `sedan`
// selects, so its value is a variant rather than a plain object — the send
// still addresses the object that variant materializes, whose machine hears
// the ping.
package test {
	private import ControlFunctions::*;

	item def Ping;

	part def Engine {
		exhibit state listening {
			entry; then waiting;

			state waiting;
			accept Ping then heard;
			state heard;
		}
	}

	abstract part family {
		variation part engine : Engine {
			variant part petrol : Engine;
			variant part electric : Engine;
		}
	}

	part sedan :> family {
		part :>> engine = engine::electric;

		perform action run {
			first start;
			then action ping {
				send new Ping() to engine->select { in e; true };
			}
			then done;
		}
	}
}
