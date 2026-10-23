package Demo {
    private import ScalarValues::*;
    part def Engine { attribute thrust : Real; }
    part engine : Engine { attribute :>> thrust = 2800.0; }
    verification def Fire {
        subject e : Engine;
        VerificationCases::PassIf(e.thrust >= 3000.0)
    }
    verification fire : Fire { subject e = engine; }
}
package Records {
    part def FireRun :> AnalysisRecords::AnalysisRun {
        attribute :>> caseName default = "Demo::fire";
        attribute result : VerificationCases::VerdictKind[0..*] ordered nonunique;
    }
    part fire_run1 : FireRun {
        @AnalysisRecords::RecordedRun {
            runAt = "2026-01-01T00:00:00Z";
            tool = "sysml dev";
            command = "%record Demo::fire";
            kind = "run";
        }
        attribute :>> caseName = "Demo::fire";
        attribute :>> kind = "run";
        attribute :>> 'objective' = "undecided";
        attribute :>> verdict = "fail";
        ref :>> 'subject' = Demo::engine;
        attribute :>> subjectName = "Demo::engine";
        attribute :>> result = (VerificationCases::VerdictKind::fail);
        part verification1 : AnalysisRecords::VerdictRecord :> verdicts {
            attribute :>> kind = "verification";
            attribute :>> name = "Demo::fire";
            attribute :>> status = "fail";
        }
    }
}
