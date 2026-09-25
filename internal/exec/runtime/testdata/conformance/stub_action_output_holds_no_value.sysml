// A declared action that computes nothing — `measure` has parameters and no
// body, as a v1 call action naming no behavior migrates to — fires, passes the
// token on and leaves its `out reading [0..1]` empty. The flow from it carries
// no value to `record.value`, which is declared admitting none, so `record`
// runs with the parameter empty: the guarded assignment is skipped, `last` is
// never written, and the activity completes rather than failing or waiting. Its
// own `out sample [0..1]`, bound to the empty output, stays empty too.
package test {
    private import ScalarValues::*;
    private import SequenceFunctions::*;

    action def Record {
        in value : Real[0..1];
        out written : Boolean = false;
        first start then step;
        action step {
            if value->notEmpty() {
                assign written := true;
            }
        }
    }

    action def Sampling {
        out sample : Real[0..1];
        attribute last : Real[0..1];
        attribute recorded : Boolean = false;
        attribute readings : Integer = 0;

        first start then gain;
        action gain { out result : Real = 2.5; }
        first gain then measure;
        action measure {
            in gain : Real;
            out reading : Real[0..1];
        }
        first measure then record;
        action record : Record;
        first record then tally;
        action tally {
            assign readings := measure.reading->size();
            assign recorded := record.written;
        }
        first tally then done;

        flow gain.result to measure.gain;
        flow measure.reading to record.value;
        bind sample = measure.reading;
    }
}
