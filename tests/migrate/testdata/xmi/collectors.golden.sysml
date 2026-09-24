package Structure {
    part def Crane {
        doc /* Lifts containers off the quay. */
        part hook : Hook;
        ref part cable : Cable;
        ref part operator : Operator;
        ref counterweight;
        connect hook to cable;
        view 'Crane Internals' {
            expose hook;
            expose cable;
            render Views::asInterconnectionDiagram;
        }
    }
    part def Hook {
        doc /* Holds the spreader. */
        part latch : Latch;
    }
    part def Latch {
        part hook : Hook;
    }
    part def Cable {
        /* applied stereotype «TODO_Owner»: TODO = Rate the cable. */
    }
    part def Operator;
    part def Gantry {
        part trolley : Trolley;
        view 'Gantry Drive' {
            doc /* The trolley the gantry carries. */
            expose Gantry;
            expose Trolley;
            render Views::asTreeDiagram;
        }
    }
    part def Trolley {
        part motor : Motor;
    }
    part def Winch {
        part motor : Motor;
    }
    part def Motor {
        part brake : Brake;
    }
    part def Brake;
    view 'Crane Structure' {
        expose Hook;
        expose Crane;
        render Views::asTreeDiagram;
    }
    view Sketch {
        render Views::asTextualNotation;
    }
}
package Needs {
    doc /* What the yard
     * is asked to do.
     */
    requirement def <'Y-1'> Lift {
        doc /* The crane lifts a loaded container. */
    }
    requirement def <'Y-2'> Sway {
        doc /* The load sways less than one degree. */
        comment /* Above the hook. */
        view 'Sway Limits' {
            expose Sway;
            render Views::asTreeDiagram;
        }
    }
    view 'Yard Requirements' {
        expose Lift;
        expose Sway;
        render Views::asTreeDiagram;
    }
}
package 'Yard Viewpoints' {
    viewpoint 'Block Diagrams Viewpoint' {
        action def 'Block Diagrams Method' {
            first start then 'Filter By Diagram Type';
            action 'Filter By Diagram Type';
            first 'Filter By Diagram Type' then Image;
            action Image;
            first Image then final;
            action final terminate;
        }
        perform action 'block Diagrams Method' : 'Block Diagrams Method';
    }
    viewpoint 'Other Diagrams Viewpoint' {
        action def 'Other Diagrams Method' {
            first start then 'Filter By Diagram Type';
            action 'Filter By Diagram Type';
            first 'Filter By Diagram Type' then Image;
            action Image;
            first Image then final;
            action final terminate;
        }
        perform action 'other Diagrams Method' : 'Other Diagrams Method';
    }
    viewpoint 'Internal Diagrams Viewpoint' {
        action def 'Internal Diagrams Method' {
            first start then 'Collect Owned Elements';
            action 'Collect Owned Elements';
            first 'Collect Owned Elements' then 'Filter By Diagram Type';
            action 'Filter By Diagram Type';
            first 'Filter By Diagram Type' then Image;
            action Image;
            first Image then final;
            action final terminate;
        }
        perform action 'internal Diagrams Method' : 'Internal Diagrams Method';
    }
    viewpoint 'Parametric Diagrams Viewpoint' {
        action def 'Parametric Diagrams Method' {
            first start then 'Filter By Diagram Type';
            action 'Filter By Diagram Type';
            first 'Filter By Diagram Type' then Image;
            action Image;
            first Image then final;
            action final terminate;
        }
        perform action 'parametric Diagrams Method' : 'Parametric Diagrams Method';
    }
    viewpoint 'Shown Viewpoint' {
        action def 'Shown Method' {
            first start then 'Collect Things On Diagram';
            action 'Collect Things On Diagram';
            first 'Collect Things On Diagram' then 'Sort By Name';
            action 'Sort By Name';
            first 'Sort By Name' then 'Shown Blocks';
            action 'Shown Blocks' {
                first start then Name;
                action Name;
                first Name then Documentation;
                action Documentation;
            }
            first 'Shown Blocks' then final;
            action final terminate;
        }
        perform action 'shown Method' : 'Shown Method';
    }
    viewpoint 'Shown Everywhere Viewpoint' {
        action def 'Shown Everywhere Method' {
            first start then 'Collect Things On Diagram';
            action 'Collect Things On Diagram';
            first 'Collect Things On Diagram' then 'Shown Everywhere';
            action 'Shown Everywhere' {
                first start then Name;
                action Name;
            }
            first 'Shown Everywhere' then final;
            action final terminate;
        }
        perform action 'shown Everywhere Method' : 'Shown Everywhere Method';
    }
    viewpoint 'Drive Parts Viewpoint' {
        action def 'Drive Parts Method' {
            first start then 'Collect By Association';
            action 'Collect By Association';
            first 'Collect By Association' then 'Drive Parts';
            action 'Drive Parts';
            first 'Drive Parts' then final;
            action final terminate;
        }
        perform action 'drive Parts Method' : 'Drive Parts Method';
    }
    viewpoint 'Parts Viewpoint' {
        action def 'Parts Method' {
            first start then 'Collect By Association';
            action 'Collect By Association';
            first 'Collect By Association' then Parts;
            action Parts;
            first Parts then final;
            action final terminate;
        }
        perform action 'parts Method' : 'Parts Method';
    }
    viewpoint 'Direct Parts Viewpoint' {
        action def 'Direct Parts Method' {
            first start then 'Collect By Association';
            action 'Collect By Association';
            first 'Collect By Association' then 'Direct Parts';
            action 'Direct Parts';
            first 'Direct Parts' then final;
            action final terminate;
        }
        perform action 'direct Parts Method' : 'Direct Parts Method';
    }
    viewpoint 'Shared Parts Viewpoint' {
        action def 'Shared Parts Method' {
            first start then 'Collect By Association';
            action 'Collect By Association';
            first 'Collect By Association' then 'Shared Parts';
            action 'Shared Parts';
            first 'Shared Parts' then final;
            action final terminate;
        }
        perform action 'shared Parts Method' : 'Shared Parts Method';
    }
    viewpoint 'To Do Viewpoint' {
        action def 'To Do Method' {
            first start then 'Collect Owned Elements';
            action 'Collect Owned Elements';
            first 'Collect Owned Elements' then 'Filter By Stereotypes';
            action 'Filter By Stereotypes';
            first 'Filter By Stereotypes' then 'To Do Items';
            action 'To Do Items' {
                first start then Name;
                action Name;
            }
            first 'To Do Items' then final;
            action final terminate;
        }
        perform action 'to Do Method' : 'To Do Method';
    }
    viewpoint 'Documented Diagrams Viewpoint' {
        action def 'Documented Diagrams Method' {
            first start then 'Sort By Documentation';
            action 'Sort By Documentation';
            first 'Sort By Documentation' then 'Documented Owners';
            action 'Documented Owners';
            first 'Documented Owners' then 'Collect Owned Elements';
            action 'Collect Owned Elements';
            first 'Collect Owned Elements' then Image;
            action Image;
            first Image then final;
            action final terminate;
        }
        perform action 'documented Diagrams Method' : 'Documented Diagrams Method';
    }
    viewpoint 'Reverse Documented Diagrams Viewpoint' {
        action def 'Reverse Documented Diagrams Method' {
            first start then 'Sort By Documentation';
            action 'Sort By Documentation';
            first 'Sort By Documentation' then 'Collect Owned Elements';
            action 'Collect Owned Elements';
            first 'Collect Owned Elements' then Image;
            action Image;
            first Image then final;
            action final terminate;
        }
        perform action 'reverse Documented Diagrams Method' : 'Reverse Documented Diagrams Method';
    }
    viewpoint 'Diagrams By Documentation Viewpoint' {
        action def 'Diagrams By Documentation Method' {
            first start then 'Sort By Documentation';
            action 'Sort By Documentation';
            first 'Sort By Documentation' then Image;
            action Image;
            first Image then final;
            action final terminate;
        }
        perform action 'diagrams By Documentation Method' : 'Diagrams By Documentation Method';
    }
    viewpoint 'Owner Sorted Diagrams Viewpoint' {
        action def 'Owner Sorted Diagrams Method' {
            first start then 'Sort By Owner';
            action 'Sort By Owner';
            first 'Sort By Owner' then Image;
            action Image;
            first Image then final;
            action final terminate;
        }
        perform action 'owner Sorted Diagrams Method' : 'Owner Sorted Diagrams Method';
    }
    viewpoint 'Specification Viewpoint' {
        action def 'Specification Method' {
            first start then 'Collect Owned Elements';
            action 'Collect Owned Elements';
            first 'Collect Owned Elements' then 'Filter By Stereotypes';
            action 'Filter By Stereotypes';
            first 'Filter By Stereotypes' then 'Requirements Specification Table';
            action 'Requirements Specification Table' {
                first start then ID;
                action ID;
                first ID then Name;
                action Name;
                first Name then 'Requirement Text';
                action 'Requirement Text';
            }
            first 'Requirements Specification Table' then final;
            action final terminate;
        }
        perform action 'specification Method' : 'Specification Method';
    }
}
package 'Yard Handbook' {
    part def 'Yard Handbook' {
        view blockDiagrams :> 'Block Diagrams';
        view otherDiagrams :> 'Other Diagrams';
        view internalDiagrams :> 'Internal Diagrams';
        view parametricDiagrams :> 'Parametric Diagrams';
        view shown :> 'Shown On The Structure Diagram';
        view shownEverywhere :> 'Shown On Both Diagrams';
        view driveParts :> 'Drive Parts';
        view parts :> 'Crane Parts';
        view directParts :> 'Direct Crane Parts';
        view sharedParts :> 'Shared Crane Parts';
        view noParts :> 'Cable Parts';
        view toDo :> 'To Do';
        view specification :> Specification;
        view documentedDiagrams :> 'Documented Diagrams';
        view reverseDocumentedDiagrams :> 'Reverse Documented Diagrams';
        view diagramsByDocumentation :> 'Diagrams By Documentation';
        view ownerSortedDiagrams :> 'Owner Sorted Diagrams';
        /* applied stereotype «Document» */
    }
    view 'Block Diagrams' {
        satisfy 'Yard Viewpoints'::'Block Diagrams Viewpoint';
        expose Structure::'Crane Structure';
        expose Structure::Crane::'Crane Internals';
        expose Needs::'Yard Requirements';
        expose Structure::Sketch;
    }
    view 'Other Diagrams' {
        satisfy 'Yard Viewpoints'::'Other Diagrams Viewpoint';
        expose Structure::'Crane Structure';
        expose Structure::Crane::'Crane Internals';
        expose Needs::'Yard Requirements';
    }
    view 'Internal Diagrams' {
        satisfy 'Yard Viewpoints'::'Internal Diagrams Viewpoint';
        expose Structure::**;
    }
    view 'Parametric Diagrams' {
        satisfy 'Yard Viewpoints'::'Parametric Diagrams Viewpoint';
        expose Structure::'Crane Structure';
    }
    view 'Shown On The Structure Diagram' {
        satisfy 'Yard Viewpoints'::'Shown Viewpoint';
        expose Structure::'Crane Structure';
    }
    view 'Shown On Both Diagrams' {
        satisfy 'Yard Viewpoints'::'Shown Everywhere Viewpoint';
        expose Structure::'Crane Structure';
        expose Structure::Sketch;
    }
    view 'Drive Parts' {
        satisfy 'Yard Viewpoints'::'Drive Parts Viewpoint';
        expose Structure::Gantry;
        expose Structure::Winch;
    }
    view 'Crane Parts' {
        satisfy 'Yard Viewpoints'::'Parts Viewpoint';
        expose Structure::Crane;
    }
    view 'Direct Crane Parts' {
        satisfy 'Yard Viewpoints'::'Direct Parts Viewpoint';
        expose Structure::Crane;
    }
    view 'Shared Crane Parts' {
        satisfy 'Yard Viewpoints'::'Shared Parts Viewpoint';
        expose Structure::Crane;
    }
    view 'Cable Parts' {
        satisfy 'Yard Viewpoints'::'Parts Viewpoint';
        expose Structure::Cable;
    }
    view 'To Do' {
        satisfy 'Yard Viewpoints'::'To Do Viewpoint';
        expose Structure::**;
    }
    view Specification {
        satisfy 'Yard Viewpoints'::'Specification Viewpoint';
        expose Needs::**;
    }
    view 'Documented Diagrams' {
        satisfy 'Yard Viewpoints'::'Documented Diagrams Viewpoint';
        expose Structure::Gantry;
        expose Needs::**;
        expose Structure::Crane;
        expose Needs::Sway;
    }
    view 'Reverse Documented Diagrams' {
        satisfy 'Yard Viewpoints'::'Reverse Documented Diagrams Viewpoint';
        expose Structure::Gantry;
        expose Structure::Crane;
        expose Needs::**;
        expose Needs::Sway;
    }
    view 'Diagrams By Documentation' {
        satisfy 'Yard Viewpoints'::'Diagrams By Documentation Viewpoint';
        expose Structure::'Crane Structure';
        expose Structure::Gantry::'Gantry Drive';
    }
    view 'Owner Sorted Diagrams' {
        satisfy 'Yard Viewpoints'::'Owner Sorted Diagrams Viewpoint';
        expose Structure::'Crane Structure';
    }
    calc def 'Yard Handbook Shown Blocks Rows' :> DocumentQueries::Query {
        DocumentQueries::Project(
            source = DocumentQueries::OrderBy(
                source = DocumentQueries::Named(qualifiedName = ("Structure::Hook", "Structure::Crane")),
                property = "name",
                direction = "ascending",
                missing = "last",
                multiple = "first"),
            properties = ("name", "documentation"))
    }
    calc def 'Yard Handbook Drive Parts Rows' :> DocumentQueries::Query {
        DocumentQueries::Project(
            source = DocumentQueries::Named(qualifiedName = ("Structure::Trolley", "Structure::Motor", "Structure::Brake")),
            properties = ("name"))
    }
    calc def 'Yard Handbook Parts Rows' :> DocumentQueries::Query {
        DocumentQueries::Project(
            source = DocumentQueries::Named(qualifiedName = ("Structure::Hook", "Structure::Latch")),
            properties = ("name"))
    }
    calc def 'Yard Handbook Direct Parts Rows' :> DocumentQueries::Query {
        DocumentQueries::Project(
            source = DocumentQueries::Named(qualifiedName = ("Structure::Hook")),
            properties = ("name"))
    }
    calc def 'Yard Handbook Shared Parts Rows' :> DocumentQueries::Query {
        DocumentQueries::Project(
            source = DocumentQueries::Named(qualifiedName = ("Structure::Cable")),
            properties = ("name"))
    }
    calc def 'Yard Handbook Requirements Specification Table Rows' :> DocumentQueries::Query {
        DocumentQueries::Project(
            source = DocumentQueries::WhereType(
                source = DocumentQueries::Descendants(
                    source = DocumentQueries::Named(qualifiedName = ("Needs"))),
                type = ("RequirementDefinition")),
            properties = ("shortName", "name", "documentation"))
    }
    calc def 'Yard Handbook Documented Owners Rows' :> DocumentQueries::Query {
        DocumentQueries::Project(
            source = DocumentQueries::OrderBy(
                source = DocumentQueries::Named(qualifiedName = ("Structure::Gantry", "Needs", "Structure::Crane", "Needs::Sway")),
                property = "documentation",
                direction = "ascending",
                missing = "last",
                multiple = "first"),
            properties = ("name"))
    }
    part def 'Yard Handbook Document' :> DocumentQueries::Document {
        attribute redefines title = "Yard Handbook";
        ref crane : Structure::Crane;
        ref sway : Needs::Sway;
        ref gantry : Structure::Gantry;
        part 'Block Diagrams' : DocumentQueries::Section {
            attribute redefines title = "Block Diagrams";
            part diagram : DocumentQueries::Diagram {
                attribute redefines caption = "Crane Structure";
                ref redefines source = Structure::'Crane Structure';
            }
        }
        part 'Other Diagrams' : DocumentQueries::Section {
            attribute redefines title = "Other Diagrams";
            part diagram : DocumentQueries::Diagram {
                attribute redefines caption = "Crane Internals";
                ref redefines source = crane.'Crane Internals';
            }
            part 'diagram 2' : DocumentQueries::Diagram {
                attribute redefines caption = "Yard Requirements";
                ref redefines source = Needs::'Yard Requirements';
            }
        }
        part 'Internal Diagrams' : DocumentQueries::Section {
            attribute redefines title = "Internal Diagrams";
            part diagram : DocumentQueries::Diagram {
                attribute redefines caption = "Crane Internals";
                ref redefines source = crane.'Crane Internals';
            }
        }
        part 'Parametric Diagrams' : DocumentQueries::Section {
            attribute redefines title = "Parametric Diagrams";
        }
        part 'Shown On The Structure Diagram' : DocumentQueries::Section {
            attribute redefines title = "Shown On The Structure Diagram";
            part table : DocumentQueries::Table {
                attribute redefines caption = "Shown Blocks";
                calc rows : 'Yard Handbook Shown Blocks Rows';
            }
        }
        part 'Shown On Both Diagrams' : DocumentQueries::Section {
            attribute redefines title = "Shown On Both Diagrams";
            /* not migrated: «TableStructure» StructuredActivityNode 'Shown Everywhere' — the elements it shows pass through «CollectThingsOnDiagram» Yard Viewpoints::Shown Everywhere Viewpoint::Shown Everywhere Method::Collect Things On Diagram is not migrated: it collects what the diagram of unknown kind 'Sketch' shows, which the archive does not record */
        }
        part 'Drive Parts' : DocumentQueries::Section {
            attribute redefines title = "Drive Parts";
            part list : DocumentQueries::List {
                attribute redefines style = "bullet";
                calc items : 'Yard Handbook Drive Parts Rows';
            }
        }
        part 'Crane Parts' : DocumentQueries::Section {
            attribute redefines title = "Crane Parts";
            part list : DocumentQueries::List {
                attribute redefines style = "bullet";
                calc items : 'Yard Handbook Parts Rows';
            }
        }
        part 'Direct Crane Parts' : DocumentQueries::Section {
            attribute redefines title = "Direct Crane Parts";
            part list : DocumentQueries::List {
                attribute redefines style = "bullet";
                calc items : 'Yard Handbook Direct Parts Rows';
            }
        }
        part 'Shared Crane Parts' : DocumentQueries::Section {
            attribute redefines title = "Shared Crane Parts";
            part list : DocumentQueries::List {
                attribute redefines style = "bullet";
                calc items : 'Yard Handbook Shared Parts Rows';
            }
        }
        part 'Cable Parts' : DocumentQueries::Section {
            attribute redefines title = "Cable Parts";
        }
        part 'To Do' : DocumentQueries::Section {
            attribute redefines title = "To Do";
            /* not migrated: «TableStructure» StructuredActivityNode 'To Do Items' — the elements it shows pass through «FilterByStereotypes» Yard Viewpoints::To Do Viewpoint::To Do Method::Filter By Stereotypes is not migrated: no v2 metaclass stands for the elements of «TODO_Owner» */
        }
        part Specification : DocumentQueries::Section {
            attribute redefines title = "Specification";
            part table : DocumentQueries::Table {
                attribute redefines caption = "Requirements Specification Table";
                calc rows : 'Yard Handbook Requirements Specification Table Rows';
            }
        }
        part 'Documented Diagrams' : DocumentQueries::Section {
            attribute redefines title = "Documented Diagrams";
            part list : DocumentQueries::List {
                attribute redefines style = "bullet";
                calc items : 'Yard Handbook Documented Owners Rows';
            }
            part diagram : DocumentQueries::Diagram {
                attribute redefines caption = "Crane Internals";
                ref redefines source = crane.'Crane Internals';
            }
            part 'diagram 2' : DocumentQueries::Diagram {
                attribute redefines caption = "Sway Limits";
                ref redefines source = sway.'Sway Limits';
            }
            part 'diagram 3' : DocumentQueries::Diagram {
                attribute redefines caption = "Yard Requirements";
                ref redefines source = Needs::'Yard Requirements';
            }
            part 'diagram 4' : DocumentQueries::Diagram {
                attribute redefines caption = "Gantry Drive";
                ref redefines source = gantry.'Gantry Drive';
            }
        }
        part 'Reverse Documented Diagrams' : DocumentQueries::Section {
            attribute redefines title = "Reverse Documented Diagrams";
            part diagram : DocumentQueries::Diagram {
                attribute redefines caption = "Yard Requirements";
                ref redefines source = Needs::'Yard Requirements';
            }
            part 'diagram 2' : DocumentQueries::Diagram {
                attribute redefines caption = "Sway Limits";
                ref redefines source = sway.'Sway Limits';
            }
            part 'diagram 3' : DocumentQueries::Diagram {
                attribute redefines caption = "Crane Internals";
                ref redefines source = crane.'Crane Internals';
            }
            part 'diagram 4' : DocumentQueries::Diagram {
                attribute redefines caption = "Gantry Drive";
                ref redefines source = gantry.'Gantry Drive';
            }
        }
        part 'Diagrams By Documentation' : DocumentQueries::Section {
            attribute redefines title = "Diagrams By Documentation";
            part diagram : DocumentQueries::Diagram {
                attribute redefines caption = "Gantry Drive";
                ref redefines source = gantry.'Gantry Drive';
            }
            part 'diagram 2' : DocumentQueries::Diagram {
                attribute redefines caption = "Crane Structure";
                ref redefines source = Structure::'Crane Structure';
            }
        }
        part 'Owner Sorted Diagrams' : DocumentQueries::Section {
            attribute redefines title = "Owner Sorted Diagrams";
            /* not migrated: «Image» CallBehaviorAction 'Image' — the diagrams it shows pass through «SortByAttribute» Yard Viewpoints::Owner Sorted Diagrams Viewpoint::Owner Sorted Diagrams Method::Sort By Owner is not migrated: no query property stands for the attribute Owner */
        }
    }
}
