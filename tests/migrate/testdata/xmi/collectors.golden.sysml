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
package Sketches {
    view Sketch {
        render Views::asTextualNotation;
    }
}
package Drafts {
    part def unnamed {
        view Overview {
            expose unnamed;
            expose Structure::Cable;
            render Views::asTreeDiagram;
        }
    }
    part def Sketchy {
        view 'Sketchy Overview' {
            expose Sketchy;
            render Views::asTreeDiagram;
        }
    }
    metadata MigrationMetadata::SynthesizedName about unnamed;
}
package 'Yard Viewpoints' {
    view def 'Block Diagrams Viewpoint' {
        viewpoint 'block Diagrams Viewpoint' {
            subject;
        }
        satisfy 'block Diagrams Viewpoint';
        action def 'Block Diagrams Method' {
            first start then 'Filter By Diagram Type';
            action 'Filter By Diagram Type';
            first 'Filter By Diagram Type' then Image;
            action Image;
            first Image then final;
            action final terminate;
            metadata MigrationMetadata::SynthesizedName about final;
        }
        perform action 'block Diagrams Method' : 'Block Diagrams Method';
        metadata MigrationMetadata::SynthesizedName about 'block Diagrams Viewpoint';
    }
    view def 'All Diagrams Viewpoint' {
        viewpoint 'all Diagrams Viewpoint' {
            subject;
        }
        satisfy 'all Diagrams Viewpoint';
        action def 'All Diagrams Method' {
            first start then 'Filter By Diagram Type';
            action 'Filter By Diagram Type';
            first 'Filter By Diagram Type' then Image;
            action Image;
            first Image then final;
            action final terminate;
            metadata MigrationMetadata::SynthesizedName about final;
        }
        perform action 'all Diagrams Method' : 'All Diagrams Method';
        metadata MigrationMetadata::SynthesizedName about 'all Diagrams Viewpoint';
    }
    view def 'No Diagrams Viewpoint' {
        viewpoint 'no Diagrams Viewpoint' {
            subject;
        }
        satisfy 'no Diagrams Viewpoint';
        action def 'No Diagrams Method' {
            first start then 'Filter By Diagram Type';
            action 'Filter By Diagram Type';
            first 'Filter By Diagram Type' then Image;
            action Image;
            first Image then final;
            action final terminate;
            metadata MigrationMetadata::SynthesizedName about final;
        }
        perform action 'no Diagrams Method' : 'No Diagrams Method';
        metadata MigrationMetadata::SynthesizedName about 'no Diagrams Viewpoint';
    }
    view def 'Sketched Diagrams Viewpoint' {
        viewpoint 'sketched Diagrams Viewpoint' {
            subject;
        }
        satisfy 'sketched Diagrams Viewpoint';
        action def 'Sketched Diagrams Method' {
            first start then 'fork';
            fork 'fork';
            first 'fork' then 'Filter By Names';
            first 'fork' then 'Filter By Diagram Type';
            action 'Filter By Names';
            first 'Filter By Names' then 'merge';
            action 'Filter By Diagram Type';
            first 'Filter By Diagram Type' then 'merge';
            merge 'merge';
            first 'merge' then Image;
            action Image;
            first Image then final;
            action final terminate;
            metadata MigrationMetadata::SynthesizedName about 'fork', 'merge', final;
        }
        perform action 'sketched Diagrams Method' : 'Sketched Diagrams Method';
        metadata MigrationMetadata::SynthesizedName about 'sketched Diagrams Viewpoint';
    }
    view def 'Retargeted Diagrams Viewpoint' {
        viewpoint 'retargeted Diagrams Viewpoint' {
            subject;
        }
        satisfy 'retargeted Diagrams Viewpoint';
        action def 'Retargeted Diagrams Method' {
            first start then 'Filter By Diagram Type';
            action 'Filter By Diagram Type';
            first 'Filter By Diagram Type' then 'fork';
            fork 'fork';
            first 'fork' then 'Filter By Names';
            first 'fork' then 'Sort By Name';
            action 'Filter By Names';
            first 'Filter By Names' then 'merge';
            action 'Sort By Name';
            first 'Sort By Name' then 'merge';
            merge 'merge';
            first 'merge' then Image;
            action Image;
            first Image then final;
            action final terminate;
            metadata MigrationMetadata::SynthesizedName about 'fork', 'merge', final;
        }
        perform action 'retargeted Diagrams Method' : 'Retargeted Diagrams Method';
        metadata MigrationMetadata::SynthesizedName about 'retargeted Diagrams Viewpoint';
    }
    view def 'Kin Diagrams Viewpoint' {
        viewpoint 'kin Diagrams Viewpoint' {
            subject;
        }
        satisfy 'kin Diagrams Viewpoint';
        action def 'Kin Diagrams Method' {
            first start then 'Collect Owners';
            action 'Collect Owners';
            first 'Collect Owners' then 'Collect Owned Elements';
            action 'Collect Owned Elements';
            first 'Collect Owned Elements' then Image;
            action Image;
            first Image then final;
            action final terminate;
            metadata MigrationMetadata::SynthesizedName about final;
        }
        perform action 'kin Diagrams Method' : 'Kin Diagrams Method';
        metadata MigrationMetadata::SynthesizedName about 'kin Diagrams Viewpoint';
    }
    view def 'Other Diagrams Viewpoint' {
        viewpoint 'other Diagrams Viewpoint' {
            subject;
        }
        satisfy 'other Diagrams Viewpoint';
        action def 'Other Diagrams Method' {
            first start then 'Filter By Diagram Type';
            action 'Filter By Diagram Type';
            first 'Filter By Diagram Type' then Image;
            action Image;
            first Image then final;
            action final terminate;
            metadata MigrationMetadata::SynthesizedName about final;
        }
        perform action 'other Diagrams Method' : 'Other Diagrams Method';
        metadata MigrationMetadata::SynthesizedName about 'other Diagrams Viewpoint';
    }
    view def 'Internal Diagrams Viewpoint' {
        viewpoint 'internal Diagrams Viewpoint' {
            subject;
        }
        satisfy 'internal Diagrams Viewpoint';
        action def 'Internal Diagrams Method' {
            first start then 'Collect Owned Elements';
            action 'Collect Owned Elements';
            first 'Collect Owned Elements' then 'Filter By Diagram Type';
            action 'Filter By Diagram Type';
            first 'Filter By Diagram Type' then Image;
            action Image;
            first Image then final;
            action final terminate;
            metadata MigrationMetadata::SynthesizedName about final;
        }
        perform action 'internal Diagrams Method' : 'Internal Diagrams Method';
        metadata MigrationMetadata::SynthesizedName about 'internal Diagrams Viewpoint';
    }
    view def 'Parametric Diagrams Viewpoint' {
        viewpoint 'parametric Diagrams Viewpoint' {
            subject;
        }
        satisfy 'parametric Diagrams Viewpoint';
        action def 'Parametric Diagrams Method' {
            first start then 'Filter By Diagram Type';
            action 'Filter By Diagram Type';
            first 'Filter By Diagram Type' then Image;
            action Image;
            first Image then final;
            action final terminate;
            metadata MigrationMetadata::SynthesizedName about final;
        }
        perform action 'parametric Diagrams Method' : 'Parametric Diagrams Method';
        metadata MigrationMetadata::SynthesizedName about 'parametric Diagrams Viewpoint';
    }
    view def 'Shown Viewpoint' {
        viewpoint 'shown Viewpoint' {
            subject;
        }
        satisfy 'shown Viewpoint';
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
            metadata MigrationMetadata::SynthesizedName about final;
        }
        perform action 'shown Method' : 'Shown Method';
        metadata MigrationMetadata::SynthesizedName about 'shown Viewpoint';
    }
    view def 'Shown Everywhere Viewpoint' {
        viewpoint 'shown Everywhere Viewpoint' {
            subject;
        }
        satisfy 'shown Everywhere Viewpoint';
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
            metadata MigrationMetadata::SynthesizedName about final;
        }
        perform action 'shown Everywhere Method' : 'Shown Everywhere Method';
        metadata MigrationMetadata::SynthesizedName about 'shown Everywhere Viewpoint';
    }
    view def 'Drive Parts Viewpoint' {
        viewpoint 'drive Parts Viewpoint' {
            subject;
        }
        satisfy 'drive Parts Viewpoint';
        action def 'Drive Parts Method' {
            first start then 'Collect By Association';
            action 'Collect By Association';
            first 'Collect By Association' then 'Drive Parts';
            action 'Drive Parts';
            first 'Drive Parts' then final;
            action final terminate;
            metadata MigrationMetadata::SynthesizedName about final;
        }
        perform action 'drive Parts Method' : 'Drive Parts Method';
        metadata MigrationMetadata::SynthesizedName about 'drive Parts Viewpoint';
    }
    view def 'Parts Viewpoint' {
        viewpoint 'parts Viewpoint' {
            subject;
        }
        satisfy 'parts Viewpoint';
        action def 'Parts Method' {
            first start then 'Collect By Association';
            action 'Collect By Association';
            first 'Collect By Association' then Parts;
            action Parts;
            first Parts then final;
            action final terminate;
            metadata MigrationMetadata::SynthesizedName about final;
        }
        perform action 'parts Method' : 'Parts Method';
        metadata MigrationMetadata::SynthesizedName about 'parts Viewpoint';
    }
    view def 'Direct Parts Viewpoint' {
        viewpoint 'direct Parts Viewpoint' {
            subject;
        }
        satisfy 'direct Parts Viewpoint';
        action def 'Direct Parts Method' {
            first start then 'Collect By Association';
            action 'Collect By Association';
            first 'Collect By Association' then 'Direct Parts';
            action 'Direct Parts';
            first 'Direct Parts' then final;
            action final terminate;
            metadata MigrationMetadata::SynthesizedName about final;
        }
        perform action 'direct Parts Method' : 'Direct Parts Method';
        metadata MigrationMetadata::SynthesizedName about 'direct Parts Viewpoint';
    }
    view def 'Shared Parts Viewpoint' {
        viewpoint 'shared Parts Viewpoint' {
            subject;
        }
        satisfy 'shared Parts Viewpoint';
        action def 'Shared Parts Method' {
            first start then 'Collect By Association';
            action 'Collect By Association';
            first 'Collect By Association' then 'Shared Parts';
            action 'Shared Parts';
            first 'Shared Parts' then final;
            action final terminate;
            metadata MigrationMetadata::SynthesizedName about final;
        }
        perform action 'shared Parts Method' : 'Shared Parts Method';
        metadata MigrationMetadata::SynthesizedName about 'shared Parts Viewpoint';
    }
    view def 'To Do Viewpoint' {
        viewpoint 'to Do Viewpoint' {
            subject;
        }
        satisfy 'to Do Viewpoint';
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
            metadata MigrationMetadata::SynthesizedName about final;
        }
        perform action 'to Do Method' : 'To Do Method';
        metadata MigrationMetadata::SynthesizedName about 'to Do Viewpoint';
    }
    view def 'Documented Diagrams Viewpoint' {
        viewpoint 'documented Diagrams Viewpoint' {
            subject;
        }
        satisfy 'documented Diagrams Viewpoint';
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
            metadata MigrationMetadata::SynthesizedName about final;
        }
        perform action 'documented Diagrams Method' : 'Documented Diagrams Method';
        metadata MigrationMetadata::SynthesizedName about 'documented Diagrams Viewpoint';
    }
    view def 'Reverse Documented Diagrams Viewpoint' {
        viewpoint 'reverse Documented Diagrams Viewpoint' {
            subject;
        }
        satisfy 'reverse Documented Diagrams Viewpoint';
        action def 'Reverse Documented Diagrams Method' {
            first start then 'Sort By Documentation';
            action 'Sort By Documentation';
            first 'Sort By Documentation' then 'Collect Owned Elements';
            action 'Collect Owned Elements';
            first 'Collect Owned Elements' then Image;
            action Image;
            first Image then final;
            action final terminate;
            metadata MigrationMetadata::SynthesizedName about final;
        }
        perform action 'reverse Documented Diagrams Method' : 'Reverse Documented Diagrams Method';
        metadata MigrationMetadata::SynthesizedName about 'reverse Documented Diagrams Viewpoint';
    }
    view def 'Diagrams By Documentation Viewpoint' {
        viewpoint 'diagrams By Documentation Viewpoint' {
            subject;
        }
        satisfy 'diagrams By Documentation Viewpoint';
        action def 'Diagrams By Documentation Method' {
            first start then 'Sort By Documentation';
            action 'Sort By Documentation';
            first 'Sort By Documentation' then Image;
            action Image;
            first Image then final;
            action final terminate;
            metadata MigrationMetadata::SynthesizedName about final;
        }
        perform action 'diagrams By Documentation Method' : 'Diagrams By Documentation Method';
        metadata MigrationMetadata::SynthesizedName about 'diagrams By Documentation Viewpoint';
    }
    view def 'Unnamed Drafts Viewpoint' {
        viewpoint 'unnamed Drafts Viewpoint' {
            subject;
        }
        satisfy 'unnamed Drafts Viewpoint';
        action def 'Unnamed Drafts Method' {
            first start then 'Collect Owned Elements';
            action 'Collect Owned Elements';
            first 'Collect Owned Elements' then 'Filter By Names';
            action 'Filter By Names';
            first 'Filter By Names' then 'Bulleted List';
            action 'Bulleted List';
            first 'Bulleted List' then 'Collect Owned Diagrams';
            action 'Collect Owned Diagrams';
            first 'Collect Owned Diagrams' then Image;
            action Image;
            first Image then final;
            action final terminate;
            metadata MigrationMetadata::SynthesizedName about final;
        }
        perform action 'unnamed Drafts Method' : 'Unnamed Drafts Method';
        metadata MigrationMetadata::SynthesizedName about 'unnamed Drafts Viewpoint';
    }
    view def 'Optional Drafts Viewpoint' {
        viewpoint 'optional Drafts Viewpoint' {
            subject;
        }
        satisfy 'optional Drafts Viewpoint';
        action def 'Optional Drafts Method' {
            first start then 'Collect Owned Elements';
            action 'Collect Owned Elements';
            first 'Collect Owned Elements' then 'Filter By Names';
            action 'Filter By Names';
            first 'Filter By Names' then 'Bulleted List';
            action 'Bulleted List';
            first 'Bulleted List' then 'Collect Owned Diagrams';
            action 'Collect Owned Diagrams';
            first 'Collect Owned Diagrams' then Image;
            action Image;
            first Image then final;
            action final terminate;
            metadata MigrationMetadata::SynthesizedName about final;
        }
        perform action 'optional Drafts Method' : 'Optional Drafts Method';
        metadata MigrationMetadata::SynthesizedName about 'optional Drafts Viewpoint';
    }
    view def 'Owner Sorted Diagrams Viewpoint' {
        viewpoint 'owner Sorted Diagrams Viewpoint' {
            subject;
        }
        satisfy 'owner Sorted Diagrams Viewpoint';
        action def 'Owner Sorted Diagrams Method' {
            first start then 'Sort By Owner';
            action 'Sort By Owner';
            first 'Sort By Owner' then Image;
            action Image;
            first Image then final;
            action final terminate;
            metadata MigrationMetadata::SynthesizedName about final;
        }
        perform action 'owner Sorted Diagrams Method' : 'Owner Sorted Diagrams Method';
        metadata MigrationMetadata::SynthesizedName about 'owner Sorted Diagrams Viewpoint';
    }
    view def 'Specification Viewpoint' {
        viewpoint 'specification Viewpoint' {
            subject;
        }
        satisfy 'specification Viewpoint';
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
            metadata MigrationMetadata::SynthesizedName about final;
        }
        perform action 'specification Method' : 'Specification Method';
        metadata MigrationMetadata::SynthesizedName about 'specification Viewpoint';
    }
}
package 'Yard Handbook' {
    part def 'Yard Handbook' {
        view blockDiagrams :> 'Block Diagrams';
        view sketchedDiagrams :> 'Sketched Diagrams';
        view retargetedDiagrams :> 'Retargeted Diagrams';
        view allDiagrams :> 'All Diagrams';
        view noDiagrams :> 'No Diagrams';
        view kinDiagrams :> 'Kin Diagrams';
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
        view unnamedDrafts :> 'Unnamed Drafts';
        view optionalDrafts :> 'Optional Drafts';
        /* applied stereotype «Document» */
    }
    view 'Block Diagrams' : 'Yard Viewpoints'::'Block Diagrams Viewpoint' {
        expose Structure::'Crane Structure';
        expose Structure::Crane::'Crane Internals';
        expose Needs::'Yard Requirements';
    }
    view 'Sketched Diagrams' : 'Yard Viewpoints'::'Sketched Diagrams Viewpoint' {
        expose Structure::'Crane Structure';
        expose Structure::Crane::'Crane Internals';
        expose Sketches::Sketch;
    }
    view 'Retargeted Diagrams' : 'Yard Viewpoints'::'Retargeted Diagrams Viewpoint' {
        expose Structure::'Crane Structure';
        expose Sketches::Sketch;
    }
    view 'All Diagrams' : 'Yard Viewpoints'::'All Diagrams Viewpoint' {
        expose Structure::'Crane Structure';
        expose Sketches::Sketch;
    }
    view 'No Diagrams' : 'Yard Viewpoints'::'No Diagrams Viewpoint' {
        expose Structure::'Crane Structure';
        expose Sketches::Sketch;
    }
    view 'Kin Diagrams' : 'Yard Viewpoints'::'Kin Diagrams Viewpoint' {
        expose Needs::Sway::'Sway Limits';
    }
    view 'Other Diagrams' : 'Yard Viewpoints'::'Other Diagrams Viewpoint' {
        expose Structure::'Crane Structure';
        expose Structure::Crane::'Crane Internals';
        expose Needs::'Yard Requirements';
    }
    view 'Internal Diagrams' : 'Yard Viewpoints'::'Internal Diagrams Viewpoint' {
        expose Structure::**;
    }
    view 'Parametric Diagrams' : 'Yard Viewpoints'::'Parametric Diagrams Viewpoint' {
        expose Structure::'Crane Structure';
    }
    view 'Shown On The Structure Diagram' : 'Yard Viewpoints'::'Shown Viewpoint' {
        expose Structure::'Crane Structure';
    }
    view 'Shown On Both Diagrams' : 'Yard Viewpoints'::'Shown Everywhere Viewpoint' {
        expose Structure::'Crane Structure';
        expose Sketches::Sketch;
    }
    view 'Drive Parts' : 'Yard Viewpoints'::'Drive Parts Viewpoint' {
        expose Structure::Gantry;
        expose Structure::Winch;
    }
    view 'Crane Parts' : 'Yard Viewpoints'::'Parts Viewpoint' {
        expose Structure::Crane;
    }
    view 'Direct Crane Parts' : 'Yard Viewpoints'::'Direct Parts Viewpoint' {
        expose Structure::Crane;
    }
    view 'Shared Crane Parts' : 'Yard Viewpoints'::'Shared Parts Viewpoint' {
        expose Structure::Crane;
    }
    view 'Cable Parts' : 'Yard Viewpoints'::'Parts Viewpoint' {
        expose Structure::Cable;
    }
    view 'To Do' : 'Yard Viewpoints'::'To Do Viewpoint' {
        expose Structure::**;
    }
    view Specification : 'Yard Viewpoints'::'Specification Viewpoint' {
        expose Needs::**;
    }
    view 'Documented Diagrams' : 'Yard Viewpoints'::'Documented Diagrams Viewpoint' {
        expose Structure::Gantry;
        expose Needs::**;
        expose Structure::Crane;
        expose Needs::Sway;
    }
    view 'Reverse Documented Diagrams' : 'Yard Viewpoints'::'Reverse Documented Diagrams Viewpoint' {
        expose Structure::Gantry;
        expose Structure::Crane;
        expose Needs::**;
        expose Needs::Sway;
    }
    view 'Diagrams By Documentation' : 'Yard Viewpoints'::'Diagrams By Documentation Viewpoint' {
        expose Structure::'Crane Structure';
        expose Structure::Gantry::'Gantry Drive';
    }
    view 'Owner Sorted Diagrams' : 'Yard Viewpoints'::'Owner Sorted Diagrams Viewpoint' {
        expose Structure::'Crane Structure';
    }
    view 'Unnamed Drafts' : 'Yard Viewpoints'::'Unnamed Drafts Viewpoint' {
        expose Drafts::**;
    }
    view 'Optional Drafts' : 'Yard Viewpoints'::'Optional Drafts Viewpoint' {
        expose Drafts::**;
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
    calc def 'Yard Handbook Bulleted List Rows' :> DocumentQueries::Query {
        DocumentQueries::Project(
            source = DocumentQueries::Named(qualifiedName = ("Drafts::unnamed", "Drafts::Sketchy")),
            properties = ("name"))
    }
    part def 'Yard Handbook Document' :> DocumentQueries::Document {
        attribute redefines title = "Yard Handbook";
        ref gantry : Structure::Gantry;
        ref sway : Needs::Sway;
        ref crane : Structure::Crane;
        ref unnamed : Drafts::unnamed;
        ref sketchy : Drafts::Sketchy;
        part 'Block Diagrams' : DocumentQueries::Section {
            attribute redefines title = "Block Diagrams";
            part diagram : DocumentQueries::Diagram {
                attribute redefines caption = "Crane Structure";
                ref redefines source = Structure::'Crane Structure';
            }
        }
        part 'Sketched Diagrams' : DocumentQueries::Section {
            attribute redefines title = "Sketched Diagrams";
            /* not migrated: «Image» CallBehaviorAction 'Image' — the diagrams it shows are not known: «FilterByDiagramType» Yard Viewpoints::Sketched Diagrams Viewpoint::Sketched Diagrams Method::Filter By Diagram Type keeps or drops the diagram 'Sketch', whose diagram type the archive does not record */
        }
        part 'Retargeted Diagrams' : DocumentQueries::Section {
            attribute redefines title = "Retargeted Diagrams";
            part diagram : DocumentQueries::Diagram {
                attribute redefines caption = "Crane Structure";
                ref redefines source = Structure::'Crane Structure';
            }
            part 'diagram 2' : DocumentQueries::Diagram {
                attribute redefines caption = "Gantry Drive";
                ref redefines source = gantry.'Gantry Drive';
            }
        }
        part 'All Diagrams' : DocumentQueries::Section {
            attribute redefines title = "All Diagrams";
            part diagram : DocumentQueries::Diagram {
                attribute redefines caption = "Crane Structure";
                ref redefines source = Structure::'Crane Structure';
            }
            /* not migrated: «Image» CallBehaviorAction 'Image' — the diagram of unknown kind 'Sketch' is a view rendered as textual notation, which a document does not draw */
        }
        part 'No Diagrams' : DocumentQueries::Section {
            attribute redefines title = "No Diagrams";
        }
        part 'Kin Diagrams' : DocumentQueries::Section {
            attribute redefines title = "Kin Diagrams";
            part diagram : DocumentQueries::Diagram {
                attribute redefines caption = "Sway Limits";
                ref redefines source = sway.'Sway Limits';
            }
            part 'diagram 2' : DocumentQueries::Diagram {
                attribute redefines caption = "Yard Requirements";
                ref redefines source = Needs::'Yard Requirements';
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
        part 'Unnamed Drafts' : DocumentQueries::Section {
            attribute redefines title = "Unnamed Drafts";
        }
        part 'Optional Drafts' : DocumentQueries::Section {
            attribute redefines title = "Optional Drafts";
            part list : DocumentQueries::List {
                attribute redefines style = "bullet";
                calc items : 'Yard Handbook Bulleted List Rows';
            }
            part diagram : DocumentQueries::Diagram {
                attribute redefines caption = "Overview";
                ref redefines source = unnamed.Overview;
            }
            part 'diagram 2' : DocumentQueries::Diagram {
                attribute redefines caption = "Sketchy Overview";
                ref redefines source = sketchy.'Sketchy Overview';
            }
        }
    }
}
