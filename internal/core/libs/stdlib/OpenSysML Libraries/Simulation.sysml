library package Simulation {
	doc
	/*
	 * NON-NORMATIVE. This package is an OpenSysML extension, not part of the OMG SysML
	 * standard library: it declares the metadata by which a model records how a
	 * behavior is meant to be run — how many runs, how random draws resolve, and the
	 * clock the runs are read on. The vendored OMG files are unmodified. A tool that
	 * does not know the extension reads the metadata as any metadata usage.
	 *
	 * A SysML v1 model migrated from a tool with its own simulation profile carries
	 * one Configuration per run configuration of that tool; the migration reference
	 * documents which tag of the tool each attribute records.
	 */

	private import ScalarValues::*;

	enum def DrawPolicy {
		doc
		/*
		 * How a run resolves a RandomFunctions draw: random takes a seeded draw,
		 * min and max the lower and upper bound of the distribution, and average
		 * its midpoint. A run is given the policy as -draws on the command line or
		 * %draws in the REPL; random is the default.
		 */
		random;
		min;
		max;
		average;
	}

	metadata def Configuration {
		doc
		/*
		 * The run configuration of the annotated action or analysis definition:
		 *
		 *   action def Nominal {
		 *       @Configuration { runs = 100; draws = DrawPolicy::max; }
		 *       part target : Vehicle;
		 *       perform action run ::> target.drive;
		 *   }
		 *
		 * runs is the number of runs the configuration was made for, to pass as
		 * -runs; draws is the policy to pass as -draws. timeVariable, startTime,
		 * stepSize and timeUnit describe the clock the configuring tool ran on;
		 * OpenSysML's clock is the run's own, so they are recorded, not applied.
		 * parallelForks records whether the tool ran a fork's branches in parallel.
		 * Every attribute is optional: an absent one was not stated.
		 */
		attribute runs : Natural[0..1];
		attribute draws : DrawPolicy[0..1];
		attribute timeVariable : String[0..1];
		attribute startTime : Real[0..1];
		attribute stepSize : Real[0..1];
		attribute timeUnit : String[0..1];
		attribute parallelForks : Boolean[0..1];
	}
}
