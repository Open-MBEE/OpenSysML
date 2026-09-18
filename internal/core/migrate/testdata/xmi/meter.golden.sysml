part def Meter {
    attribute total : ScalarValues::Real default = 0.0;
    attribute ticks : ScalarValues::Integer default = 7;
    attribute half : ScalarValues::Integer default = 0;
    attribute ratio : ScalarValues::Real default = 0.0;
    action def Measure {
        first start then sense;
        action sense {
            in x : ScalarValues::Real = 1.5;
            out y : ScalarValues::Real;
            assign y := x * 2;
        }
        first sense then record;
        action record {
            in v : ScalarValues::Real;
            assign this.total := v + 1;
        }
        first record then split;
        action split {
            assign this.half := RealFunctions::floor((this.ticks - this.ticks % 2) / 2);
            assign this.ratio := this.total / 2;
        }
        first split then final;
        action final terminate;
        flow sense.y to record.v;
    }
    action def Skip {
        first start then idle;
        action idle {
            out z : ScalarValues::Real;
            assign this.total := this.total * 1;
        }
        first idle then sink;
        action sink {
            in w : ScalarValues::Real;
            assign this.total := w;
        }
        first sink then dark;
        action dark {
            out q : ScalarValues::Real;
            /* body not migrated (the statement assigns q, which nothing visible from Meter::Skip::dark is called (language Python)) {Python}:
             * q = 1
             */
        }
        first dark then drain;
        action drain {
            in w : ScalarValues::Real;
            assign this.total := w;
        }
        first drain then final;
        action final terminate;
        /* flow idle.z to sink.w not written: the body of 'idle' never assigns idle.z */
        /* flow dark.q to drain.w not written: 'dark' is not migrated and produces no value */
    }
}
