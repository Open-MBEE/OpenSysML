package Loops {
    action def Looping {
        in feed : ScalarValues::Integer[1];
        out 'out' : ScalarValues::Integer[1];
        first start then 'loop';
        /* not migrated: ObjectFlow (_ze1) — the loop variable 's' takes its value from its loop-variable input */
        /* not migrated: ObjectFlow (_back) — the flow runs from a later part of the loop to an earlier one; nothing carries the value back */
        action 'loop' {
            in iIn : ScalarValues::Integer[1];
            in sIn : ScalarValues::Integer[1];
            out iOut : ScalarValues::Integer[1];
            out sOut : ScalarValues::Integer[1];
            private attribute i : ScalarValues::Integer := iIn;
            private attribute s : ScalarValues::Integer := sIn;
            private attribute ended : ScalarValues::Boolean := false;
            first start then setup;
            action setup {
                first start then zero;
                action zero {
                    out r : ScalarValues::Integer[1];
                    assign r := 0;
                }
            }
            then action iterate {
                loop {
                    action test {
                        first start then le;
                        action le {
                            in x : ScalarValues::Integer[1] = i;
                            in y : ScalarValues::Integer[1] = s;
                            in z : ScalarValues::Integer[1] = setup.zero.r;
                            out r : ScalarValues::Boolean[1];
                            out k : ScalarValues::Integer[1];
                            assign r := x <= y;
                            assign k := 10 * x;
                        }
                    }
                    then if not test.le.r {
                        assign ended := true;
                    } else {
                        action body {
                            first start then plus;
                            action plus {
                                in x : ScalarValues::Integer[1] = s;
                                in y : ScalarValues::Integer[1] = test.le.k;
                                out r : ScalarValues::Integer[1];
                                assign r := x + y;
                            }
                            first plus then inc;
                            action inc {
                                in x : ScalarValues::Integer[1] = i;
                                out r : ScalarValues::Integer[1];
                                assign r := x + 1;
                            }
                        }
                        then action next {
                            assign i := body.inc.r;
                            then assign s := body.plus.r;
                        }
                    }
                } until ended;
            }
            then action results {
                assign iOut := i;
                then assign sOut := s;
            }
            metadata MigrationMetadata::SynthesizedName about iterate, results, ended, setup;
            metadata MigrationMetadata::StandIn about iterate, results, ended, setup;
        }
        first 'loop' then final;
        action final terminate;
        bind 'loop'.iIn = feed;
        bind 'loop'.sIn = feed;
        bind 'out' = 'loop'.sOut;
        metadata MigrationMetadata::SynthesizedName about final;
    }
    action def PostLoop {
        first start then post;
        action post {
            in xIn[0..*] nonunique;
            out xOut[0..*] nonunique;
            private ref x[0..*] nonunique := xIn;
            private attribute ended : ScalarValues::Boolean := false;
            first start then iterate;
            then action iterate {
                loop {
                    action body {
                        first start then grow;
                        action grow {
                            in v[0..*] nonunique = x;
                            out r[0..*] nonunique;
                            /* body not migrated (the call "v.including" is not in the translated function table) {JavaScript}:
                             * r = v.including(7);
                             */
                        }
                    }
                    then action next {
                        assign x := body.grow.r;
                    }
                    then action test {
                        first start then lt;
                        action lt {
                            in v[0..*] nonunique = x;
                            out r : ScalarValues::Boolean[1];
                            /* body not migrated (the call "v.size" is not in the translated function table) {JavaScript}:
                             * r = v.size() < 3;
                             */
                        }
                    }
                    then if not test.lt.r {
                        assign ended := true;
                    }
                } until ended;
            }
            then action results {
                assign xOut := x;
            }
            metadata MigrationMetadata::SynthesizedName about iterate, results, ended;
            metadata MigrationMetadata::StandIn about iterate, results, ended;
        }
        first post then final;
        action final terminate;
        metadata MigrationMetadata::SynthesizedName about final;
    }
}
