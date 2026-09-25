package 'Fleet Profile' {
    metadata def Safety;
}
package Fleet {
    package Structure {
        part def Truck {
            doc /* Hauls one trailer. */
            attribute payload : ScalarValues::Real;
            part axles : Axle[2];
            satisfy requirement : Fleet::Requirements::'Load Limit';
            view 'Truck Internals' {
                expose axles;
                render Views::asInterconnectionDiagram;
            }
        }
        part def Trailer {
            doc /* Carries the load. */
            attribute payload : ScalarValues::Real;
            satisfy requirement : Fleet::Requirements::'Brake Distance';
        }
        part def Axle;
        view 'Truck Structure' {
            expose Truck;
            expose Trailer;
            expose Axle;
            render Views::asTreeDiagram;
        }
    }
    package Requirements {
        requirement def <'R-1'> 'Load Limit' {
            doc /* The payload stays under the axle rating. */
            comment /* The payload stays under the axle rating. */
        }
        requirement def <'R-2'> 'Brake Distance' {
            doc /* A loaded truck stops within the legal distance. */
            comment /* A loaded truck stops within the legal distance. */
            @'Fleet Profile'::Safety;
        }
        requirement def <'R-3'> 'Axle Count' {
            doc /* A truck has two axles. */
            @'Fleet Profile'::Safety;
        }
    }
}
package 'Fleet Viewpoints' {
    viewpoint 'Parts Viewpoint' {
        doc /* List the parts of the fleet. */
        action def 'Parts Method' {
            view 'Parts Method Flow' : StandardViewDefinitions::ActionFlowView {
                expose 'Parts Method';
                render Views::asInterconnectionDiagram;
            }
            first start then 'Collect Owned Elements';
            action 'Collect Owned Elements' {
                out result[0..1];
            }
            first 'Collect Owned Elements' then 'Filter By Metaclasses';
            action 'Filter By Metaclasses' {
                in input[0..1];
            }
            first 'Filter By Metaclasses' then 'Sort By Name';
            action 'Sort By Name';
            first 'Sort By Name' then 'Fleet Parts';
            action 'Fleet Parts' {
                first start then Name;
                action Name;
                first Name then Payload;
                action Payload;
                first Payload then 'Qualified Name';
                action 'Qualified Name';
                first 'Qualified Name' then 'Owner Name';
                action 'Owner Name';
                first 'Owner Name' then Alias;
                action Alias;
                first Alias then done;
            }
            first 'Fleet Parts' then final;
            action final terminate;
            flow 'Collect Owned Elements'.result to 'Filter By Metaclasses'.input;
            metadata MigrationMetadata::SynthesizedName about final;
        }
        perform action 'parts Method' : 'Parts Method';
    }
    viewpoint 'Requirements Viewpoint' {
        doc /* List the requirements.
         * method: Fleet Viewpoints::Requirements Viewpoint::Requirements Method
         */
        action def 'Requirements Method' {
            first start then 'Collect Owned Elements';
            action 'Collect Owned Elements';
            first 'Collect Owned Elements' then 'Filter By Stereotypes';
            action 'Filter By Stereotypes';
            first 'Filter By Stereotypes' then 'Filter By Names';
            action 'Filter By Names';
            first 'Filter By Names' then Introduction;
            action Introduction;
            first Introduction then 'Requirement List';
            action 'Requirement List';
            first 'Requirement List' then 'Requirement Texts';
            action 'Requirement Texts';
            first 'Requirement Texts' then final;
            action final terminate;
            metadata MigrationMetadata::SynthesizedName about final;
        }
    }
    viewpoint 'Safety Viewpoint' {
        doc /* List the safety requirements. */
        abstract action def View;
        action 'view' : View;
        action def 'Safety Method' {
            first start then 'Collect Owned Elements';
            action 'Collect Owned Elements';
            first 'Collect Owned Elements' then 'Filter By Stereotypes';
            action 'Filter By Stereotypes';
            first 'Filter By Stereotypes' then 'Filter By Names';
            action 'Filter By Names';
            first 'Filter By Names' then 'Safety Requirements';
            action 'Safety Requirements' {
                first start then Name;
                action Name;
                first Name then Text;
                action Text;
                first Text then Owner;
                action Owner;
            }
            first 'Safety Requirements' then final;
            action final terminate;
            metadata MigrationMetadata::SynthesizedName about final;
        }
    }
    viewpoint 'Figures Viewpoint' {
        action def 'Figures Method' {
            first start then Image;
            action Image;
            first Image then final;
            action final terminate;
            metadata MigrationMetadata::SynthesizedName about final;
        }
        perform action 'figures Method' : 'Figures Method';
    }
    viewpoint 'Gallery Viewpoint' {
        action def 'Gallery Method' {
            first start then Image;
            action Image;
            first Image then final;
            action final terminate;
            metadata MigrationMetadata::SynthesizedName about final;
        }
        perform action 'gallery Method' : 'Gallery Method';
    }
    viewpoint 'Selected Figures Viewpoint' {
        action def 'Selected Figures Method' {
            first start then 'Truck Diagrams';
            action 'Truck Diagrams';
            first 'Truck Diagrams' then Sort;
            action Sort;
            first Sort then Image;
            action Image;
            first Image then final;
            action final terminate;
            metadata MigrationMetadata::SynthesizedName about final;
        }
        perform action 'selected Figures Method' : 'Selected Figures Method';
    }
    viewpoint 'Other Figures Viewpoint' {
        action def 'Other Figures Method' {
            first start then 'Not Truck Diagrams';
            action 'Not Truck Diagrams';
            first 'Not Truck Diagrams' then Image;
            action Image;
            first Image then final;
            action final terminate;
            metadata MigrationMetadata::SynthesizedName about final;
        }
        perform action 'other Figures Method' : 'Other Figures Method';
    }
    viewpoint 'Figure Owners Viewpoint' {
        action def 'Figure Owners Method' {
            first start then 'Collect Owners';
            action 'Collect Owners';
            first 'Collect Owners' then Sort;
            action Sort;
            first Sort then List;
            action List;
            first List then final;
            action final terminate;
            metadata MigrationMetadata::SynthesizedName about final;
        }
        perform action 'figure Owners Method' : 'Figure Owners Method';
    }
    viewpoint 'No Figures Viewpoint' {
        action def 'No Figures Method' {
            first start then 'Packages Only';
            action 'Packages Only';
            first 'Packages Only' then Image;
            action Image;
            first Image then final;
            action final terminate;
            metadata MigrationMetadata::SynthesizedName about final;
        }
        perform action 'no Figures Method' : 'No Figures Method';
    }
    viewpoint 'Traceability Viewpoint' {
        action def 'Traceability Method' {
            first start then 'fork';
            fork 'fork';
            first 'fork' then 'Collect Satisfiers';
            first 'fork' then 'Collect Owned Elements';
            action 'Collect Satisfiers';
            first 'Collect Satisfiers' then 'merge';
            action 'Collect Owned Elements';
            first 'Collect Owned Elements' then 'merge';
            merge 'merge';
            first 'merge' then 'Related Elements';
            action 'Related Elements';
            first 'Related Elements' then final;
            action final terminate;
            metadata MigrationMetadata::SynthesizedName about 'fork', 'merge', final;
        }
        perform action 'traceability Method' : 'Traceability Method';
    }
    viewpoint 'Oddities Viewpoint' {
        action def 'Oddities Method' {
            first start then 'Evaluated Paragraph';
            action 'Evaluated Paragraph';
            first 'Evaluated Paragraph' then Image;
            action Image;
            first Image then 'Collect Owned Elements';
            action 'Collect Owned Elements';
            first 'Collect Owned Elements' then 'Collect Types';
            action 'Collect Types';
            first 'Collect Types' then 'Type Table';
            action 'Type Table' {
                first start then Name;
                action Name;
            }
            first 'Type Table' then 'Per Truck';
            action 'Per Truck' : 'Fleet Viewpoints'::'Oddities Viewpoint'::'Per Truck';
            first 'Per Truck' then 'Missing Behavior';
            action 'Missing Behavior' {
                /* not migrated: CallBehaviorAction 'Missing Behavior' — 1 behavior reference(s) resolve to nothing in the document (_act_gone); the action calls no behavior */
            }
            first 'Missing Behavior' then final;
            action final terminate;
            metadata MigrationMetadata::SynthesizedName about final;
        }
        action def 'Per Truck' {
            first start then Paragraph;
            action Paragraph;
            first Paragraph then 'Per Truck Again';
            action 'Per Truck Again' : 'Per Truck';
            first 'Per Truck Again' then final;
            action final terminate;
            /* applied stereotype «Dynamic_View»: loop = true; title = Per Truck */
            metadata MigrationMetadata::SynthesizedName about final;
        }
        perform action 'oddities Method' : 'Oddities Method';
    }
    viewpoint 'Broken Viewpoint' {
        action def 'Broken Method' {
            first start then Paragraph;
            action Paragraph;
            first Paragraph then final;
            action final terminate;
            metadata MigrationMetadata::SynthesizedName about final;
        }
        perform action 'broken Method' : 'Broken Method';
    }
    viewpoint 'Severed Viewpoint' {
        action def 'Severed Method' {
            first start then Paragraph;
            action Paragraph;
        }
        perform action 'severed Method' : 'Severed Method';
    }
    viewpoint 'Headless Viewpoint' {
        doc /* method: _act_vanished */
    }
}
package 'Fleet Documents' {
    part def 'Fleet Handbook' {
        view introduction :> Introduction;
        view requirements :> Requirements;
        view figures :> Figures;
        view traceability :> Traceability;
        view oddities :> Oddities;
        view broken :> Broken;
        view severed :> Severed;
        view headless :> Headless;
        view notes :> Notes;
        attribute author : ScalarValues::String;
        /* applied stereotype «Document» */
    }
    part def 'Fleet Brief' {
        view figures :> Figures;
        view fleet :> 'Fleet Views'::Fleet;
        view gallery :> 'Fleet Views'::Gallery;
        view selected :> 'Fleet Views'::'Truck Figures';
        view others :> 'Fleet Views'::'Other Figures';
        view owners :> 'Fleet Views'::'Figure Owners';
        view noFigures :> 'Fleet Views'::'No Figures';
        /* applied stereotype «Document» */
    }
    view Introduction {
        satisfy 'Fleet Viewpoints'::'Parts Viewpoint';
        expose Fleet::Structure::**;
    }
    view Requirements {
        satisfy 'Fleet Viewpoints'::'Requirements Viewpoint';
        view safety :> Safety;
        expose Fleet::Requirements::**;
        expose Fleet::Requirements::**;
    }
    view Safety {
        satisfy 'Fleet Viewpoints'::'Safety Viewpoint';
        public import Fleet::*;
        /* applied stereotype «view» */
    }
    view Figures {
        satisfy 'Fleet Viewpoints'::'Figures Viewpoint';
        expose Fleet::Structure::'Truck Structure';
    }
    view Traceability {
        satisfy 'Fleet Viewpoints'::'Traceability Viewpoint';
        expose Fleet::Requirements::'Load Limit';
    }
    view Oddities {
        satisfy 'Fleet Viewpoints'::'Oddities Viewpoint';
        expose Fleet::Structure::Truck;
    }
    view Broken {
        satisfy 'Fleet Viewpoints'::'Broken Viewpoint';
        expose Fleet::**;
    }
    view Severed {
        satisfy 'Fleet Viewpoints'::'Severed Viewpoint';
        expose Fleet::**;
    }
    view Headless {
        satisfy 'Fleet Viewpoints'::'Headless Viewpoint';
    }
    view Notes {
        doc /* Second note. */
        comment /* First note. */
        comment /* Figure: the fleet at the depot */
        comment /* Named, not bodied. */
        /* not migrated: «Conform» Generalization (_gen_notes) — the viewpoint is not in the document; applied stereotypes «Conform» */
    }
    calc def 'Fleet Handbook Fleet Parts Rows' :> DocumentQueries::Query {
        DocumentQueries::Project(
            source = DocumentQueries::OrderBy(
                source = DocumentQueries::WhereType(
                    source = DocumentQueries::Descendants(
                        source = DocumentQueries::Named(qualifiedName = ("Fleet::Structure")),
                        maxDepth = 1),
                    type = ("PartDefinition", "RequirementDefinition", "ConstraintDefinition", "PortDefinition", "VerificationCaseDefinition", "ActionDefinition", "StateDefinition", "CalculationDefinition", "ViewUsage", "ViewpointUsage")),
                property = "name",
                direction = "ascending",
                missing = "last",
                multiple = "first"),
            properties = ("name", "qualifiedName", "documentation"),
            columns = (
                DocumentQueries::Column(name = "Payload", expression = Fleet::Structure::Truck::payload ?? ""),
                DocumentQueries::Column(name = "name 2", expression = Fleet::Structure::Truck::payload ?? "")))
    }
    calc def 'Fleet Handbook Requirement List Rows' :> DocumentQueries::Query {
        DocumentQueries::Project(
            source = DocumentQueries::WhereName(
                source = DocumentQueries::WhereType(
                    source = DocumentQueries::Descendants(
                        source = DocumentQueries::Named(qualifiedName = ("Fleet::Requirements"))),
                    type = ("RequirementDefinition")),
                operator = "matches",
                value = "^(?:Axle.*)$|^(?:Brake.*)$|^(?:Load.*)$"),
            properties = ("name"))
    }
    calc def 'Fleet Handbook Requirement Texts Rows' :> DocumentQueries::Query {
        DocumentQueries::Project(
            source = DocumentQueries::WhereName(
                source = DocumentQueries::WhereType(
                    source = DocumentQueries::Descendants(
                        source = DocumentQueries::Named(qualifiedName = ("Fleet::Requirements"))),
                    type = ("RequirementDefinition")),
                operator = "matches",
                value = "^(?:Axle.*)$|^(?:Brake.*)$|^(?:Load.*)$"),
            properties = ("documentation"))
    }
    calc def 'Fleet Handbook Safety Requirements Rows' :> DocumentQueries::Query {
        DocumentQueries::Project(
            source = DocumentQueries::Except(
                source = DocumentQueries::WhereMetadata(
                    source = DocumentQueries::Descendants(
                        source = DocumentQueries::Named(qualifiedName = ("Fleet"))),
                    'metadata' = ("Fleet Profile::Safety")),
                exclude = DocumentQueries::WhereName(
                    source = DocumentQueries::WhereMetadata(
                        source = DocumentQueries::Descendants(
                            source = DocumentQueries::Named(qualifiedName = ("Fleet"))),
                        'metadata' = ("Fleet Profile::Safety")),
                    operator = "matches",
                    value = "^(?:.*Count)$")),
            properties = ("name", "documentation"))
    }
    calc def 'Fleet Handbook Related Elements Rows' :> DocumentQueries::Query {
        DocumentQueries::Project(
            source = DocumentQueries::OrderBy(
                source = DocumentQueries::Union(
                    source = DocumentQueries::RelatedElements(
                        source = DocumentQueries::Named(qualifiedName = ("Fleet::Requirements::Load Limit")),
                        relationshipKind = "satisfaction",
                        direction = "incoming",
                        maxDepth = 1),
                    other = DocumentQueries::Descendants(
                        source = DocumentQueries::Named(qualifiedName = ("Fleet::Requirements::Load Limit")),
                        maxDepth = 1)),
                property = "name",
                direction = "ascending",
                missing = "last",
                multiple = "first"),
            properties = ("name", "documentation"))
    }
    part def 'Fleet Handbook Document' :> DocumentQueries::Document {
        attribute redefines title = "Fleet Handbook";
        part Introduction : DocumentQueries::Section {
            attribute redefines title = "Introduction";
            part table : DocumentQueries::Table {
                attribute redefines caption = "Fleet Parts";
                calc rows : 'Fleet Handbook Fleet Parts Rows';
            }
            part paragraph : DocumentQueries::Paragraph {
                attribute redefines text = "The parts of the fleet, by name.";
            }
        }
        part Requirements : DocumentQueries::Section {
            attribute redefines title = "Requirements";
            part paragraph : DocumentQueries::Paragraph {
                attribute redefines text = "Every truck of the fleet satisfies these requirements.";
            }
            part list : DocumentQueries::List {
                attribute redefines style = "number";
                calc items : 'Fleet Handbook Requirement List Rows';
            }
            part 'paragraph 2' : DocumentQueries::Paragraph {
                calc values : 'Fleet Handbook Requirement Texts Rows';
            }
            part Safety : DocumentQueries::Section {
                attribute redefines title = "Safety";
                part table : DocumentQueries::Table {
                    attribute redefines caption = "Safety Requirements";
                    calc rows : 'Fleet Handbook Safety Requirements Rows';
                }
            }
        }
        part Figures : DocumentQueries::Section {
            attribute redefines title = "Figures";
            part diagram : DocumentQueries::Diagram {
                attribute redefines caption = "Truck Structure";
                ref redefines source = Fleet::Structure::'Truck Structure';
            }
            part paragraph : DocumentQueries::Paragraph {
                attribute redefines text = "The truck and what it hauls";
            }
        }
        part Traceability : DocumentQueries::Section {
            attribute redefines title = "Traceability";
            part list : DocumentQueries::List {
                attribute redefines style = "bullet";
                calc items : 'Fleet Handbook Related Elements Rows';
            }
        }
        part Oddities : DocumentQueries::Section {
            attribute redefines title = "Oddities";
            /* not migrated: «Paragraph» CallBehaviorAction 'Evaluated Paragraph' — its body is evaluated as OCL, which no query evaluates */
            /* not migrated: «TableStructure» StructuredActivityNode 'Type Table' — the elements it shows pass through «CollectOwnedElements» Fleet Viewpoints::Oddities Viewpoint::Oddities Method::Collect Owned Elements is not migrated: the depth "many" is not a whole number */
            part 'Per Truck' : DocumentQueries::Section {
                attribute redefines title = "Per Truck";
                part paragraph : DocumentQueries::Paragraph {
                    attribute redefines text = "One truck.";
                }
                part 'Per Truck' : DocumentQueries::Section {
                    attribute redefines title = "Per Truck";
                    /* not migrated: the «Dynamic_View» Activity Fleet Viewpoints::Oddities Viewpoint::Per Truck calls itself, and a recursive section has no static spelling */
                }
            }
        }
        part Broken : DocumentQueries::Section {
            attribute redefines title = "Broken";
            /* not migrated: the method Fleet Viewpoints::Broken Viewpoint::Broken Method is not migrated: no initial node */
        }
        part Severed : DocumentQueries::Section {
            attribute redefines title = "Severed";
            /* not migrated: the method Fleet Viewpoints::Severed Viewpoint::Severed Method is not migrated: ControlFlow _severed_e2's target "_severed_gone" names no node */
        }
        part Headless : DocumentQueries::Section {
            attribute redefines title = "Headless";
            /* not migrated: the viewpoint Fleet Viewpoints::Headless Viewpoint's method is not migrated: method "_act_vanished" names no element */
        }
        part Notes : DocumentQueries::Section {
            attribute redefines title = "Notes";
            part paragraph : DocumentQueries::Paragraph {
                attribute redefines text = "First note.";
            }
            part 'paragraph 2' : DocumentQueries::Paragraph {
                attribute redefines text = "Second note.";
            }
            /* not migrated: «Image Paragraph» Comment '<Comment>' — the attached image "fleet.png" is not in the archive */
            /* not migrated: «Image Paragraph» Comment '<Comment>' — the attached image "depot.png" is not in the archive */
            /* not migrated: «Paragraph» Comment '<Comment>' — property "META:QPROP:Element:name" is not the comment body */
            /* not migrated: «Paragraph» Comment '<Comment>' — the paragraph's comment has no body */
        }
    }
    calc def 'Fleet Brief List Rows' :> DocumentQueries::Query {
        DocumentQueries::Project(
            source = DocumentQueries::OrderBy(
                source = DocumentQueries::Named(qualifiedName = ("Fleet::Structure::Truck", "Fleet::Structure")),
                property = "name",
                direction = "ascending",
                missing = "last",
                multiple = "first"),
            properties = ("name"))
    }
    part def 'Fleet Brief Document' :> DocumentQueries::Document {
        attribute redefines title = "Fleet Brief";
        ref 'parts Method' : 'Fleet Viewpoints'::'Parts Viewpoint'::'Parts Method';
        ref truck : $::Fleet::Structure::Truck;
        part Figures : DocumentQueries::Section {
            attribute redefines title = "Figures";
            part diagram : DocumentQueries::Diagram {
                attribute redefines caption = "Truck Structure";
                ref redefines source = $::Fleet::Structure::'Truck Structure';
            }
            part paragraph : DocumentQueries::Paragraph {
                attribute redefines text = "The truck and what it hauls";
            }
        }
        part Fleet : DocumentQueries::Section {
            attribute redefines title = "Fleet";
            part diagram : DocumentQueries::Diagram {
                attribute redefines caption = "Truck Structure";
                ref redefines source = $::Fleet::Structure::'Truck Structure';
            }
            part paragraph : DocumentQueries::Paragraph {
                attribute redefines text = "The truck and what it hauls";
            }
            part 'diagram 2' : DocumentQueries::Diagram {
                attribute redefines caption = "Parts Method Flow";
                ref redefines source = 'parts Method'.'Parts Method Flow';
            }
            part 'diagram 3' : DocumentQueries::Diagram {
                attribute redefines caption = "Truck Internals";
                ref redefines source = truck.'Truck Internals';
            }
            part 'diagram 4' : DocumentQueries::Diagram {
                attribute redefines caption = "Fleet Overview";
                ref redefines source = 'Fleet Views'::Fleet.'Fleet Overview';
            }
        }
        part Gallery : DocumentQueries::Section {
            attribute redefines title = "Gallery";
            part diagram : DocumentQueries::Diagram {
                attribute redefines caption = "Figure: Inside the truck";
                ref redefines source = truck.'Truck Internals';
            }
        }
        part 'Truck Figures' : DocumentQueries::Section {
            attribute redefines title = "Truck Figures";
            part diagram : DocumentQueries::Diagram {
                attribute redefines caption = "Truck Internals";
                ref redefines source = truck.'Truck Internals';
            }
            part 'diagram 2' : DocumentQueries::Diagram {
                attribute redefines caption = "Truck Structure";
                ref redefines source = $::Fleet::Structure::'Truck Structure';
            }
        }
        part 'Other Figures' : DocumentQueries::Section {
            attribute redefines title = "Other Figures";
            part diagram : DocumentQueries::Diagram {
                attribute redefines caption = "Fleet Overview";
                ref redefines source = 'Fleet Views'::Fleet.'Fleet Overview';
            }
        }
        part 'Figure Owners' : DocumentQueries::Section {
            attribute redefines title = "Figure Owners";
            part list : DocumentQueries::List {
                attribute redefines style = "bullet";
                calc items : 'Fleet Brief List Rows';
            }
        }
        part 'No Figures' : DocumentQueries::Section {
            attribute redefines title = "No Figures";
        }
    }
}
package 'Fleet Views' {
    view Fleet {
        satisfy 'Fleet Viewpoints'::'Figures Viewpoint';
        expose $::Fleet::Structure::'Truck Structure';
        expose 'Fleet Viewpoints'::'Parts Viewpoint'::'Parts Method'::'Parts Method Flow';
        expose $::Fleet::Structure::Truck::'Truck Internals';
        expose 'Fleet Overview';
        view 'Fleet Overview' {
            expose $::Fleet::Structure;
            expose $::Fleet::Requirements;
            render Views::asTreeDiagram;
        }
    }
    view Gallery {
        satisfy 'Fleet Viewpoints'::'Gallery Viewpoint';
        expose $::Fleet::Structure::Truck::'Truck Internals';
    }
    view 'Truck Figures' {
        satisfy 'Fleet Viewpoints'::'Selected Figures Viewpoint';
        expose $::Fleet::Structure::'Truck Structure';
        expose 'Fleet Viewpoints'::'Parts Viewpoint'::'Parts Method'::'Parts Method Flow';
        expose $::Fleet::Structure::Truck::'Truck Internals';
        expose Fleet::'Fleet Overview';
        expose $::Fleet::Structure::'Truck Structure';
    }
    view 'Other Figures' {
        satisfy 'Fleet Viewpoints'::'Other Figures Viewpoint';
        expose $::Fleet::Structure::'Truck Structure';
        expose 'Fleet Viewpoints'::'Parts Viewpoint'::'Parts Method'::'Parts Method Flow';
        expose $::Fleet::Structure::Truck::'Truck Internals';
        expose Fleet::'Fleet Overview';
    }
    view 'Figure Owners' {
        satisfy 'Fleet Viewpoints'::'Figure Owners Viewpoint';
        expose $::Fleet::Structure::Truck::'Truck Internals';
        expose $::Fleet::Structure::'Truck Structure';
    }
    view 'No Figures' {
        satisfy 'Fleet Viewpoints'::'No Figures Viewpoint';
        expose $::Fleet::Structure::'Truck Structure';
    }
}
