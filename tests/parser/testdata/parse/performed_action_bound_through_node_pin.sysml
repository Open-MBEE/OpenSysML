package test {
    private import ScalarValues::*;
    item def Report { attribute size : Real; }
    action def Measure { out report : Report; }
    action def Decide {
        in report : Report;
        out go : Boolean = report.size <= 10.0;
    }
    action def Sequence {
        action measure : Measure;
        then action decision : Decide { in report = measure.report; }
        then done;
    }
    part context {
        item theReport : Report { attribute :>> size = 6.8; }
        perform action run : Sequence;
        bind run.measure.report = theReport;
        attribute go : Boolean = run.decision.go;
    }
    part farContext {
        item theReport : Report { attribute :>> size = 12.5; }
        perform action run : Sequence;
        bind run.measure.report = theReport;
        attribute go : Boolean = run.decision.go;
    }
    part site {
        part near : context;
        part far : farContext;
        attribute nearGo : Boolean = near.go;
        attribute farGo : Boolean = far.go;
    }
}
