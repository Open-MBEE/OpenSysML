package 'Plant Profile' {
    metadata def Critical {
        attribute level : ScalarValues::Integer;
    }
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
            satisfy requirement : Plant::Requirements::FlowRequirement;
            @'Plant Profile'::Critical {
                level = 2;
            }
        }
        part def ReservePump :> Pump;
        part def Valve {
            attribute mass : ScalarValues::Real;
            satisfy requirement : Plant::Requirements::SealRequirement;
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
            DocumentQueries::Project(
                source = DocumentQueries::WhereType(
                    source = DocumentQueries::RelatedElements(
                        source = DocumentQueries::Named(qualifiedName = ("Plant::Structure::Pump")),
                        relationshipKind = "satisfaction",
                        direction = "outgoing",
                        maxDepth = 2),
                    type = ("PartDefinition", "RequirementDefinition", "ConstraintDefinition", "PortDefinition", "VerificationCaseDefinition", "ActionDefinition", "StateDefinition", "CalculationDefinition", "ViewUsage", "ViewpointUsage")),
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
        individual part def p1 :> Plant::Structure::Pump {
            attribute :>> mass = 12.5;
            attribute :>> 'flow' = 3.0;
            attribute :>> name = "primary";
        }
        individual part def p2 :> Plant::Structure::Pump {
            attribute :>> mass = 9.0;
        }
        individual part def r1 :> Plant::Structure::ReservePump {
            attribute :>> mass = 14.0;
        }
        individual part def v1 :> Plant::Structure::Valve {
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
                source = DocumentQueries::OrderBy(
                    source = DocumentQueries::WhereFeature(
                        source = DocumentQueries::WhereType(
                            source = DocumentQueries::Union(
                                source = DocumentQueries::Descendants(
                                    source = DocumentQueries::Named(qualifiedName = ("Plant::Inventory"))),
                                other = DocumentQueries::Named(qualifiedName = ("Plant::Spares::s1", "Plant::Spares::s2"))),
                            type = ("Plant::Structure::Pump")),
                        'feature' = "isIndividual",
                        operator = "=",
                        value = "true"),
                    property = "mass",
                    direction = "descending",
                    missing = "last",
                    multiple = "first"),
                properties = ("name"),
                columns = (
                    DocumentQueries::Column(name = "mass", expression = Plant::Structure::Pump::mass ?? ""),
                    DocumentQueries::Column(name = "flow", expression = Plant::Structure::Pump::'flow' ?? "")))
        }
        part def 'Pump Table Document' :> DocumentQueries::Document {
            attribute redefines title = "Pump Table";
            part rows : DocumentQueries::Table {
                attribute redefines caption = "Pump Table";
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
                    source = DocumentQueries::WhereFeature(
                        source = DocumentQueries::WhereType(
                            source = DocumentQueries::Descendants(
                                source = DocumentQueries::Named(qualifiedName = ("Plant::Inventory"))),
                            type = ("Plant::Structure::Pump")),
                        'feature' = "isIndividual",
                        operator = "=",
                        value = "true"),
                    property = "name",
                    direction = "ascending",
                    missing = "last",
                    multiple = "first"),
                properties = ("name", "qualifiedName"),
                columns = (
                    DocumentQueries::Column(name = "name 2", expression = Plant::Structure::Pump::name ?? ""),
                    DocumentQueries::Column(name = "mass", expression = Plant::Structure::Pump::mass ?? "")))
        }
        part def 'Pump Ledger Document' :> DocumentQueries::Document {
            attribute redefines title = "Pump Ledger";
            part rows : DocumentQueries::Table {
                attribute redefines caption = "Pump Ledger";
                calc rows : 'Pump Ledger Rows';
            }
        }
    }
    package Spares {
        individual part def s1 :> Plant::Structure::Pump {
            attribute :>> mass = 7.0;
        }
        individual part def s2 :> Plant::Structure::Pump;
    }
    package Requirements {
        requirement def <'R-1'> FlowRequirement {
            doc /* The pump keeps the flow above the minimum. */
            comment /* The pump keeps the flow up. */
            @'Plant Profile'::Critical {
                level = 1;
            }
        }
        requirement def <'R-2'> SealRequirement {
            doc /* Every valve seals when closed. */
            comment /* Every valve seals when closed. */
        }
        requirement def <'R-3'> MassRequirement {
            doc /* The pump mass stays under the limit. */
        }
        connection def 'Derive MassRequirement' :> RequirementDerivation::Derivation {
            end #RequirementDerivation::original originalRequirement : FlowRequirement;
            end #RequirementDerivation::derive derivedRequirement : MassRequirement;
        }
        view 'Requirement Table' {
            expose FlowRequirement;
            expose SealRequirement;
            expose MassRequirement;
            expose 'Requirement Table Document';
            render Views::asElementTable;
        }
        calc def 'Requirement Table Rows' :> DocumentQueries::Query {
            DocumentQueries::Project(
                source = DocumentQueries::OrderBy(
                    source = DocumentQueries::WhereType(
                        source = DocumentQueries::Union(
                            source = DocumentQueries::Named(qualifiedName = ("Plant Profile", "Plant")),
                            other = DocumentQueries::Descendants(
                                source = DocumentQueries::Named(qualifiedName = ("Plant Profile", "Plant")))),
                        type = ("RequirementDefinition")),
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
        view 'Satisfaction Matrix' {
            expose FlowRequirement;
            expose SealRequirement;
            expose Plant::Structure::Pump;
            expose Plant::Structure::Valve;
            expose 'Satisfaction Matrix Document';
            render Views::asElementTable;
        }
        calc def 'Satisfaction Matrix Rows' :> DocumentQueries::Query {
            DocumentQueries::Project(
                source = DocumentQueries::WhereType(
                    source = DocumentQueries::Descendants(
                        source = DocumentQueries::Named(qualifiedName = ("Plant::Requirements"))),
                    type = ("RequirementDefinition")),
                properties = ("name"),
                columns = (
                    DocumentQueries::RelatedColumn(
                        name = "Trace",
                        relationshipKind = "satisfaction",
                        direction = "incoming",
                        maxDepth = 1,
                        aggregate = "list",
                        targets = DocumentQueries::WhereType(
                            source = DocumentQueries::Descendants(
                                source = DocumentQueries::Named(qualifiedName = ("Plant::Structure"))),
                            type = ("PartDefinition"))),
                    DocumentQueries::RelatedColumn(
                        name = "Trace 2",
                        relationshipKind = "derivation",
                        direction = "incoming",
                        maxDepth = 1,
                        aggregate = "list",
                        targets = DocumentQueries::WhereType(
                            source = DocumentQueries::Descendants(
                                source = DocumentQueries::Named(qualifiedName = ("Plant::Structure"))),
                            type = ("PartDefinition")))))
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
