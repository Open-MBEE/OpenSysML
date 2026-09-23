package Plant {
    package Structure {
        part def Pump {
            doc /* Moves fluid through the plant. */
            attribute mass : ScalarValues::Real;
            part valve : Valve;
            abstract action def prime;
            action 'prime 2' : prime;
            state def Cycle {
                entry; then Idle;
                state Idle;
                state Running;
                transition first Idle then Running;
            }
        }
        part def Valve;
        enum def Mode {
            on;
            off;
        }
        individual part def p1 :> Pump {
            attribute :>> mass = 12.5;
        }
        dependency needs from Pump to Valve;
    }
    package Views {
        view Overview;
        viewpoint Operations;
    }
}
package Tables {
    view 'Packageable Elements' {
        expose Plant::Structure::Pump;
        expose 'Packageable Elements Document';
        render Views::asElementTable;
    }
    calc def 'Packageable Elements Rows' :> DocumentQueries::Query {
        DocumentQueries::Project(
            source = DocumentQueries::OrderBy(
                source = DocumentQueries::WhereType(
                    source = DocumentQueries::Descendants(
                        source = DocumentQueries::Named(qualifiedName = ("Plant"))),
                    type = ("Package", "Definition", "ViewUsage", "ViewpointUsage", "Dependency", "SatisfyRequirementUsage", "AllocationUsage")),
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
    view Namespaces {
        expose Plant::Structure::Pump;
        expose 'Namespaces Document';
        render Views::asElementTable;
    }
    calc def 'Namespaces Rows' :> DocumentQueries::Query {
        DocumentQueries::Project(
            source = DocumentQueries::OrderBy(
                source = DocumentQueries::WhereType(
                    source = DocumentQueries::Descendants(
                        source = DocumentQueries::Named(qualifiedName = ("Plant"))),
                    type = ("Package", "Definition", "ViewUsage", "ViewpointUsage", "StateUsage")),
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
    view Types {
        expose Plant::Structure::Pump;
        expose 'Types Document';
        render Views::asElementTable;
    }
    calc def 'Types Rows' :> DocumentQueries::Query {
        DocumentQueries::Project(
            source = DocumentQueries::OrderBy(
                source = DocumentQueries::WhereType(
                    source = DocumentQueries::Descendants(
                        source = DocumentQueries::Named(qualifiedName = ("Plant"))),
                    type = ("Definition", "ViewUsage", "ViewpointUsage")),
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
    view Classifiers {
        expose Plant::Structure::Pump;
        expose 'Classifiers Document';
        render Views::asElementTable;
    }
    calc def 'Classifiers Rows' :> DocumentQueries::Query {
        DocumentQueries::Project(
            source = DocumentQueries::OrderBy(
                source = DocumentQueries::WhereType(
                    source = DocumentQueries::Descendants(
                        source = DocumentQueries::Named(qualifiedName = ("Plant"))),
                    type = ("Definition", "ViewUsage", "ViewpointUsage")),
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
    view Diagrams {
        expose Plant::Structure::Pump;
        expose 'Diagrams Document';
        render Views::asElementTable;
    }
    calc def 'Diagrams Rows' :> DocumentQueries::Query {
        DocumentQueries::Project(
            source = DocumentQueries::OrderBy(
                source = DocumentQueries::WhereType(
                    source = DocumentQueries::Union(
                        source = DocumentQueries::Named(qualifiedName = ("Plant", "Tables", "Model Overview")),
                        other = DocumentQueries::Descendants(
                            source = DocumentQueries::Named(qualifiedName = ("Plant", "Tables", "Model Overview")))),
                    type = ("ViewUsage")),
                property = "qualifiedName",
                direction = "ascending",
                missing = "last",
                multiple = "first"),
            properties = ("qualifiedName"))
    }
    part def 'Diagrams Document' :> DocumentQueries::Document {
        attribute redefines title = "Diagrams";
        part rows : DocumentQueries::Table {
            attribute redefines caption = "Diagrams";
            calc rows : 'Diagrams Rows';
        }
    }
}
view 'Model Overview' {
    expose Plant;
    expose Tables;
    render Views::asTreeDiagram;
}
