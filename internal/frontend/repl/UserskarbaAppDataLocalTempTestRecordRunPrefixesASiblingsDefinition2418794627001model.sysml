package Demo {
    private import ScalarValues::*;
    package A {
        analysis def Check { out x : Real[1] = 1.0; }
        analysis check : Check;
    }
    package B {
        analysis def Check { out x : Real[1] = 2.0; }
        analysis check : Check;
    }
    package Records {
        part def CheckRun :> AnalysisRecords::AnalysisRun {
            attribute :>> caseName default = "Demo::A::check";
            attribute x : ScalarValues::Real;
        }
        part check_run1 : CheckRun {
            @AnalysisRecords::RecordedRun {
                runAt = "2026-01-01T00:00:00Z";
                tool = "sysml dev";
                command = "%record Demo::A::check";
                kind = "run";
            }
            attribute :>> caseName = "Demo::A::check";
            attribute :>> kind = "run";
            attribute :>> 'objective' = "undecided";
            attribute :>> x = 1.0;
        }
        part def B_checkRun :> AnalysisRecords::AnalysisRun {
            attribute :>> caseName default = "Demo::B::check";
            attribute x : ScalarValues::Real;
        }
        part B_check_run1 : B_checkRun {
            @AnalysisRecords::RecordedRun {
                runAt = "2026-01-01T00:00:00Z";
                tool = "sysml dev";
                command = "%record Demo::B::check";
                kind = "run";
            }
            attribute :>> caseName = "Demo::B::check";
            attribute :>> kind = "run";
            attribute :>> 'objective' = "undecided";
            attribute :>> x = 2.0;
        }
    }
}
