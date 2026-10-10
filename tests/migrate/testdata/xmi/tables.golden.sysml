package 'Plant Profile' {
    metadata def Critical {
        attribute level : ScalarValues::Integer;
        attribute grade : Grade[0..*];
    }
    metadata def Urgent :> Critical;
    enum def Grade {
        high;
        low;
    }
    /* applied stereotype «auxiliaryResource» */
}
package Plant {
    package Structure {
        part def Pump {
            doc /* Moves fluid through the plant. */
            attribute mass : ScalarValues::Real;
            attribute 'flow' : ScalarValues::Real;
            attribute name : ScalarValues::String;
            abstract action def prime;
            action 'prime 2' : prime;
            satisfy Plant::Requirements::FlowRequirement;
            @'Plant Profile'::Critical {
                level = 2;
            }
        }
        part def ReservePump :> Pump;
        part def Valve {
            attribute mass : ScalarValues::Real;
            satisfy Plant::Requirements::SealRequirement;
        }
        part def Station {
            part pumps : Pump[1..*];
        }
        view 'Pump Requirement Map' {
            expose Pump;
            expose Plant::Requirements::FlowRequirement;
            expose 'Pump Requirement Map Document';
            render Views::asTreeDiagram;
        }
        calc def 'Pump Requirement Map Rows' :> DocumentQueries::Query {
            in candidates : KerML::Root::Element[0..*] ordered = DocumentQueries::RelatedElements(
                source = DocumentQueries::Named(qualifiedName = ("Plant::Structure::Pump")),
                relationshipKind = "satisfaction",
                direction = "outgoing",
                maxDepth = 2);
            DocumentQueries::Project(
                source = DocumentQueries::Union(
                    source = DocumentQueries::Except(
                        source = DocumentQueries::WhereType(source = candidates, type = ("PartDefinition", "ConstraintDefinition", "PortDefinition", "VerificationCaseDefinition", "ActionDefinition", "StateDefinition", "CalculationDefinition", "ViewUsage", "ViewpointUsage", "OccurrenceDefinition")),
                        exclude = DocumentQueries::Except(
                            source = DocumentQueries::WhereType(source = candidates, type = ("ItemDefinition")),
                            exclude = DocumentQueries::WhereType(source = candidates, type = ("PartDefinition", "ConstraintDefinition", "PortDefinition", "VerificationCaseDefinition", "ActionDefinition", "StateDefinition", "CalculationDefinition", "ViewUsage", "ViewpointUsage")))),
                    other = DocumentQueries::Except(
                        source = DocumentQueries::Named(qualifiedName = ("Plant::Requirements::FlowRequirement", "Plant::Requirements::SealRequirement", "Plant::Requirements::MassRequirement")),
                        exclude = DocumentQueries::Except(
                            source = DocumentQueries::Named(qualifiedName = ("Plant::Requirements::FlowRequirement", "Plant::Requirements::SealRequirement", "Plant::Requirements::MassRequirement")),
                            exclude = candidates))),
                properties = ("qualifiedName", "@type"))
        }
        part def 'Pump Requirement Map Document' :> DocumentQueries::Document {
            attribute redefines title = "Pump Requirement Map";
            part rows : DocumentQueries::Table {
                attribute redefines caption = "Pump Requirement Map";
                calc rows : 'Pump Requirement Map Rows';
            }
        }
    }
    package Inventory {
        part p1 : Plant::Structure::Pump {
            attribute :>> mass = 12.5;
            attribute :>> 'flow' = 3.0;
            attribute :>> name = "primary";
        }
        part p2 : Plant::Structure::Pump {
            attribute :>> mass = 9.0;
        }
        part r1 : Plant::Structure::ReservePump {
            attribute :>> mass = 14.0;
        }
        part st1 : Plant::Structure::Station {
            part :>> pumps = p1;
        }
        part v1 : Plant::Structure::Valve {
            attribute :>> mass = 2.0;
        }
        view 'Pump Table' {
            expose p1;
            expose p2;
            expose r1;
            expose 'Pump Table Document';
            render Views::asElementTable;
        }
        calc def 'Pump Table Rows' :> DocumentQueries::Query {
            DocumentQueries::Project(
                source = DocumentQueries::Tree(
                    source = DocumentQueries::OrderBy(
                        source = DocumentQueries::Except(
                            source = DocumentQueries::Except(
                                source = DocumentQueries::WhereType(
                                    source = DocumentQueries::WhereType(
                                        source = DocumentQueries::Union(
                                            source = DocumentQueries::Named(qualifiedName = ("Plant::Spares::s1", "Plant::Spares::s2")),
                                            other = DocumentQueries::Descendants(
                                                source = DocumentQueries::Named(qualifiedName = ("Plant::Inventory")))),
                                        type = ("Plant::Structure::Pump")),
                                    type = ("Usage")),
                                exclude = DocumentQueries::Descendants(
                                    source = DocumentQueries::WhereType(
                                        source = DocumentQueries::Descendants(
                                            source = DocumentQueries::Union(
                                                source = DocumentQueries::Named(qualifiedName = ("Plant::Inventory")),
                                                other = DocumentQueries::WhereType(
                                                    source = DocumentQueries::Union(
                                                        source = DocumentQueries::Named(qualifiedName = ("Plant::Spares::s1", "Plant::Spares::s2")),
                                                        other = DocumentQueries::Descendants(
                                                            source = DocumentQueries::Named(qualifiedName = ("Plant::Inventory")))),
                                                    type = ("Package"))),
                                            maxDepth = 1),
                                        type = ("Type")))),
                            exclude = DocumentQueries::Named(qualifiedName = ("Plant::Inventory::p2"))),
                        property = "mass",
                        direction = "descending",
                        missing = "last",
                        multiple = "first")),
                properties = ("name", "general"),
                columns = (
                    DocumentQueries::Column(name = "mass", cell = { in row : Plant::Structure::Pump; row.mass ?? "" }),
                    DocumentQueries::Column(name = "flow", cell = { in row : Plant::Structure::Pump; row.'flow' ?? "" })))
        }
        part def 'Pump Table Document' :> DocumentQueries::Document {
            attribute redefines title = "Pump Table";
            part rows : DocumentQueries::Table {
                attribute redefines caption = "Pump Table";
                attribute redefines columnWidths = (200, 0, 0, 0);
                attribute redefines columnLabels = ("", "classifier");
                calc rows : 'Pump Table Rows';
            }
        }
        view 'Pump Ledger' {
            expose p1;
            expose p2;
            expose 'Pump Ledger Document';
            render Views::asElementTable;
        }
        calc def 'Pump Ledger Rows' :> DocumentQueries::Query {
            DocumentQueries::Project(
                source = DocumentQueries::OrderBy(
                    source = DocumentQueries::Except(
                        source = DocumentQueries::WhereType(
                            source = DocumentQueries::WhereType(
                                source = DocumentQueries::Descendants(
                                    source = DocumentQueries::Named(qualifiedName = ("Plant::Inventory"))),
                                type = ("Plant::Structure::Pump")),
                            type = ("Usage")),
                        exclude = DocumentQueries::Descendants(
                            source = DocumentQueries::WhereType(
                                source = DocumentQueries::Descendants(
                                    source = DocumentQueries::Union(
                                        source = DocumentQueries::Named(qualifiedName = ("Plant::Inventory")),
                                        other = DocumentQueries::WhereType(
                                            source = DocumentQueries::Descendants(
                                                source = DocumentQueries::Named(qualifiedName = ("Plant::Inventory"))),
                                            type = ("Package"))),
                                    maxDepth = 1),
                                type = ("Type")))),
                    property = "name",
                    direction = "ascending",
                    missing = "last",
                    multiple = "first"),
                properties = ("name", "qualifiedName"),
                columns = (
                    DocumentQueries::Column(name = "name 2", cell = { in row : Plant::Structure::Pump; row.name ?? "" }),
                    DocumentQueries::Column(name = "mass", cell = { in row : Plant::Structure::Pump; row.mass ?? "" })))
        }
        part def 'Pump Ledger Document' :> DocumentQueries::Document {
            attribute redefines title = "Pump Ledger";
            part rows : DocumentQueries::Table {
                attribute redefines caption = "Pump Ledger";
                attribute redefines columnLabels = ("", "", "name");
                calc rows : 'Pump Ledger Rows';
            }
        }
    }
    package Spares {
        part s1 : Plant::Structure::Pump {
            attribute :>> mass = 7.0;
        }
        part s2 : Plant::Structure::Pump;
    }
    package Requirements {
        requirement <'R-1'> FlowRequirement {
            doc /* The pump keeps the flow above the minimum. */
            comment /* The pump keeps the flow up. */
            @'Plant Profile'::Critical {
                grade = ('Plant Profile'::Grade::high, 'Plant Profile'::Grade::low);
                level = 1;
            }
        }
        requirement <'R-2'> SealRequirement {
            doc /* Every valve seals when closed. */
            comment /* Every valve seals when closed. */
            @'Plant Profile'::Urgent {
                level = 3;
            }
        }
        requirement <'R-3'> MassRequirement {
            doc /* The pump mass stays under the limit. */
        }
        connection def 'Derive MassRequirement' :> RequirementDerivation::Derivation {
            end #RequirementDerivation::original originalRequirement :> FlowRequirement;
            end #RequirementDerivation::derive derivedRequirement :> MassRequirement;
        }
        view 'Requirement Table' {
            expose FlowRequirement;
            expose SealRequirement;
            expose MassRequirement;
            expose 'Requirement Table Document';
            render Views::asElementTable;
        }
        calc def 'Requirement Table Rows' :> DocumentQueries::Query {
            in candidates : KerML::Root::Element[0..*] ordered = DocumentQueries::Union(
                source = DocumentQueries::Named(qualifiedName = ("Plant Profile", "Plant")),
                other = DocumentQueries::Descendants(
                    source = DocumentQueries::Named(qualifiedName = ("Plant Profile", "Plant"))));
            DocumentQueries::Project(
                source = DocumentQueries::OrderBy(
                    source = DocumentQueries::Except(
                        source = DocumentQueries::Named(qualifiedName = ("Plant::Requirements::FlowRequirement", "Plant::Requirements::SealRequirement", "Plant::Requirements::MassRequirement")),
                        exclude = DocumentQueries::Except(
                            source = DocumentQueries::Named(qualifiedName = ("Plant::Requirements::FlowRequirement", "Plant::Requirements::SealRequirement", "Plant::Requirements::MassRequirement")),
                            exclude = candidates)),
                    property = "name",
                    direction = "ascending",
                    missing = "last",
                    multiple = "first"),
                properties = ("name", "documentation"))
        }
        part def 'Requirement Table Document' :> DocumentQueries::Document {
            attribute redefines title = "Requirement Table";
            part rows : DocumentQueries::Table {
                attribute redefines caption = "Requirement Table";
                calc rows : 'Requirement Table Rows';
            }
        }
        view 'Key Requirements' {
            expose FlowRequirement;
            expose SealRequirement;
            expose MassRequirement;
            expose 'Key Requirements Document';
            render Views::asElementTable;
        }
        calc def 'Key Requirements Rows' :> DocumentQueries::Query {
            in candidates : KerML::Root::Element[0..*] ordered = DocumentQueries::Union(
                source = DocumentQueries::Named(qualifiedName = ("Plant::Requirements::FlowRequirement", "Plant::Requirements::SealRequirement", "Plant::Requirements::MassRequirement")),
                other = DocumentQueries::Descendants(
                    source = DocumentQueries::Named(qualifiedName = ("Plant::Requirements"))));
            DocumentQueries::WhereText(
                source = DocumentQueries::Project(
                    source = DocumentQueries::OrderBy(
                        source = DocumentQueries::Except(
                            source = DocumentQueries::Named(qualifiedName = ("Plant::Requirements::FlowRequirement", "Plant::Requirements::SealRequirement", "Plant::Requirements::MassRequirement")),
                            exclude = DocumentQueries::Except(
                                source = DocumentQueries::Named(qualifiedName = ("Plant::Requirements::FlowRequirement", "Plant::Requirements::SealRequirement", "Plant::Requirements::MassRequirement")),
                                exclude = candidates)),
                        property = "shortName",
                        direction = "ascending",
                        missing = "last",
                        multiple = "first"),
                    properties = ("shortName", "name", "documentation"),
                    columns = (
                        DocumentQueries::Column(name = "level", cell = { in row : 'Plant Profile'::Critical; row.level ?? "" }),
                        DocumentQueries::Column(name = "level 2", cell = { in row : 'Plant Profile'::Critical; row.level ?? "" }),
                        DocumentQueries::Column(name = "grade", cell = { in row : 'Plant Profile'::Critical; row.grade ?? "" }),
                        DocumentQueries::Column(name = "level 3", cell = { in row : 'Plant Profile'::Critical; row.level ?? "" }))),
                columns = ("level 2"),
                operator = "matches",
                value = "(?i)1.*")
        }
        part def 'Key Requirements Document' :> DocumentQueries::Document {
            attribute redefines title = "Key Requirements";
            part rows : DocumentQueries::Table {
                attribute redefines caption = "Key Requirements";
                attribute redefines columnWidths = (60, 0, 400, 80, 0, 0, 0);
                attribute redefines columnLabels = ("Id", "", "Text", "", "level", "", "level");
                calc rows : 'Key Requirements Rows';
            }
        }
        view 'Badly Filtered Requirements' {
            expose FlowRequirement;
            expose SealRequirement;
            expose MassRequirement;
            expose 'Badly Filtered Requirements Document';
            render Views::asElementTable;
        }
        calc def 'Badly Filtered Requirements Rows' :> DocumentQueries::Query {
            in candidates : KerML::Root::Element[0..*] ordered = DocumentQueries::Descendants(
                source = DocumentQueries::Named(qualifiedName = ("Plant::Requirements")));
            DocumentQueries::Project(
                source = DocumentQueries::OrderBy(
                    source = DocumentQueries::Except(
                        source = DocumentQueries::Named(qualifiedName = ("Plant::Requirements::FlowRequirement", "Plant::Requirements::SealRequirement", "Plant::Requirements::MassRequirement")),
                        exclude = DocumentQueries::Except(
                            source = DocumentQueries::Named(qualifiedName = ("Plant::Requirements::FlowRequirement", "Plant::Requirements::SealRequirement", "Plant::Requirements::MassRequirement")),
                            exclude = candidates)),
                    property = "shortName",
                    direction = "ascending",
                    missing = "last",
                    multiple = "first"),
                properties = ("shortName", "name"))
        }
        part def 'Badly Filtered Requirements Document' :> DocumentQueries::Document {
            attribute redefines title = "Badly Filtered Requirements";
            part rows : DocumentQueries::Table {
                attribute redefines caption = "Badly Filtered Requirements";
                attribute redefines columnLabels = ("Id");
                calc rows : 'Badly Filtered Requirements Rows';
            }
        }
        view 'Satisfaction Matrix' {
            expose FlowRequirement;
            expose SealRequirement;
            expose Plant::Structure::Pump;
            expose Plant::Structure::Valve;
            expose 'Satisfaction Matrix Document';
            render Views::asElementTable;
        }
        calc def 'Satisfaction Matrix Rows' :> DocumentQueries::Query {
            in candidates : KerML::Root::Element[0..*] ordered = DocumentQueries::Descendants(
                source = DocumentQueries::Named(qualifiedName = ("Plant::Requirements")));
            in candidates2 : KerML::Root::Element[0..*] ordered = DocumentQueries::Descendants(
                source = DocumentQueries::Named(qualifiedName = ("Plant::Structure")));
            DocumentQueries::Project(
                source = DocumentQueries::Except(
                    source = DocumentQueries::Named(qualifiedName = ("Plant::Requirements::FlowRequirement", "Plant::Requirements::SealRequirement", "Plant::Requirements::MassRequirement")),
                    exclude = DocumentQueries::Except(
                        source = DocumentQueries::Named(qualifiedName = ("Plant::Requirements::FlowRequirement", "Plant::Requirements::SealRequirement", "Plant::Requirements::MassRequirement")),
                        exclude = candidates)),
                properties = ("name"),
                columns = (
                    DocumentQueries::RelatedColumn(
                        name = "Trace",
                        relationshipKind = "satisfaction",
                        direction = "incoming",
                        maxDepth = 1,
                        aggregate = "list",
                        targets = DocumentQueries::Except(
                            source = DocumentQueries::WhereType(source = candidates2, type = ("PartDefinition", "OccurrenceDefinition")),
                            exclude = DocumentQueries::Except(
                                source = DocumentQueries::WhereType(source = candidates2, type = ("ItemDefinition")),
                                exclude = DocumentQueries::WhereType(source = candidates2, type = ("PartDefinition"))))),
                    DocumentQueries::RelatedColumn(
                        name = "Trace 2",
                        relationshipKind = "derivation",
                        direction = "outgoing",
                        maxDepth = 1,
                        aggregate = "list",
                        targets = DocumentQueries::Except(
                            source = DocumentQueries::WhereType(source = candidates2, type = ("PartDefinition", "OccurrenceDefinition")),
                            exclude = DocumentQueries::Except(
                                source = DocumentQueries::WhereType(source = candidates2, type = ("ItemDefinition")),
                                exclude = DocumentQueries::WhereType(source = candidates2, type = ("PartDefinition")))))))
        }
        part def 'Satisfaction Matrix Document' :> DocumentQueries::Document {
            attribute redefines title = "Satisfaction Matrix";
            part rows : DocumentQueries::Table {
                attribute redefines caption = "Satisfaction Matrix";
                calc rows : 'Satisfaction Matrix Rows';
            }
        }
        view 'Trace Matrix' {
            expose FlowRequirement;
            render Views::asElementTable;
        }
        /* not migrated: «DependencyMatrix» 'Trace Matrix' — the criterion Owner chain is a metaChainExpressionSpecification, which no relationship walk expresses */
    }
    view 'Critical Elements' {
        expose Plant::Structure::Pump;
        expose Plant::Requirements::FlowRequirement;
        expose 'Critical Elements Document';
        render Views::asElementTable;
    }
    calc def 'Critical Elements Rows' :> DocumentQueries::Query {
        DocumentQueries::Project(
            source = DocumentQueries::WhereMetadata(
                source = DocumentQueries::Descendants(
                    source = DocumentQueries::Named(qualifiedName = ("Plant"))),
                'metadata' = ("Plant Profile::Critical")),
            properties = ("qualifiedName"))
    }
    part def 'Critical Elements Document' :> DocumentQueries::Document {
        attribute redefines title = "Critical Elements";
        part rows : DocumentQueries::Table {
            attribute redefines caption = "Critical Elements";
            calc rows : 'Critical Elements Rows';
        }
    }
}
