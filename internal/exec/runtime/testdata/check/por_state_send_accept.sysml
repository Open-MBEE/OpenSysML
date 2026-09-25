// An action sends the signal a machine's transition accepts, and a bystander
// branch writes a feature the transition's effect reads: the send pairs with the
// accept and the write is dependent on the effect, so both readings are reached.
package test {
	private import ScalarValues::*;

	item def Go;

	part def Board {
		attribute mark : Integer = 0;
	}

	part board : Board;

	action sender {
		attribute other : Integer = 0;
		first start;
		fork split;
		action go { send Go() to receiver; }
		action bystander { assign board.mark := 10; }
		join sync;
		done;
		succession first start then split;
		succession first split then go;
		succession first split then bystander;
		succession first go then sync;
		succession first bystander then sync;
		succession first sync then done;
	}

	state receiver {
		attribute took : Integer = 0;
		entry; then waiting;
		state waiting;
		state taken;
		transition first waiting accept Go do assign took := board.mark + 1 then taken;
	}
}
