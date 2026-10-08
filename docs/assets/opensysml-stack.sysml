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

    // A subsystem a project ships, drawn inside the project's box.
    part def Component {
        attribute label : String;
        attribute kind : String = "component";
        attribute role : String;
    }

    interface def Interchange {
        end client : ModelPort;
        end store : ~ModelPort;
    }

    part stack {
        part opensysml : Project {
            :>> label = "OpenSysML Runtime Environment and Development Kit";
            :>> kind = "runtime";
            :>> lang = "Go";
            :>> role = "the runtime";
            :>> repo = "https://github.com/Open-MBEE/OpenSysML";
            part parser : Component {
                :>> label = "Parser and formatter";
                :>> role = "lexer, parser, formatter";
            }
            part validation : Component {
                :>> label = "Validation";
                :>> role = "syntax, names, types, constraints";
            }
            part stdlib : Component {
                :>> label = "Standard library";
                :>> role = "bundled KerML and SysML libraries";
            }
            part lsp : Component {
                :>> label = "Language server";
                :>> role = "sysml-lsp";
            }
            part editors : Component {
                :>> label = "Editor integrations";
                :>> role = "VS Code, Cameo MDK, SysON";
            }
            part repl : Component {
                :>> label = "REPL";
                :>> role = "sysml";
            }
            part jupyter : Component {
                :>> label = "Jupyter kernel";
                :>> role = "sysml-jupyter-kernel";
            }
            part engine : Component {
                :>> label = "Execution runtime";
                :>> role = "parts, actions, states";
            }
            part solver : Component {
                :>> label = "Constraint solver";
                :>> role = "SMT via Z3, model checking";
            }
            part analyzers : Component {
                :>> label = "Analysis engines";
                :>> role = "explore, check, FMI co-simulation";
            }
            part service : Component {
                :>> label = "gRPC service";
                :>> role = "sysml-grpc";
            }
            part clients : Component {
                :>> label = "Clients and bindings";
                :>> role = "Python, Node, Java, Rust, Julia, MATLAB, WebAssembly";
            }
            part docgen : Component {
                :>> label = "Document generation";
                :>> role = "Markdown, HTML, PDF";
            }
            part interchange : Component {
                :>> label = "Interchange";
                :>> role = "RDF, XMI, notation";
            }
            part migration : Component {
                :>> label = "SysML v1 migration";
                :>> role = "XMI to v2 notation";
            }
            part oslc : Component {
                :>> label = "OSLC query";
                :>> role = "REPL and service";
            }
        }
        part pilot : Project {
            :>> label = "SysML v2 Pilot Implementation";
            :>> kind = "reference";
            :>> lang = "Java";
            :>> role = "OMG reference";
            :>> repo = "https://github.com/Systems-Modeling/SysML-v2-Pilot-Implementation";
            part grammars : Component {
                :>> label = "Xtext parsers";
                :>> role = "KerML and SysML grammars";
            }
            part validation : Component {
                :>> label = "Validation";
                :>> role = "batch KerML and SysML validators";
            }
            part evaluator : Component {
                :>> label = "Expression evaluator";
                :>> role = "headless execution";
            }
            part stdlib : Component {
                :>> label = "Standard library";
                :>> role = "KerML and SysML libraries";
            }
            part editors : Component {
                :>> label = "Xtext editors";
                :>> role = "Eclipse";
            }
            part plantuml : Component {
                :>> label = "PlantUML visualization";
                :>> role = "diagrams from text";
            }
            part jupyter : Component {
                :>> label = "Jupyter kernel";
                :>> role = "notebooks";
            }
            part xmi : Component {
                :>> label = "Interchange";
                :>> role = "XMI";
            }
        }
        part toolkit : Project {
            :>> label = "sysml-toolkit";
            :>> kind = "toolchain";
            :>> lang = "Rust";
            :>> role = "toolchain + LSP";
            :>> repo = "https://github.com/Open-MBEE/sysml-toolkit";
            part parser : Component {
                :>> label = "Parser and formatter";
                :>> role = "span-preserving syntax tree";
            }
            part linter : Component {
                :>> label = "Linter";
                :>> role = "style and model checks";
            }
            part interchange : Component {
                :>> label = "Interchange";
                :>> role = "JSON, CBOR";
            }
            part solver : Component {
                :>> label = "Constraint solver";
                :>> role = "constraint satisfaction";
            }
            part transform : Component {
                :>> label = "Model transformation";
                :>> role = "span-preserving rewrites";
            }
            part lsp : Component {
                :>> label = "Language server";
                :>> role = "editor integration";
            }
            part bindings : Component {
                :>> label = "Bindings";
                :>> role = "Python, WebAssembly";
            }
        }
        part flexo : Project {
            :>> label = "Flexo MMS";
            :>> kind = "model store";
            :>> lang = "Kotlin";
            :>> role = "the versioned store";
            :>> repo = "https://github.com/Open-MBEE/flexo-mms.openmbee.org";
            part layer1 : Component {
                :>> label = "Layer 1 service";
                :>> role = "orgs, repos, branches, commits";
            }
            part sysmlv2 : Component {
                :>> label = "SysML v2 API service";
                :>> role = "flexo-mms-sysmlv2";
            }
            part quadstore : Component {
                :>> label = "Quadstore";
                :>> role = "SPARQL 1.1";
            }
            part auth : Component {
                :>> label = "Auth";
                :>> role = "JSON Web Tokens";
            }
        }

        interface opensysml_flexo : Interchange connect
            client ::> opensysml.api to store ::> flexo.api;
        interface pilot_flexo : Interchange connect
            client ::> pilot.api to store ::> flexo.api;
        interface toolkit_flexo : Interchange connect
            client ::> toolkit.api to store ::> flexo.api;
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
