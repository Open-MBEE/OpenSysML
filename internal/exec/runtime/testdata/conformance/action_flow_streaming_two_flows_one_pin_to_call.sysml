package test {
	private import ScalarValues::*;
	private import SequenceFunctions::*;

	// Two flows out of one source pin each carry the pin's value to the target, so a
	// target performed once per enabling succession is bound on both performances,
	// the second write of the same pin replacing only what its own flow left waiting.
	action def Copier {
		in input : Integer;
		out copied : Integer;
		action read { out v : Integer = input; }
		action write { in v : Integer[0..1]; assign copied := v; }
		flow read.v to write.v;
		succession first start then read;
		succession first read then write;
		succession first write then done;
	}

	action stream {
		attribute total : Integer[0..*] nonunique = ();

		action producer { out result : Integer = 7; }
		fork twice;
		merge again;
		action copier : Copier;
		action gather { in v : Integer[0..*] nonunique; assign total := (total, v); }

		flow producer.result to copier.input;
		flow producer.result to copier.input;
		flow copier.copied to gather.v;

		succession first start then producer;
		succession first producer then twice;
		succession first twice then again;
		succession first twice then again;
		succession first again then copier;
		succession first copier then gather;
	}
}
