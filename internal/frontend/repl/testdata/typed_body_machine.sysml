// A part exhibiting a machine typed by a definition while stating a body of its
// own, so the machine is addressable by the definition even though the usage,
// not the definition, holds the body being run. The entry action writes two
// slots, so a second performance of the machine would be visible.
package Fleet {
	private import ScalarValues::*;

	state def Mission {
		attribute log : String = "";
		attribute level : Integer = 0;
		entry; then waiting;
		state waiting {
			entry action w {
				assign log := log + "W";
				assign level := level + 10;
			}
		}
	}

	part def Tank {
		exhibit state m : Mission {
			state extra;
		}
	}

	part tank : Tank;
}
