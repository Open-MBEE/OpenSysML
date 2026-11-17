package Dose {
    part def Tank;
    part def Gauge;
    action def Fill {
        in 'from' : Tank[1];
        out into : Tank[1];
    }
    action def Mix {
        in source : Tank[1];
        ref part spare : Tank {
            @MigrationMetadata::AppliedStereotype {
                stereotype = "AdjunctProperty";
                profile = "http://www.omg.org/spec/SysML/20181001/SysML";
                tags = "principal = Mix::fill::spare";
            }
        }
        ref part noted : Tank {
            @MigrationMetadata::AppliedStereotype {
                stereotype = "AdjunctProperty";
                profile = "http://www.omg.org/spec/SysML/20181001/SysML";
                tags = "principal = Mix::note::noted";
            }
        }
        first start then fill;
        action fill : Fill;
        first fill then settle;
        action settle {
            in settled : Tank[0..1];
        }
        /* not migrated: ControlFlow (_e3) — the edge leads to 'note', which never fires: no value reaches its input pin 'noted' */
        ref action note {
            in noted : Tank[1];
        }
        bind fill.'from' = source;
        /* flow fill.into to settle.settled not written: nothing in the called Fill gives its parameter into a value */
    }
    part def Plant {
        part gauge : Gauge;
        ref action mixing : Mix;
        connection watch connect gauge to mixing.fill.into;
        connection check connect gauge to mixing.settle.settled;
    }
    allocation def 'into to gauge' {
        end :>> source : Mix;
        end :>> target : Plant;
        allocate source.fill.into to target.gauge;
    }
    connection def MixFeed {
        end mix : Mix;
        end 'from' : Tank;
    }
    action mix : Mix;
    part tank : Tank;
    connection mixFeed : MixFeed connect mix to tank;
    view 'Mix BDD' {
        expose Mix;
        expose Mix::fill::into;
        render Views::asTreeDiagram;
    }
    metadata MigrationMetadata::SynthesizedName about 'into to gauge', mixFeed;
}
