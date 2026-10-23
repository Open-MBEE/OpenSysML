package Demo {
    private import ScalarValues::*;
    analysis def Bound { out y : Real[1] = 3.0; }
    analysis 'fuel budget' : Bound;
}
package Records {
    part def 'Fuel budgetRun' :> AnalysisRecords::AnalysisRun {
        attribute :>> caseName default = "Demo::fuel budget";
        attribute y : ScalarValues::Real;
    }
    part 'fuel budget_run1' : 'Fuel budgetRun' {
        @AnalysisRecords::RecordedRun {
            runAt = "2026-01-01T00:00:00Z";
            tool = "sysml dev";
            command = "%record Demo::'fuel budget'";
            kind = "run";
        }
        attribute :>> caseName = "Demo::fuel budget";
        attribute :>> kind = "run";
        attribute :>> 'objective' = "undecided";
        attribute :>> y = 3.0;
    }
}
