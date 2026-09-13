// Send/accept pairing: two accepts park in parallel branches and a sender posts
// to each; a bystander branch writes a feature both branches read afterwards.
package test {
	private import ScalarValues::*;

	action communicator {
		attribute number : Integer = 0;
		attribute message : String = "none";
		attribute mark : Integer = 0;
		first start;
		fork split;
		action listenNumber accept n : Integer;
		action tookNumber { assign number := n + mark; }
		action listenText accept t : String;
		action tookText { assign message := t; }
		action sender {
			send "go" to listenText;
			send 5 to listenNumber;
		}
		action bystander { assign mark := 10; }
		join sync;
		done;
		succession first start then split;
		succession first split then listenNumber;
		succession first split then listenText;
		succession first split then sender;
		succession first split then bystander;
		succession first listenNumber then tookNumber;
		succession first listenText then tookText;
		succession first tookNumber then sync;
		succession first tookText then sync;
		succession first sender then sync;
		succession first bystander then sync;
		succession first sync then done;
	}
}
