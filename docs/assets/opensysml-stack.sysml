// The OpenSysML stack, as the landing page draws it. The in-browser engine
// parses this file, instantiates and renders `stack` through RenderView, and
// runs `ModelJourney` for the animation along its wires.
package OpenSysMLStack {
    private import ScalarValues::*;

    part def Project {
        attribute label : String;
        attribute kind : String;
        attribute lang : String;
        attribute role : String;
        attribute repo : String;
        port api : ModelPort;
    }

    port def ModelPort;

    interface def Interchange {
        end client : ModelPort;
        end store : ~ModelPort;
    }

    part stack {
        part opensysml : Project {
            :>> label = "OpenSysML";
            :>> kind = "runtime";
            :>> lang = "Go";
            :>> role = "runtime + LSP";
            :>> repo = "https://github.com/Open-MBEE/OpenSysML";
        }
        part toolkit : Project {
            :>> label = "sysml-toolkit";
            :>> kind = "toolchain";
            :>> lang = "Rust";
            :>> role = "toolchain + LSP";
            :>> repo = "https://github.com/Open-MBEE/sysml-toolkit";
        }
        part pilot : Project {
            :>> label = "SysML v2 Pilot Implementation";
            :>> kind = "reference";
            :>> lang = "Java";
            :>> role = "OMG reference";
            :>> repo = "https://github.com/Systems-Modeling/SysML-v2-Pilot-Implementation";
        }
        part flexo : Project {
            :>> label = "Flexo MMS";
            :>> kind = "model store";
            :>> lang = "Kotlin";
            :>> role = "the versioned store";
            :>> repo = "https://github.com/Open-MBEE/flexo-mms.openmbee.org";
        }

        interface opensysml_flexo : Interchange connect
            client ::> opensysml.api to store ::> flexo.api;
        interface toolkit_flexo : Interchange connect
            client ::> toolkit.api to store ::> flexo.api;
        interface pilot_flexo : Interchange connect
            client ::> pilot.api to store ::> flexo.api;
    }

    // Where one model change travels. Flexo is the hub: every project commits or
    // pushes to it and pulls from it, and no two projects trade directly. The
    // start and each Pull are free choices, so a seeded schedule picks the path.
    // Each state is named for the part it is at.
    item def Commit;
    item def Pull;
    item def Check;
    item def Push;

    state def ModelJourney {
        entry; then start;

        state start;
        transition write_first first start then opensysml;
        transition store_first first start then flexo;

        state opensysml;
        transition commit_change first opensysml accept Commit then flexo;

        state flexo;
        transition pull_into_toolkit first flexo accept Pull then toolkit;
        transition pull_into_pilot first flexo accept Pull then pilot;
        transition pull_into_opensysml first flexo accept Pull then opensysml;
        transition check_change first flexo accept Check then pilot;

        state toolkit;
        transition push_change first toolkit accept Push then flexo;

        state pilot;
        transition push_verdict first pilot accept Push then flexo;
    }
}
