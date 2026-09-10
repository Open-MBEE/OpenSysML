// The architecture document itself, written in the notation and rendered by
// OpenSysML's own document generator (see the README for the command).
package OpenSysMLDocument {
    private import DocumentQueries::*;
    private import KerML::Root::Element;
    private import ScalarValues::*;

    // The document's query roots: a table's query is bound to a usage, so the
    // containers it tabulates are named here as usages.
    part analysisPipeline : OpenSysMLPipeline::AnalysisPipeline;
    part passRegistry : OpenSysMLPipeline::PassRegistry;
    part runtime : OpenSysMLPipeline::Runtime;
    part viewEngine : OpenSysMLSurfaces::ViewEngine;

    // --- Queries the document runs over this model -----------------------

    // A usage's definition, so a query can reach the parts declared in it.
    calc def DefinitionOf :> Query {
        in root : Element;
        RelatedElements(
            source = root,
            relationshipKind = "typing",
            direction = "outgoing",
            maxDepth = 1
        )
    }

    // Every unit a container declares, with the directory implementing it;
    // the container's channels carry no package and drop out here.
    calc def UnitsWithPackages :> Query {
        in root : Element;
        Project(
            source = WhereFeature(
                source = WhereType(
                    source = OwnedElements(source = DefinitionOf(root = root)),
                    type = "PartUsage"
                ),
                'feature' = "goPackage",
                operator = "!=",
                value = ""
            ),
            properties = ("name", "goPackage")
        )
    }

    // The validation passes, with the tier each one runs at.
    calc def PassesWithTiers :> Query {
        in root : Element;
        Project(
            source = WhereType(
                source = OwnedElements(source = DefinitionOf(root = root)),
                type = "PartUsage"
            ),
            properties = ("name", "level", "elementScoped")
        )
    }

    // The runtime's budgets: the executors it also declares carry no envVar
    // and drop out here.
    calc def BudgetsWithBounds :> Query {
        in root : Element;
        Project(
            source = WhereFeature(
                source = WhereType(
                    source = OwnedElements(source = DefinitionOf(root = root)),
                    type = "PartUsage"
                ),
                'feature' = "envVar",
                operator = "!=",
                value = ""
            ),
            properties = ("name", "counts", "envVar", "defaultBound", "exhaustion")
        )
    }

    // Every rendering kind the view engine recognizes, and whether it draws it.
    calc def RenderingKinds :> Query {
        in root : Element;
        Project(
            source = WhereType(
                source = OwnedElements(source = DefinitionOf(root = root)),
                type = "PartUsage"
            ),
            properties = ("kind", "supported")
        )
    }

    // The conformance oracles, with the baseline each one ratchets.
    calc def OraclesWithBaselines :> Query {
        in root : Element;
        Project(
            source = WhereFeature(
                source = WhereType(
                    source = OwnedElements(source = DefinitionOf(root = root)),
                    type = "PartUsage"
                ),
                'feature' = "baseline",
                operator = "!=",
                value = ""
            ),
            properties = ("name", "reference", "baseline")
        )
    }

    // --- The document ----------------------------------------------------

    part def ArchitectureDocument :> Document {
        attribute redefines title = "OpenSysML Architecture";

        part summary : Paragraph {
            attribute redefines text = "OpenSysML analyses SysML v2 and KerML in one pass from text to a runnable graph. This document is generated from a model of that architecture rather than written alongside it: every diagram and table below is rendered from the same model the toolchain analyses, so a stage that moves in the code moves here too.";
        }

        part reading : Paragraph {
            part lead : Span {
                attribute redefines text = "The model lives in";
            }
            part source : Link {
                attribute redefines text = "examples/self-model";
                attribute redefines target = "https://github.com/Open-MBEE/OpenSysML/tree/main/examples/self-model";
            }
            part rest : Span {
                attribute redefines text = "and the prose reference it illustrates is docs/internals/architecture.md.";
            }
        }

        part pipeline : Section {
            attribute redefines title = "The analysis pipeline";

            part intro : Paragraph {
                attribute redefines text = "Text becomes tokens, a tree, a symbol index, resolved names, types, diagnostics and finally an execution graph. Each stage owns one artifact and hands it on; nothing downstream reaches back.";
            }

            part structure : Diagram {
                attribute redefines caption = "The stages and the artifacts that travel between them";
                attribute redefines direction = "LR";
                ref redefines source = OpenSysMLViews::pipelineStructure;
            }

            part stages : Table {
                attribute redefines caption = "Each stage and the package that implements it";
                calc rows : UnitsWithPackages {
                    in redefines root = analysisPipeline;
                }
            }

            part immutable : Paragraph {
                attribute redefines text = "The tree is immutable once parsed and carries syntax only. Everything derived from it lives in a side table keyed by node or symbol, which is why resolution can be lazy and memoized without a stale tree to invalidate.";
            }

            part libraryLoading : Paragraph {
                attribute redefines text = "The bundled standard library is loaded once per process from an embedded snapshot of its frozen index, written at go generate time and checked against the digest of the bundled files, the snapshot format and a checksum before it is trusted. A snapshot that fails any of those checks is ignored and the files are parsed as before, so the library files remain the source of truth and the snapshot is only a faster way to arrive at the same index.";
            }

            part libraryLoad : Diagram {
                attribute redefines caption = "Loading the library: the snapshot when it matches the files, the files otherwise";
                attribute redefines direction = "TB";
                ref redefines source = OpenSysMLViews::libraryLoadFlow;
            }
        }

        part checking : Section {
            attribute redefines title = "Validation and execution";

            part tiers : Paragraph {
                attribute redefines text = "Validation is a ladder: syntax, name resolution, type, then constraint. A blocking finding skips the document-scoped passes of the tiers above it, while a pass that names its subject still runs for the elements not downstream of the failure, so one broken declaration does not silence the rest of the document.";
            }

            part tierStates : Diagram {
                attribute redefines caption = "The tier ladder, and where it halts";
                attribute redefines direction = "TB";
                ref redefines source = OpenSysMLViews::tierStates;
            }

            part passes : Table {
                attribute redefines caption = "Every registered validation pass, its tier, and which ones gate themselves per element";
                calc rows : PassesWithTiers {
                    in redefines root = passRegistry;
                }
            }

            part executing : Paragraph {
                attribute redefines text = "What runs is the lowered graph, never the syntax tree: actions execute in succession order, as tokens over the lowered graph, and states as run-to-completion event dispatch. Each kind of work has its own budget, so a model that cannot finish reports which budget it exhausted rather than hanging the tool.";
            }

            part budgets : Table {
                attribute redefines caption = "The runtime budgets, their defaults, and the error each reports when exhausted";
                calc rows : BudgetsWithBounds {
                    in redefines root = runtime;
                }
            }

            part exhaustion : Diagram {
                attribute redefines caption = "Six budgets, six ways a run ends early";
                attribute redefines direction = "TB";
                ref redefines source = OpenSysMLViews::budgetExhaustion;
            }

            part calculating : Paragraph {
                attribute redefines text = "A calc whose body is scalar — an expression, or local declarations, returns and conditionals over scalars, calling other such calcs by position or by name and the library's scalar functions — is compiled on its first invocation into closures over an unboxed frame, kept in a side table on the context like every other derived fact. The evaluator remains the reference: the compiled tier must reproduce its values, errors and step charges exactly, a traced run or an invocation it cannot take goes to the evaluator whole, and an environment variable switches the tier off for bisecting.";
            }

            part calculation : Diagram {
                attribute redefines caption = "Invoking a calc: compiled when it may be, on the evaluator otherwise";
                attribute redefines direction = "TB";
                ref redefines source = OpenSysMLViews::calcInvocationFlow;
            }

            part analysed : Diagram {
                attribute redefines caption = "One document analysed end to end";
                attribute redefines direction = "TB";
                ref redefines source = OpenSysMLViews::analysisFlow;
            }
        }

        part surfaces : Section {
            attribute redefines title = "Surfaces over one pipeline";

            part intro : Paragraph {
                attribute redefines text = "The REPL, the language server, the gRPC service and the command line are four surfaces over one analysis pipeline. The service speaks gRPC and Connect from one protobuf schema, and the Go, Python, Node, Rust and Java clients are hand-written over stubs generated from that schema, so a rule fixed in the engine is fixed everywhere at once.";
            }

            part toolchain : Diagram {
                attribute redefines caption = "The whole toolchain";
                attribute redefines direction = "LR";
                ref redefines source = OpenSysMLViews::toolchainStructure;
            }

            part editing : Paragraph {
                attribute redefines text = "An editor reaches more than diagnostics: semantic tokens, quick fixes carrying their edits, suggestions for a name that did not resolve, and span-based edits whose result is re-parsed and re-analysed before it is accepted.";
            }

            part editor : Diagram {
                attribute redefines caption = "The editor pipeline";
                attribute redefines direction = "LR";
                ref redefines source = OpenSysMLViews::editorPipeline;
            }
        }

        part documents : Section {
            attribute redefines title = "Documents and views";

            part intro : Paragraph {
                attribute redefines text = "A document is written in the notation: its queries compile to an immutable plan, the plan evaluates to a backend-agnostic tree carrying provenance on every node, and the output form is chosen last, so Markdown and PDF cannot disagree about the content. This document is one of them.";
            }

            part path : Diagram {
                attribute redefines caption = "From a query in the model to a rendered artifact";
                attribute redefines direction = "LR";
                ref redefines source = OpenSysMLViews::documentPipeline;
            }

            part branches : Diagram {
                attribute redefines caption = "The two branches in rendering a document";
                attribute redefines direction = "TB";
                ref redefines source = OpenSysMLViews::documentFlow;
            }

            part kinds : Table {
                attribute redefines caption = "The rendering kinds a view can state, and which ones are produced";
                calc rows : RenderingKinds {
                    in redefines root = viewEngine;
                }
            }
        }

        part identity : Section {
            attribute redefines title = "Element identity";

            part intro : Paragraph {
                attribute redefines text = "An element's repository identity is an annotation, not a field on the tree: an explicit id when one is declared, otherwise one derived from the qualified name. The constraint tier checks the whole generated id space for collisions, and the RDF writer and reader carry the distinction between a declared and a derived id across a round trip.";
            }

            part path : Diagram {
                attribute redefines caption = "The identity path, from the library to RDF and back";
                attribute redefines direction = "LR";
                ref redefines source = OpenSysMLViews::identityPath;
            }

            part roundTrip : Diagram {
                attribute redefines caption = "Carrying an identity through a graph and back";
                attribute redefines direction = "TB";
                ref redefines source = OpenSysMLViews::identityRoundTrip;
            }

            part sync : Paragraph {
                attribute redefines text = "Repository synchronisation diffs a local model against its repository graph keyed by effective id, so a rename is an update rather than a delete and a create. Ids are minted and deletes confirmed only on request, and a conflict is reported in the exit status rather than resolved. The one phase of the identity design not built is the notation extension submitted to OMG.";
            }

            part syncing : Diagram {
                attribute redefines caption = "Diffing a local model against its repository";
                attribute redefines direction = "TB";
                ref redefines source = OpenSysMLViews::syncFlow;
            }
        }

        part quality : Section {
            attribute redefines title = "Invariants and oracles";

            part intro : Paragraph {
                attribute redefines text = "The architecture rules are requirements over the modelled parts, and the test suites that verify them are named in the model, so each rule points at the run that would catch its violation.";
            }

            part invariants : Diagram {
                attribute redefines caption = "The architecture invariants and the parts they constrain";
                ref redefines source = OpenSysMLViews::invariantTable;
            }

            part oracles : Table {
                attribute redefines caption = "The conformance oracles and the baselines they ratchet";
                calc rows : OraclesWithBaselines {
                    in redefines root = OpenSysMLSurfaces::opensysml;
                }
            }
        }
    }
}
