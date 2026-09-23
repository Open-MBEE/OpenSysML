package Traceability {
    metadata def Fulfil;
}
package Plant {
    package Structure {
        part def Pump {
            satisfy requirement : Plant::Requirements::FlowRequirement;
        }
        part def Valve {
            satisfy requirement : Plant::Requirements::SealRequirement {
                @Traceability::Fulfil;
            }
        }
        view 'Valve Requirement Map' {
            expose Valve;
            expose 'Valve Requirement Map Document';
            render Views::asTreeDiagram;
        }
        calc def 'Valve Requirement Map Rows' :> DocumentQueries::Query {
            DocumentQueries::Project(
                source = DocumentQueries::WhereType(
                    source = DocumentQueries::RelatedElements(
                        source = DocumentQueries::Named(qualifiedName = ("Plant::Structure::Valve")),
                        relationshipKind = "satisfaction",
                        direction = "outgoing",
                        maxDepth = 1),
                    type = ("PartDefinition", "RequirementDefinition", "ConstraintDefinition", "PortDefinition", "VerificationCaseDefinition", "ActionDefinition", "StateDefinition", "CalculationDefinition", "ViewUsage", "ViewpointUsage")),
                properties = ("qualifiedName", "@type"))
        }
        part def 'Valve Requirement Map Document' :> DocumentQueries::Document {
            attribute redefines title = "Valve Requirement Map";
            part rows : DocumentQueries::Table {
                attribute redefines caption = "Valve Requirement Map";
                calc rows : 'Valve Requirement Map Rows';
            }
        }
    }
    package Requirements {
        requirement def <'R-1'> FlowRequirement {
            doc /* The pump keeps the flow above the minimum. */
        }
        requirement def <'R-2'> SealRequirement {
            doc /* Every valve seals when closed. */
        }
        connection def 'Derive SealRequirement' :> RequirementDerivation::Derivation {
            end #RequirementDerivation::original originalRequirement : FlowRequirement;
            end #RequirementDerivation::derive derivedRequirement : SealRequirement;
        }
        view 'Exact Satisfaction Matrix' {
            expose FlowRequirement;
            expose Plant::Structure::Pump;
            expose 'Exact Satisfaction Matrix Document';
            render Views::asElementTable;
        }
        calc def 'Exact Satisfaction Matrix Rows' :> DocumentQueries::Query {
            DocumentQueries::Project(
                source = DocumentQueries::WhereType(
                    source = DocumentQueries::Descendants(
                        source = DocumentQueries::Named(qualifiedName = ("Plant::Requirements"))),
                    type = ("RequirementDefinition")),
                properties = ("name"),
                columns = (
                    DocumentQueries::RelatedColumn(
                        name = "Satisfied by",
                        relationshipKind = "satisfaction",
                        direction = "incoming",
                        maxDepth = 1,
                        aggregate = "list",
                        targets = DocumentQueries::WhereType(
                            source = DocumentQueries::Descendants(
                                source = DocumentQueries::Named(qualifiedName = ("Plant::Structure"))),
                            type = ("PartDefinition")))))
        }
        part def 'Exact Satisfaction Matrix Document' :> DocumentQueries::Document {
            attribute redefines title = "Exact Satisfaction Matrix";
            part rows : DocumentQueries::Table {
                attribute redefines caption = "Exact Satisfaction Matrix";
                calc rows : 'Exact Satisfaction Matrix Rows';
            }
        }
        view 'Wide Satisfaction Matrix' {
            expose FlowRequirement;
            expose SealRequirement;
            expose Plant::Structure::Pump;
            expose Plant::Structure::Valve;
            expose 'Wide Satisfaction Matrix Document';
            render Views::asElementTable;
        }
        calc def 'Wide Satisfaction Matrix Rows' :> DocumentQueries::Query {
            DocumentQueries::Project(
                source = DocumentQueries::WhereType(
                    source = DocumentQueries::Descendants(
                        source = DocumentQueries::Named(qualifiedName = ("Plant::Requirements"))),
                    type = ("RequirementDefinition")),
                properties = ("name"),
                columns = (
                    DocumentQueries::RelatedColumn(
                        name = "Satisfied by",
                        relationshipKind = "satisfaction",
                        direction = "incoming",
                        maxDepth = 1,
                        aggregate = "list",
                        targets = DocumentQueries::WhereType(
                            source = DocumentQueries::Descendants(
                                source = DocumentQueries::Named(qualifiedName = ("Plant"))),
                            type = ("PartDefinition", "RequirementDefinition"))),
                    DocumentQueries::RelatedColumn(
                        name = "Derived by",
                        relationshipKind = "derivation",
                        direction = "outgoing",
                        maxDepth = 1,
                        aggregate = "list",
                        targets = DocumentQueries::WhereType(
                            source = DocumentQueries::Descendants(
                                source = DocumentQueries::Named(qualifiedName = ("Plant"))),
                            type = ("PartDefinition", "RequirementDefinition")))))
        }
        part def 'Wide Satisfaction Matrix Document' :> DocumentQueries::Document {
            attribute redefines title = "Wide Satisfaction Matrix";
            part rows : DocumentQueries::Table {
                attribute redefines caption = "Wide Satisfaction Matrix";
                calc rows : 'Wide Satisfaction Matrix Rows';
            }
        }
        view 'Seal Derivation Map' {
            expose SealRequirement;
            expose 'Seal Derivation Map Document';
            render Views::asTreeDiagram;
        }
        calc def 'Seal Derivation Map Rows' :> DocumentQueries::Query {
            DocumentQueries::Project(
                source = DocumentQueries::WhereType(
                    source = DocumentQueries::RelatedElements(
                        source = DocumentQueries::Named(qualifiedName = ("Plant::Requirements::SealRequirement")),
                        relationshipKind = "derivation",
                        direction = "incoming",
                        maxDepth = 1),
                    type = ("PartDefinition", "RequirementDefinition", "ConstraintDefinition", "PortDefinition", "VerificationCaseDefinition", "ActionDefinition", "StateDefinition", "CalculationDefinition", "ViewUsage", "ViewpointUsage")),
                properties = ("qualifiedName", "@type"))
        }
        part def 'Seal Derivation Map Document' :> DocumentQueries::Document {
            attribute redefines title = "Seal Derivation Map";
            part rows : DocumentQueries::Table {
                attribute redefines caption = "Seal Derivation Map";
                calc rows : 'Seal Derivation Map Rows';
            }
        }
    }
}
