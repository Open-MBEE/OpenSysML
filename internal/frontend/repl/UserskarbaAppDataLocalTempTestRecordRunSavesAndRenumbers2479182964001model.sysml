package Demo {
    private import ScalarValues::*;
    part def Probe {
        attribute t : Real = 3.0;
        action tick { first start; then assign t := t + 1.0; then done; }
    }
    part probe : Probe;
    analysis def Check {
        subject s : Probe;
        in gain : Real[1];
        out x : Real[1] = s.t + gain;
    }
    analysis def Bound {
        out y : Real[1] = 1.0 + 2.0;
    }
    analysis timed : Check { subject s = probe; in gain = 2.0; }
    calc def Sum { in a : Real[1]; return : Real[1] = a; }
}
package Records {
    part def TimedRun :> AnalysisRecords::AnalysisRun {
        attribute :>> caseName default = "Demo::timed";
        attribute gain : ScalarValues::Real;
        attribute x : ScalarValues::Real;
    }
    part timed_run1 : TimedRun {
        @AnalysisRecords::RecordedRun {
            runAt = "2026-01-01T00:00:00Z";
            tool = "sysml dev";
            command = "%record Demo::timed";
            kind = "run";
        }
        attribute :>> caseName = "Demo::timed";
        attribute :>> kind = "run";
        attribute :>> 'objective' = "undecided";
        ref :>> 'subject' = Demo::probe;
        attribute :>> subjectName = "Demo::probe";
        attribute :>> gain = 2.0;
        attribute :>> x = 5.0;
    }
}
