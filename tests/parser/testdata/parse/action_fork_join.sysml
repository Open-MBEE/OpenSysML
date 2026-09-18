action def ParallelWork {
	attribute x : Integer = 0;
	attribute y : Integer = 0;

	first start;
	fork split;
	action left { assign x := 1; }
	action right { assign y := 2; }
	join sync;
	done;

	succession first start then split;
	succession first split then left;
	succession first split then right;
	succession first left then sync;
	succession first right then sync;
	succession first sync then done;
}
