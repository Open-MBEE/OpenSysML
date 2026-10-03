// The OpenSysML stack, as the landing page draws it. The in-browser engine
// parses this file, instantiates `stack` for the diagram's boxes and wires,
// and runs `ModelJourney` for the animation along them.
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

    // Where one model change travels: written and run in OpenSysML, committed
    // to Flexo, pulled into sysml-toolkit and pushed back, then checked
    // against the OMG reference. Each state is named for the part it is at.
    state def ModelJourney {
        entry; then opensysml;

        state opensysml;
        transition commit_change first opensysml accept commit then flexo;

        state flexo;
        transition pull_change first flexo accept pull then toolkit;
        transition check_change first flexo accept check then pilot;

        state toolkit;
        transition push_change first toolkit accept push then flexo;

        state pilot;
    }
}
