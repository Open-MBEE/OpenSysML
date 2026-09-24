library package AnalysisRecords {
	/* NON-NORMATIVE OpenSysML run-record vocabulary;
	 * not part of the OMG SysML or KerML standard library. */

	private import ScalarValues::*;

	/* RecordedRun is the metadata an analysis run, sweep or sample records
	 * itself with: when it ran, the tool and command that ran it, and the
	 * shape of the run it records. */
	metadata def RecordedRun {
		/* runAt is the timestamp of the run, written in UTC. */
		attribute runAt : String;
		/* tool names the program that made the run. */
		attribute tool : String;
		/* command is the invocation text that made the run. */
		attribute command : String;
		/* kind is the run shape recorded: "run", "trade", "sweep" or "runs". */
		attribute kind : String;
	}

	/* VerdictRecord is what one check of a recorded run decided: an
	 * objective's required conditions or an assertion in the case's body. */
	part def VerdictRecord {
		/* kind is "objective" or "assertion". */
		attribute kind : String;
		/* name names the objective or asserted constraint, or spells an
		 * anonymous assertion's condition. */
		attribute name : String;
		/* status is what the check decided. */
		attribute status : String;
		/* detail is the violated condition of a failed check, or why an
		 * undecided one could not be evaluated. */
		attribute detail : String;
	}

	/* EvaluationRecord is one application of a case's calc to an
	 * alternative, as a trade study's evaluation records it. */
	part def EvaluationRecord {
		/* function is the qualified name of the calc applied. */
		attribute function : String;
		/* alternative is what the calc was applied to. */
		attribute alternative : String;
		/* score is the value the evaluation computed. */
		attribute score : Real;
		/* result is the computed value's text. */
		attribute result : String;
		/* selected marks the evaluation the case picked. */
		attribute selected : Boolean;
		/* tied marks an evaluation computing what the selected did. */
		attribute tied : Boolean;
		/* error is why an evaluation computed nothing. */
		attribute error : String;
	}

	/* AnalysisRun is the record one run of an analysis case makes: the
	 * values its inputs bound and its outputs took, the object it ran on,
	 * and what each of its checks decided and alternatives evaluated. */
	part def AnalysisRun {
		/* caseName is the qualified name of the case that ran. */
		attribute caseName : String;
		/* kind is the run shape this record makes. */
		attribute kind : String;
		/* 'objective' is the status of the run's objective verdict,
		 * "undecided" when the case declares none. */
		attribute 'objective' : String;
		/* iteration is the run's position in a sweep or sample. */
		attribute iteration : Integer;
		/* 'subject' is the object the run was made on. */
		ref part 'subject';
		/* subjectName is the subject object's text. */
		attribute subjectName : String;
		/* verdicts are what the run's checks decided, in order. */
		part verdicts : VerdictRecord[0..*];
		/* evaluations are the calc applications the run made, in order. */
		part evaluations : EvaluationRecord[0..*];
	}
}
