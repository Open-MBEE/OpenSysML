// The shape a %features listing used to truncate: a context of twelve parts,
// each exhibiting machines whose counters sit two levels below the context.
package Plant {
	part def Counters {
		attribute started = 0;
		attribute completed = 0;
		attribute faulted = 0;
		attribute retried = 0;
		attribute blocked = 0;
		attribute idleTicks = 0;
		attribute busyTicks = 0;
		attribute scrapped = 0;
	}

	part def Machine {
		attribute idle = true;
		part infeed : Counters;
		part outfeed : Counters;
	}

	part def Station {
		attribute name = "station";
		part machine : Machine;
	}

	part def Context {
		part stations : Station[12];
	}
}
