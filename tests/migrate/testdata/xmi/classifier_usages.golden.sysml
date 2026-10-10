package Operations {
    part Operator;
    use case Scan {
        include Focus;
    }
    use case Focus;
    occurrence def Console {
        ref use case current :> Scan;
        ref part user :> Operator;
    }
    connection 'Scan includes Focus' connect Scan to Focus;
}
package Tables {
    view Classifiers {
        expose 'Classifiers Document';
        render Views::asElementTable;
    }
    calc def 'Classifiers Rows' :> DocumentQueries::Query {
        in candidates : KerML::Root::Element[0..*] ordered = DocumentQueries::Descendants(
            source = DocumentQueries::Named(qualifiedName = ("Operations")));
        DocumentQueries::Project(
            source = DocumentQueries::OrderBy(
                source = DocumentQueries::Union(
                    source = DocumentQueries::WhereType(source = candidates, type = ("Definition", "ViewUsage", "ViewpointUsage")),
                    other = DocumentQueries::Except(
                        source = DocumentQueries::Named(qualifiedName = ("Operations::Operator", "Operations::Scan", "Operations::Focus")),
                        exclude = DocumentQueries::Except(
                            source = DocumentQueries::Named(qualifiedName = ("Operations::Operator", "Operations::Scan", "Operations::Focus")),
                            exclude = candidates))),
                property = "qualifiedName",
                direction = "ascending",
                missing = "last",
                multiple = "first"),
            properties = ("qualifiedName"))
    }
    part def 'Classifiers Document' :> DocumentQueries::Document {
        attribute redefines title = "Classifiers";
        part rows : DocumentQueries::Table {
            attribute redefines caption = "Classifiers";
            calc rows : 'Classifiers Rows';
        }
    }
    view Types {
        expose 'Types Document';
        render Views::asElementTable;
    }
    calc def 'Types Rows' :> DocumentQueries::Query {
        in candidates : KerML::Root::Element[0..*] ordered = DocumentQueries::Descendants(
            source = DocumentQueries::Named(qualifiedName = ("Operations")));
        DocumentQueries::Project(
            source = DocumentQueries::OrderBy(
                source = DocumentQueries::Union(
                    source = DocumentQueries::WhereType(source = candidates, type = ("Definition", "ViewUsage", "ViewpointUsage")),
                    other = DocumentQueries::Except(
                        source = DocumentQueries::Named(qualifiedName = ("Operations::Operator", "Operations::Scan", "Operations::Focus")),
                        exclude = DocumentQueries::Except(
                            source = DocumentQueries::Named(qualifiedName = ("Operations::Operator", "Operations::Scan", "Operations::Focus")),
                            exclude = candidates))),
                property = "qualifiedName",
                direction = "ascending",
                missing = "last",
                multiple = "first"),
            properties = ("qualifiedName"))
    }
    part def 'Types Document' :> DocumentQueries::Document {
        attribute redefines title = "Types";
        part rows : DocumentQueries::Table {
            attribute redefines caption = "Types";
            calc rows : 'Types Rows';
        }
    }
    view Namespaces {
        expose 'Namespaces Document';
        render Views::asElementTable;
    }
    calc def 'Namespaces Rows' :> DocumentQueries::Query {
        in candidates : KerML::Root::Element[0..*] ordered = DocumentQueries::Descendants(
            source = DocumentQueries::Named(qualifiedName = ("Operations")));
        DocumentQueries::Project(
            source = DocumentQueries::OrderBy(
                source = DocumentQueries::Union(
                    source = DocumentQueries::WhereType(source = candidates, type = ("Package", "Definition", "ViewUsage", "ViewpointUsage", "StateUsage")),
                    other = DocumentQueries::Except(
                        source = DocumentQueries::Named(qualifiedName = ("Operations::Operator", "Operations::Scan", "Operations::Focus")),
                        exclude = DocumentQueries::Except(
                            source = DocumentQueries::Named(qualifiedName = ("Operations::Operator", "Operations::Scan", "Operations::Focus")),
                            exclude = candidates))),
                property = "qualifiedName",
                direction = "ascending",
                missing = "last",
                multiple = "first"),
            properties = ("qualifiedName"))
    }
    part def 'Namespaces Document' :> DocumentQueries::Document {
        attribute redefines title = "Namespaces";
        part rows : DocumentQueries::Table {
            attribute redefines caption = "Namespaces";
            calc rows : 'Namespaces Rows';
        }
    }
    view 'Packageable Elements' {
        expose 'Packageable Elements Document';
        render Views::asElementTable;
    }
    calc def 'Packageable Elements Rows' :> DocumentQueries::Query {
        in candidates : KerML::Root::Element[0..*] ordered = DocumentQueries::Descendants(
            source = DocumentQueries::Named(qualifiedName = ("Operations")));
        DocumentQueries::Project(
            source = DocumentQueries::OrderBy(
                source = DocumentQueries::Union(
                    source = DocumentQueries::WhereType(source = candidates, type = ("Package", "Definition", "ViewUsage", "ViewpointUsage", "Dependency", "SatisfyRequirementUsage", "AllocationUsage")),
                    other = DocumentQueries::Except(
                        source = DocumentQueries::Named(qualifiedName = ("Operations::Operator", "Operations::Scan", "Operations::Focus")),
                        exclude = DocumentQueries::Except(
                            source = DocumentQueries::Named(qualifiedName = ("Operations::Operator", "Operations::Scan", "Operations::Focus")),
                            exclude = candidates))),
                property = "qualifiedName",
                direction = "ascending",
                missing = "last",
                multiple = "first"),
            properties = ("qualifiedName"))
    }
    part def 'Packageable Elements Document' :> DocumentQueries::Document {
        attribute redefines title = "Packageable Elements";
        part rows : DocumentQueries::Table {
            attribute redefines caption = "Packageable Elements";
            calc rows : 'Packageable Elements Rows';
        }
    }
    view Actors {
        expose 'Actors Document';
        render Views::asElementTable;
    }
    calc def 'Actors Rows' :> DocumentQueries::Query {
        in candidates : KerML::Root::Element[0..*] ordered = DocumentQueries::Descendants(
            source = DocumentQueries::Named(qualifiedName = ("Operations")));
        DocumentQueries::Project(
            source = DocumentQueries::OrderBy(
                source = DocumentQueries::Except(
                    source = DocumentQueries::Named(qualifiedName = ("Operations::Operator")),
                    exclude = DocumentQueries::Except(
                        source = DocumentQueries::Named(qualifiedName = ("Operations::Operator")),
                        exclude = candidates)),
                property = "qualifiedName",
                direction = "ascending",
                missing = "last",
                multiple = "first"),
            properties = ("qualifiedName"))
    }
    part def 'Actors Document' :> DocumentQueries::Document {
        attribute redefines title = "Actors";
        part rows : DocumentQueries::Table {
            attribute redefines caption = "Actors";
            calc rows : 'Actors Rows';
        }
    }
    view 'Use Cases' {
        expose 'Use Cases Document';
        render Views::asElementTable;
    }
    calc def 'Use Cases Rows' :> DocumentQueries::Query {
        in candidates : KerML::Root::Element[0..*] ordered = DocumentQueries::Descendants(
            source = DocumentQueries::Named(qualifiedName = ("Operations")));
        DocumentQueries::Project(
            source = DocumentQueries::OrderBy(
                source = DocumentQueries::Except(
                    source = DocumentQueries::Named(qualifiedName = ("Operations::Scan", "Operations::Focus")),
                    exclude = DocumentQueries::Except(
                        source = DocumentQueries::Named(qualifiedName = ("Operations::Scan", "Operations::Focus")),
                        exclude = candidates)),
                property = "qualifiedName",
                direction = "ascending",
                missing = "last",
                multiple = "first"),
            properties = ("qualifiedName"))
    }
    part def 'Use Cases Document' :> DocumentQueries::Document {
        attribute redefines title = "Use Cases";
        part rows : DocumentQueries::Table {
            attribute redefines caption = "Use Cases";
            calc rows : 'Use Cases Rows';
        }
    }
}
