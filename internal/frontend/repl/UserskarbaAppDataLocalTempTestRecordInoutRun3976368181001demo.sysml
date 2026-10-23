package Demo {
    private import ScalarValues::*;
    private import DocumentQueries::*;
    private import KerML::Root::Element;
    part def Probe;
    part probe : Probe;
    analysis def Doubling {
        subject s : Probe;
        inout counter : Integer[1] = 3;
        return doubled : Integer[1] = counter * 2;
    }
    analysis tick : Doubling { subject s = probe; }
    calc def Counts :> DocumentQueries::Query {
        in root : Element[1];
        Project(source = WhereMetadata(
            source = Descendants(source = root, maxDepth = 10),
            'metadata' = "AnalysisRecords::RecordedRun"),
            properties = ("counter", "counterIn", "doubled"))
    }
    package Log {
        part def TickRun :> AnalysisRecords::AnalysisRun {
            attribute :>> caseName default = "Demo::tick";
            attribute counter : ScalarValues::Integer;
            attribute counterIn : ScalarValues::Integer;
            attribute doubled : ScalarValues::Integer;
        }
        part tick_run1 : TickRun {
            @AnalysisRecords::RecordedRun {
                runAt = "2026-01-01T00:00:00Z";
                tool = "sysml dev";
                command = "%record Demo::tick into Demo::Log";
                kind = "run";
            }
            attribute :>> caseName = "Demo::tick";
            attribute :>> kind = "run";
            attribute :>> 'objective' = "undecided";
            ref :>> 'subject' = Demo::probe;
            attribute :>> subjectName = "Demo::probe";
            attribute :>> counter = 3;
            attribute :>> counterIn = 3;
            attribute :>> doubled = 6;
        }
    }
}
