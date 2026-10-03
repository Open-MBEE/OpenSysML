part def Pipeline {
    attribute a : ScalarValues::Real default = 0.0;
    attribute b : ScalarValues::Real default = 0.0;
    attribute n : ScalarValues::Integer default = 0;
    attribute on : ScalarValues::Boolean default = true;
    action relay {
        first start then produce;
        action produce {
            out y : ScalarValues::Real[1];
            assign y := 2.5;
        }
        action consume {
            in v : ScalarValues::Real[1];
            assign a := v;
        }
        first consume then final;
        action final terminate;
        succession flow of ScalarValues::Real from produce.y to consume.v;
        metadata MigrationMetadata::SynthesizedName about final;
    }
    action gate {
        first start then produce;
        action produce {
            out y : ScalarValues::Real[1];
            assign y := 4.0;
        }
        action consume {
            in v : ScalarValues::Real[1];
            assign a := v;
        }
        first consume then final;
        action final terminate;
        first produce if on then 'produce.y to consume.v';
        succession flow 'produce.y to consume.v' of ScalarValues::Real from produce.y to consume.v;
        metadata MigrationMetadata::SynthesizedName about final, 'produce.y to consume.v';
    }
    action spread {
        first start then produce;
        action produce {
            out y : ScalarValues::Real[1];
            assign y := 3.0;
        }
        fork copy {
            in ref inputObject1 : ScalarValues::Real;
            out ref outputObject1 : ScalarValues::Real = inputObject1;
            out ref outputObject2 : ScalarValues::Real = inputObject1;
        }
        action left {
            in v : ScalarValues::Real[1];
            assign a := v;
        }
        first left then both;
        action right {
            in v : ScalarValues::Real[1];
            assign b := v * 2;
        }
        first right then both;
        join both;
        first both then final;
        action final terminate;
        succession flow of ScalarValues::Real from produce.y to copy.inputObject1;
        succession flow of ScalarValues::Real from copy.outputObject1 to left.v;
        succession flow of ScalarValues::Real from copy.outputObject2 to right.v;
        metadata MigrationMetadata::SynthesizedName about final;
    }
    action fan {
        first start then produce;
        action produce {
            out y : ScalarValues::Real[1];
            assign y := 5.0;
        }
        action left {
            in v : ScalarValues::Real[1];
            assign a := v;
        }
        action right {
            in v : ScalarValues::Real[1];
            assign b := v + 1;
        }
        succession flow of ScalarValues::Real from produce.y to left.v;
        succession flow of ScalarValues::Real from produce.y to right.v;
    }
    action hold {
        first start then 'start both';
        fork 'start both';
        first 'start both' then produce;
        first 'start both' then warm;
        action produce {
            out y : ScalarValues::Real[1];
            assign y := 7.0;
        }
        action warm {
            assign n := n + 1;
        }
        first warm then ready;
        join ready {
            in ref inputObject1 : ScalarValues::Real;
            out ref outputObject1 : ScalarValues::Real = inputObject1;
        }
        action consume {
            in v : ScalarValues::Real[1];
            assign a := v;
        }
        first consume then final;
        action final terminate;
        succession flow of ScalarValues::Real from produce.y to ready.inputObject1;
        succession flow of ScalarValues::Real from ready.outputObject1 to consume.v;
        metadata MigrationMetadata::SynthesizedName about final;
    }
    action either {
        first start then which;
        decide which;
        first which if on then high;
        first which if not on then low;
        action high {
            out y : ScalarValues::Real[1];
            assign y := 9.0;
        }
        action low {
            out y : ScalarValues::Real[1];
            assign y := 1.0;
        }
        merge pick {
            in ref inputObject1 : ScalarValues::Real;
            in ref inputObject2 : ScalarValues::Real;
            out ref outputObject1 : ScalarValues::Real = (inputObject1, inputObject2);
        }
        action consume {
            in v : ScalarValues::Real[1];
            assign a := v;
        }
        first consume then final;
        action final terminate;
        succession flow of ScalarValues::Real from high.y to pick.inputObject1;
        succession flow of ScalarValues::Real from low.y to pick.inputObject2;
        succession flow of ScalarValues::Real from pick.outputObject1 to consume.v;
        metadata MigrationMetadata::SynthesizedName about final;
    }
    action count {
        first start then zero;
        action zero {
            out y : ScalarValues::Integer[1];
            assign y := 0;
        }
        merge again {
            in ref inputObject1 : ScalarValues::Integer;
            in ref inputObject2 : ScalarValues::Integer;
            out ref outputObject1 : ScalarValues::Integer = (inputObject1, inputObject2);
        }
        action 'step' {
            in x : ScalarValues::Integer[1];
            out y : ScalarValues::Integer[1];
            assign y := x + 1;
            assign n := y;
        }
        first 'step' then 'fork';
        fork 'fork';
        first 'fork' then 'done yet';
        first 'fork' if n < 3 then again;
        decide 'done yet';
        else done;
        first 'done yet' if n >= 3 then final;
        action final terminate;
        succession flow of ScalarValues::Integer from zero.y to again.inputObject1;
        succession flow of ScalarValues::Integer from again.outputObject1 to 'step'.x;
        flow 'step'.y to again.inputObject2;
        metadata MigrationMetadata::SynthesizedName about 'fork', final;
        metadata MigrationMetadata::StandIn about 'fork';
    }
    action def Pair {
        first start then both;
        fork both;
        first both then low;
        first both then high;
        action low {
            out y : ScalarValues::Real[1];
            assign y := 1.5;
        }
        action high {
            out y : ScalarValues::Real[1];
            assign y := 8.5;
        }
        join gather {
            in ref inputObject1 : ScalarValues::Real;
            in ref inputObject2 : ScalarValues::Real;
            out ref outputObject1 : ScalarValues::Real nonunique = (inputObject1, inputObject2);
        }
        action keep {
            in v : ScalarValues::Real[2];
            /* body not migrated (the body is empty) */
        }
        succession flow of ScalarValues::Real from low.y to gather.inputObject1;
        succession flow of ScalarValues::Real from high.y to gather.inputObject2;
        succession flow of ScalarValues::Real from gather.outputObject1 to keep.v;
    }
    action def Sink {
        in v : ScalarValues::Real[1];
    }
    action def Stream {
        in ref context : Pipeline[1];
        first start then produce;
        action produce {
            out y : ScalarValues::Real[1];
            assign y := 6.0;
        }
        first produce then feed;
        action feed : Sink;
        flow produce.y to feed.v;
    }
}
