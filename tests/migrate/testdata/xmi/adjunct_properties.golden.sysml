package Model {
    part def Pump;
    part def Tank;
    connection def Pipe {
        end 'from' : Pump;
        end 'to' : Tank;
    }
    part def Plant {
        part pump : Pump;
        part tank : Tank;
        connection feed connect pump to tank;
        state def Modes {
            in ref context : Plant[1];
            entry; then Cycling;
            state Cycling : Cycle { in ref :>> context = Modes::context; }
        }
        state def Cycle {
            in ref context : Plant[1];
            entry; then On;
            state On;
        }
        occurrence def Prime {
            ref part p : Pump;
        }
        occurrence def Startup {
            ref part p : Pump;
            occurrence priming : Prime;
            /* not migrated: «AdjunctProperty» Property 'stray' — owned by an Interaction, whose v2 body is its lifelines and messages, not a place for a «AdjunctProperty» Property no lifeline stands for; «AdjunctProperty» is not collapsed into its principal: its principal is not in the document; applied stereotypes «AdjunctProperty» (principal = _nowhere) */
        }
        exhibit state modes : Modes { in ref :>> context = this; }
        occurrence prime : Prime {
            ref part :>> p = pump;
        }
        occurrence startup : Startup {
            ref part :>> p = pump;
        }
        metadata MigrationMetadata::SynthesizedName about prime, startup;
    }
    action def Fill;
    action def Drain;
    action def Run {
        in level : ScalarValues::Real[1];
        /* not migrated: Comment (_adjNamedDoc) — owned by an «AdjunctProperty» that is written as the usage of its principal, which has no place for a Comment */
        ref part queue : Tank[0..*] {
            @MigrationMetadata::AppliedStereotype {
                stereotype = "AdjunctProperty";
                profile = "http://www.omg.org/spec/SysML/20181001/SysML";
                tags = "principal = Run::queue";
            }
        }
        private attribute count : ScalarValues::Integer;
        first start then 'fork';
        fork 'fork';
        first 'fork' then call;
        first 'fork' then 'queue 2';
        action call : Fill;
        first call then empty;
        action empty : Drain;
        action 'queue 2';
        metadata MigrationMetadata::SynthesizedName about 'fork', call;
        metadata MigrationMetadata::StandIn about 'fork';
    }
    connection def RunToFill {
        end run : Run;
        end call : Fill crosses run.call;
        metadata MigrationMetadata::SynthesizedName about run, call;
    }
    allocation def 'empty to Pump' {
        end :>> source : Run;
        end :>> target : Pump;
        allocate source.empty to target;
    }
    action def Apart {
        ref action lost : Fill {
            @MigrationMetadata::AppliedStereotype {
                stereotype = "AdjunctProperty";
                profile = "http://www.omg.org/spec/SysML/20181001/SysML";
                tags = "principal = _nowhere";
            }
        }
        ref action note : Fill {
            @MigrationMetadata::AppliedStereotype {
                stereotype = "AdjunctProperty";
                profile = "http://www.omg.org/spec/SysML/20181001/SysML";
                tags = "principal = Apart::note";
            }
        }
        ref action borrowed : Drain {
            @MigrationMetadata::AppliedStereotype {
                stereotype = "AdjunctProperty";
                profile = "http://www.omg.org/spec/SysML/20181001/SysML";
                tags = "principal = Run::empty";
            }
        }
        ref action refill : Drain {
            @MigrationMetadata::AppliedStereotype {
                stereotype = "AdjunctProperty";
                profile = "http://www.omg.org/spec/SysML/20181001/SysML";
                tags = "principal = Apart::refill";
            }
        }
        first start then 'fork';
        fork 'fork';
        first 'fork' then 'note 2';
        first 'fork' then 'refill 2';
        action 'note 2';
        action 'refill 2' : Fill;
        metadata MigrationMetadata::SynthesizedName about 'fork';
        metadata MigrationMetadata::StandIn about 'fork';
    }
    part pump : Pump;
    part tank : Tank;
    connection pipe : Pipe connect pump to tank;
    action run : Run;
    action fill : Fill;
    connection 'run to fill' : RunToFill connect run to fill;
    metadata MigrationMetadata::SynthesizedName about RunToFill, 'empty to Pump', pipe, 'run to fill';
}
