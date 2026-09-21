// A node joining the performance an object already runs of its action streams the
// outputs written while it performs it, as a node beginning a performance does: the
// consumer begun afterwards reads the pin's value, the last of them.
package test {
	private import ScalarValues::*;

	action def Count {
		out value : Integer;
		attribute i : Integer = 0;

		action emit { assign i := i + 1; assign value := i; }
		decide again;

		succession first start then emit;
		succession first emit then again;
		succession first again if i < 3 then emit;
		succession first again if i >= 3 then done;
	}

	part def Host {
		perform action go : Count;
	}

	action def Cfg {
		part target : Host;
		out total : Integer = 0;

		perform action run ::> target.go;
		action consumer {
			in value : Integer;
			assign total := total + value;
		}

		succession first start then run;
		succession first run then consumer;
		succession first consumer then done;

		flow run.value to consumer.value;
	}
}
